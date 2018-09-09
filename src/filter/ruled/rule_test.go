package ruled

import (
	"testing"

	"github.com/polyfloyd/trollibox/src/library"
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
					Operation: opEquals,
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
					Operation: opEquals,
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
					Operation: opEquals,
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
					Operation: opContains,
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
					Operation: opContains,
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
					Operation: opContains,
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
