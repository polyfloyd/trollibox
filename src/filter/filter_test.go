package filter

import (
	"context"
	"sort"
	"strings"
	"testing"

	"trollibox/src/library"
)

func TestFilterTracks(t *testing.T) {
	tracks := []library.Track{
		{
			Artist: "The B-Trees",
			Title:  "Lucy in the Cloud with Sine Waves",
			Genre:  "Test",
		},
		{
			Artist: "Michael FLACson",
			Title:  "One or Zero",
			Genre:  "Test",
		},
		{
			Artist: "DJ Testo Ft. Curry RAII Yepsend",
			Title:  "call() me Maybe<T>",
			Genre:  "Test",
		},
	}

	results, _ := Tracks(context.Background(), Func(func(track library.Track) (SearchResult, bool) {
		return SearchResult{}, true
	}), tracks)
	if len(results) != 3 {
		t.Fatalf("Unexpected number of results: %v", len(results))
	}

	results, _ = Tracks(context.Background(), Func(func(track library.Track) (SearchResult, bool) {
		return SearchResult{}, false
	}), tracks)
	if len(results) != 0 {
		t.Fatalf("Unexpected number of results: %v", len(results))
	}

	results, _ = Tracks(context.Background(), Func(func(track library.Track) (SearchResult, bool) {
		return SearchResult{}, strings.Contains(track.Artist, "Test")
	}), tracks)
	if len(results) != 1 {
		t.Fatalf("Unexpected number of results: %v", len(results))
	}
}

func TestNumMatches(t *testing.T) {
	result := SearchResult{}
	if n := result.NumMatches(); n != 0 {
		t.Fatalf("Unesxpected number of matches: %v", n)
	}

	result = SearchResult{
		Matches: map[string][]SearchMatch{
			"artist": {{0, 1}, {2, 3}},
		},
	}
	if n := result.NumMatches(); n != 2 {
		t.Fatalf("Unesxpected number of matches: %v", n)
	}

	result = SearchResult{}
	result.AddMatch("artist", 0, 1)
	result.AddMatch("title", 0, 1)
	if n := result.NumMatches(); n != 2 {
		t.Fatalf("Unesxpected number of matches: %v", n)
	}
}

func TestMatchSorting(t *testing.T) {
	results := []SearchResult{
		{
			Track: library.Track{URI: "bar"},
			Matches: map[string][]SearchMatch{
				"artist": {{0, 1}, {2, 3}},
				"title":  {{0, 1}, {2, 3}},
			},
		},
		{
			Track: library.Track{URI: "baz"},
			Matches: map[string][]SearchMatch{
				"artist": {{0, 1}, {2, 3}},
			},
		},
		{
			Track: library.Track{URI: "foo"},
			Matches: map[string][]SearchMatch{
				"artist": {{0, 1}, {2, 3}},
				"title":  {{0, 1}, {2, 3}},
				"album":  {{0, 1}, {2, 3}},
			},
		},
	}
	sort.Sort(ByNumMatches(results))
	if results[0].URI != "foo" {
		t.Fatalf("Wrong sort order: %q at index %v", results[0].URI, 0)
	}
	if results[1].URI != "bar" {
		t.Fatalf("Wrong sort order: %q at index %v", results[0].URI, 1)
	}
	if results[2].URI != "baz" {
		t.Fatalf("Wrong sort order: %q at index %v", results[0].URI, 2)
	}
}
