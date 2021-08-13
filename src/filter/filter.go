package filter

import (
	"context"
	"runtime"
	"sync"

	"trollibox/src/library"
)

// The Filter interface implements a method for filtering tracks.
type Filter interface {
	// Checks whether the track passes the filter's criteria.
	Filter(track library.Track) (SearchResult, bool)
}

// Func adds an implementation of the Filter interface to a function with a
// similar signature.
type Func func(library.Track) (SearchResult, bool)

// Filter implements the filter.Filter interface.
func (ff Func) Filter(track library.Track) (SearchResult, bool) {
	return ff(track)
}

// A SearchMatch records the start and end offset in the matched atttributes
// value. This information can be used for highlighting.
type SearchMatch struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

// A SearchResult is a track that matched some Filter's criteria along with
// what properties were matched.
type SearchResult struct {
	library.Track
	Matches map[string][]SearchMatch
}

// AddMatch marks a portion of the named property value as matched.
//
// Multiple possibliy overlapping matches may be added. The propertes accepted
// are the same as Track.Attr().
func (sr *SearchResult) AddMatch(property string, start, end int) {
	sr.AddMatches(property, SearchMatch{Start: start, End: end})
}

// AddMatches marks portions of the named property value as matched.
func (sr *SearchResult) AddMatches(property string, matches ...SearchMatch) {
	if sr.Matches == nil {
		sr.Matches = map[string][]SearchMatch{}
	}
	sr.Matches[property] = append(sr.Matches[property], matches...)
}

// NumMatches returns the total number of matches across all properties.
func (sr SearchResult) NumMatches() (n int) {
	for _, prop := range sr.Matches {
		n += len(prop)
	}
	return
}

// ByNumMatches implements the sort.Interface to sort a list of search results
// by the number of times a track attribute was matched in descending order.
type ByNumMatches []SearchResult

func (l ByNumMatches) Len() int           { return len(l) }
func (l ByNumMatches) Swap(a, b int)      { l[a], l[b] = l[b], l[a] }
func (l ByNumMatches) Less(a, b int) bool { return l[a].NumMatches() > l[b].NumMatches() }

// Tracks filters a list of tracks by applying the specified filter to all
// tracks.
//
// An error is returned if, and only if, the specified context is canceled.
func Tracks(ctx context.Context, filter Filter, tracks []library.Track) ([]SearchResult, error) {
	trackStream := make(chan library.Track)
	matchStream := make(chan SearchResult, runtime.NumCPU())
	go func() {
		defer close(trackStream)
		for _, track := range tracks {
			select {
			case trackStream <- track:
			case <-ctx.Done():
			}
		}
	}()

	var wg sync.WaitGroup
	wg.Add(runtime.NumCPU())
	for i := 0; i < runtime.NumCPU(); i++ {
		go func() {
			defer wg.Done()
			for track := range trackStream {
				if res, ok := filter.Filter(track); ok {
					matchStream <- res
				}
			}
		}()
	}
	go func() {
		defer close(matchStream)
		wg.Wait()
	}()

	results := make([]SearchResult, 0, len(tracks))
	for {
		select {
		case match, ok := <-matchStream:
			if !ok {
				return results, nil
			}
			results = append(results, match)
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}
