package filter

import (
	"../player"
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
	const SPLIT = 1000
	results := make([]SearchResult, 0, len(tracks))
	matchedStream := make(chan *SearchResult, 32)
	remaining := 0

	for input := tracks; len(input) != 0; {
		var part []player.Track
		if len(input) >= SPLIT {
			part = input[0:SPLIT]
			input = input[SPLIT:]
		} else {
			part = input
			input = []player.Track{}
		}

		remaining++
		go func(in []player.Track) {
			for _, track := range in {
				if res, ok := filter.Filter(track); ok {
					matchedStream <- &res
				}
			}
			matchedStream <- nil
		}(part)
	}

	for remaining > 0 {
		res := <-matchedStream
		if res == nil {
			remaining--
			continue
		}
		results = append(results, *res)
	}
	close(matchedStream)
	return results
}
