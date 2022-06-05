package ruled

import (
	"testing"
	"time"

	"trollibox/src/library"
)

func TestMatchEquals(t *testing.T) {
	tt := []struct {
		track       library.Track
		shouldMatch bool
		rules       []Rule
	}{
		{
			track: library.Track{
				Artist: "Foo Bar",
			},
			shouldMatch: true,
			rules: []Rule{
				{
					Attribute: "artist",
					Operation: Equals,
					Value:     "Foo Bar",
				},
			},
		},
		{
			track: library.Track{
				Artist: "Foo Bar",
			},
			shouldMatch: false,
			rules: []Rule{
				{
					Attribute: "artist",
					Operation: Equals,
					Value:     "Foo Bar",
					Invert:    true,
				},
			},
		},
		{
			track: library.Track{
				Artist: "Bar Foo",
			},
			shouldMatch: false,
			rules: []Rule{
				{
					Attribute: "artist",
					Operation: Equals,
					Value:     "Baz",
				},
			},
		},
	}
	for i, tc := range tt {
		f, err := BuildFilter(tc.rules)
		if err != nil {
			t.Fatal(err)
		}
		if _, matched := f.Filter(tc.track); matched != tc.shouldMatch {
			t.Fatalf("unexpected result for test case %d", i)
		}
	}
}

func TestMatchContains(t *testing.T) {
	tt := []struct {
		track       library.Track
		shouldMatch bool
		rules       []Rule
	}{
		{
			track: library.Track{
				Artist: "Foo Bar",
			},
			shouldMatch: true,
			rules: []Rule{
				{
					Attribute: "artist",
					Operation: Contains,
					Value:     "Foo",
				},
			},
		},
		{
			track: library.Track{
				Artist: "Kevin Bloody Wilson",
			},
			shouldMatch: false,
			rules: []Rule{
				{
					Attribute: "artist",
					Operation: Contains,
					Value:     "Kevin",
					Invert:    true,
				},
			},
		},
		{
			track: library.Track{
				Artist: "Bar Foo",
			},
			shouldMatch: false,
			rules: []Rule{
				{
					Attribute: "artist",
					Operation: Contains,
					Value:     "Baz",
				},
			},
		},
	}
	for i, tc := range tt {
		f, err := BuildFilter(tc.rules)
		if err != nil {
			t.Fatal(err)
		}
		if _, matched := f.Filter(tc.track); matched != tc.shouldMatch {
			t.Fatalf("unexpected result for test case %d", i)
		}
	}
}

func TestMatchMatches(t *testing.T) {
	tt := []struct {
		track       library.Track
		shouldMatch bool
		rules       []Rule
	}{
		{
			track: library.Track{
				Artist: "Foo Bar",
			},
			shouldMatch: true,
			rules: []Rule{
				{
					Attribute: "artist",
					Operation: Matches,
					Value:     "(?i)foo",
				},
			},
		},
		{
			track: library.Track{
				Artist: "Foo Bar",
			},
			shouldMatch: false,
			rules: []Rule{
				{
					Attribute: "artist",
					Operation: Matches,
					Value:     "F{2,}",
				},
			},
		},
		{
			track: library.Track{
				Artist: "Foo Bar",
			},
			shouldMatch: true,
			rules: []Rule{
				{
					Attribute: "artist",
					Operation: Matches,
					Value:     "asdfasdf",
					Invert:    true,
				},
			},
		},
	}
	for i, tc := range tt {
		f, err := BuildFilter(tc.rules)
		if err != nil {
			t.Fatal(err)
		}
		if _, matched := f.Filter(tc.track); matched != tc.shouldMatch {
			t.Fatalf("unexpected result for test case %d", i)
		}
	}
}

func TestMatchGreater(t *testing.T) {
	tt := []struct {
		track       library.Track
		shouldMatch bool
		rules       []Rule
	}{
		{
			track: library.Track{
				Duration: time.Second * 42,
			},
			shouldMatch: true,
			rules: []Rule{
				{
					Attribute: "duration",
					Operation: Greater,
					Value:     12.0,
				},
			},
		},
		{
			track: library.Track{
				Duration: time.Second * 42,
			},
			shouldMatch: false,
			rules: []Rule{
				{
					Attribute: "duration",
					Operation: Greater,
					Value:     12.0,
					Invert:    true,
				},
			},
		},
		{
			track: library.Track{
				Duration: time.Second * 42,
			},
			shouldMatch: false,
			rules: []Rule{
				{
					Attribute: "duration",
					Operation: Greater,
					Value:     int64(64),
				},
			},
		},
	}
	for i, tc := range tt {
		f, err := BuildFilter(tc.rules)
		if err != nil {
			t.Fatal(err)
		}
		if _, matched := f.Filter(tc.track); matched != tc.shouldMatch {
			t.Fatalf("unexpected result for test case %d: matched=%t", i, matched)
		}
	}
}

func TestMatchesError(t *testing.T) {
	_, err := BuildFilter([]Rule{
		{
			Attribute: "artist",
			Operation: Matches,
			Value:     "{1}",
		},
	})
	if err == nil {
		t.Fatalf("expected an error on regex compilation failure")
	}
}
