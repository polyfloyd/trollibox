package filter

import (
	"runtime"
	"sync"

	"github.com/polyfloyd/trollibox/src/player"
)

// The Filter interface implements a method for filtering tracks.
type Filter interface {
	// Checks whether the track passes the filter's criteria.
	Filter(track player.Track) (SearchResult, bool)
}

// A SearchMatch records the start and end offset in the matched atttributes
// value. This information can be used for highlighting.
type SearchMatch struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

type SearchResult struct {
	player.Track
	Matches map[string][]SearchMatch
}

func (sr *SearchResult) AddMatch(property string, start, end int) {
	sr.Matches[property] = append(sr.Matches[property], SearchMatch{Start: start, End: end})
}

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

// Filters the tracks by applying the filter to all tracks.
func FilterTracks(filter Filter, tracks []player.Track) []SearchResult {
	trackStream := make(chan player.Track)
	matchStream := make(chan SearchResult)
	go func() {
		defer close(trackStream)
		for _, track := range tracks {
			trackStream <- track
		}
	}()

	var wg sync.WaitGroup
	wg.Add(runtime.NumCPU())
	for i := 0; i < runtime.NumCPU(); i++ {
		go func() {
			for track := range trackStream {
				if res, ok := filter.Filter(track); ok {
					matchStream <- res
				}
			}
			wg.Done()
		}()
	}
	go func() {
		defer close(matchStream)
		wg.Wait()
	}()

	results := make([]SearchResult, 0, len(tracks))
	for match := range matchStream {
		results = append(results, match)
	}
	return results
}
