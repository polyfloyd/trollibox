package keyed

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/polyfloyd/trollibox/src/library"
)

func TestCompileQuery(t *testing.T) {
	query, err := CompileQuery("foo bar baz", []string{"artist", "title"})
	if err != nil {
		t.Fatal(err)
	}
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
