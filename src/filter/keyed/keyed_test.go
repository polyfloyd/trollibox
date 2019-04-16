package keyed

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/polyfloyd/trollibox/src/library"
)

func TestParser(t *testing.T) {
	testcases := []struct {
		query  string
		expect []rule
	}{
		{
			"artist:foo",
			[]rule{stringContainsRule{property: "artist", needle: "foo"}},
		},
		{
			"title=foo",
			[]rule{stringEqualsRule{property: "title", needle: "foo"}},
		},
		{
			"duration=42",
			[]rule{ordEqualsRule{property: "duration", ref: 42}},
		},
		{
			"duration<1",
			[]rule{ordLessThanRule{property: "duration", ref: 1}},
		},
		{
			"duration>1337",
			[]rule{ordGreaterThanRule{property: "duration", ref: 1337}},
		},
		{
			"foo",
			[]rule{unkeyedRule{properties: []string{"property"}, needle: "foo"}},
		},
		{
			"foo\\ bar",
			[]rule{unkeyedRule{properties: []string{"property"}, needle: "foo bar"}},
		},
		{
			"artist:foo\\ bar",
			[]rule{stringContainsRule{property: "artist", needle: "foo bar"}},
		},
		{
			"foo bar",
			[]rule{
				unkeyedRule{properties: []string{"property"}, needle: "foo"},
				unkeyedRule{properties: []string{"property"}, needle: "bar"},
			},
		},
		{
			"foo artist:bar",
			[]rule{
				unkeyedRule{properties: []string{"property"}, needle: "foo"},
				stringContainsRule{property: "artist", needle: "bar"},
			},
		},
	}
	p := parser([]string{"property"})
	for _, tt := range testcases {
		t.Run(tt.query, func(t *testing.T) {
			vv, r := p(tt.query)
			if r < 0 && tt.expect != nil {
				t.Fatalf("Expected a match")
			} else if r >= 0 && tt.expect == nil {
				t.Fatalf("Expected no match")
			}
			rules := vv.([]rule)
			if !reflect.DeepEqual(rules, tt.expect) {
				t.Logf("exp %#v", tt.expect)
				t.Logf("got %#v", rules)
				t.Fatalf("Unexpected rules")
			}
		})
	}
}

func TestCompileQuery(t *testing.T) {
	query, err := CompileQuery("foo bar baz", []string{"artist", "title"})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%#v", query.rules)
	if !reflect.DeepEqual(query.Untagged, []string{"artist", "title"}) {
		t.Fatalf("Unexpected untagged: %#v", query.Untagged)
	} else if query.Query != "foo bar baz" {
		t.Fatalf("Unexpected query: %v", query.Query)
	}

	if _, err := CompileQuery("", []string{"artist", "title"}); err == nil {
		t.Fatalf("Empty queries should not compile")
	}

	if _, err := CompileQuery("foo artist:bar", []string{"artist", "title"}); err != nil {
		t.Fatal(err)
	}
}

func TestCompileQueryNoUntagged(t *testing.T) {
	if _, err := CompileQuery("foo bar baz", []string{}); err == nil {
		t.Fatalf("Queries without attribute tags and untagged keywords should not compile")
	}
	if _, err := CompileQuery("foo artist:bar", []string{}); err == nil {
		t.Fatalf("Queries without attribute tags and untagged keywords should not compile")
	}
	if _, err := CompileQuery("artist:foo title:bar", []string{}); err != nil {
		t.Fatal(err)
	}
}

func TestFilter(t *testing.T) {
	query, err := CompileQuery("foo", []string{"artist", "title"})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%#v", query.rules)
	result, ok := query.Filter(library.Track{
		Artist: "asdffootest123",
	})
	if !ok {
		t.Fatalf("No match while a match was expected")
	} else if m := result.Matches["artist"][0]; m.Start != 4 || m.End != 7 {
		t.Fatalf("Unexpected match indices: %#v", m)
	}

	result, ok = query.Filter(library.Track{
		Artist: "FOO",
	})
	if !ok {
		t.Fatalf("Matching should be case insensitive")
	} else if m := result.Matches["artist"][0]; m.Start != 0 || m.End != 3 {
		t.Fatalf("Unexpected match indices: %#v", m)
	}

	query, err = CompileQuery("artist:foo\\ bar\\ baz", []string{})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%#v", query.rules)
	result, ok = query.Filter(library.Track{
		Artist: "foo bar baz",
	})
	if !ok {
		t.Fatalf("White space escaping was ignored")
	} else if m := result.Matches["artist"][0]; m.Start != 0 || m.End != 11 {
		t.Fatalf("Unexpected match indices: %#v", m)
	}
}

func TestJSON(t *testing.T) {
	query, err := CompileQuery("foo artist:baz", []string{"artist", "title"})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%#v", query.rules)

	encoded, err := json.Marshal(query)
	if err != nil {
		t.Fatal(err)
	}
	var decodedQuery Query
	if err := json.Unmarshal(encoded, &decodedQuery); err != nil {
		t.Fatal(err)
	}

	if decodedQuery.Query != query.Query {
		t.Fatalf("Incorrect Query after decoding: %v", decodedQuery.Query)
	}
	if !reflect.DeepEqual(decodedQuery.Untagged, query.Untagged) {
		t.Fatalf("Incorrect Untagged after decoding: %v", decodedQuery.Untagged)
	}

	result, ok := decodedQuery.Filter(library.Track{
		Artist: "baz",
		Title:  "foo",
	})
	if !ok {
		t.Fatalf("No match while a match was expected")
	} else if n := result.NumMatches(); n != 2 {
		t.Fatalf("Unexpected number of matches: %v", n)
	}
}
