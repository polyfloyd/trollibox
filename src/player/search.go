package player

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var (
	trackAttrs = map[string]bool{
		"artist":      true,
		"title":       true,
		"genre":       true,
		"album":       true,
		"albumartist": true,
		"albumtrack":  true,
		"albumdisc":   true,
	}
)

type SearchMatch struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

type SearchResult struct {
	Track
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

type SearchQuery struct {
	patterns map[string][]*regexp.Regexp
	untagged []string
}

// Compiles a search query so that it may be used to discriminate tracks.
// The query is made up of keywords of the following format:
//   [property:]<value>
//
// A track should contain all the keywords to pass selection. If no property is
// set, the value is searched for in the fields specified by untaggedFields.
//
// It is possible to use asterisks as wildcards.
// A literal whitespace character may be specified by a leading backslash.
//
// The query could look something like this:
//   foo bar baz title:something album:one\ two artist:foo*ar
func CompileSearchQuery(query string, untaggedFields []string) (*SearchQuery, error) {
	if query == "" {
		return nil, fmt.Errorf("Query is empty")
	}

	regexControlRe := regexp.MustCompile("([\\.\\^\\$\\?\\+\\[\\]\\{\\}\\(\\)\\|\\\\])")
	escapedWhite := regexp.MustCompile("\\\\(\\s)")
	queryRe := regexp.MustCompile("(?:(\\w+):)?((?:(?:\\\\\\s)|[^:\\s])+)")

	matches := queryRe.FindAllStringSubmatch(query, -1)
	if matches == nil || len(matches) == 0 {
		return nil, fmt.Errorf("Query does not match the expected format")
	}

	compiled := &SearchQuery{
		patterns: map[string][]*regexp.Regexp{},
		untagged: untaggedFields,
	}
	for _, group := range matches {
		property := group[1]
		if property != "" && !trackAttrs[property] {
			continue
		}

		value := group[2]
		value = escapedWhite.ReplaceAllString(value, "$1")
		value = regexControlRe.ReplaceAllString(value, "\\$1")
		value = strings.Replace(value, "*", ".*", -1)
		re, err := regexp.Compile("(?i)" + value)
		if err != nil {
			return nil, fmt.Errorf("Unable to compile %q for property %q: %v", value, property, err)
		}

		compiled.patterns[property] = append(compiled.patterns[property], re)
	}
	return compiled, nil
}

func (sq *SearchQuery) Matches(track Track) (SearchResult, bool) {
	if sq == nil || len(sq.patterns) == 0 {
		return SearchResult{}, false
	}

	result := SearchResult{
		Track:   track,
		Matches: map[string][]SearchMatch{},
	}
	for property, patterns := range sq.patterns {
		for _, re := range patterns {
			if property == "" {
				foundMatch := false
				for _, prop := range sq.untagged {
					if val, ok := TrackAttr(track, prop).(string); ok {
						if match := re.FindStringIndex(val); match != nil {
							result.AddMatch(prop, match[0], match[1])
							foundMatch = true
						}
					}
				}
				if !foundMatch {
					return SearchResult{}, false
				}

			} else {
				if val, ok := TrackAttr(track, property).(string); ok {
					if match := re.FindStringIndex(val); match != nil {
						result.AddMatch(property, match[0], match[1])
						continue
					}
				}
				return SearchResult{}, false
			}
		}
	}
	return result, len(result.Matches) > 0
}

// Compiles the query and filters the specified tracks. The result is sorted by
// the number of matches in descending order.
func Search(tracks []Track, query string, untaggedFields []string) ([]SearchResult, error) {
	compiledQuery, err := CompileSearchQuery(query, untaggedFields)
	if err != nil {
		return nil, err
	}

	results := make([]SearchResult, 0, len(tracks)/4)
	for _, track := range tracks {
		if res, ok := compiledQuery.Matches(track); ok {
			results = append(results, res)
		}
	}

	sort.Sort(byNumMatches(results))
	return results, nil
}

type byNumMatches []SearchResult

func (l byNumMatches) Len() int           { return len(l) }
func (l byNumMatches) Swap(a, b int)      { l[a], l[b] = l[b], l[a] }
func (l byNumMatches) Less(a, b int) bool { return l[a].NumMatches() > l[b].NumMatches() }
