package keyed

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/polyfloyd/trollibox/src/filter"
	"github.com/polyfloyd/trollibox/src/library"
)

var (
	regexControlRe = regexp.MustCompile("([\\.\\^\\$\\?\\+\\[\\]\\{\\}\\(\\)\\|\\\\])")
	escapedWhite   = regexp.MustCompile("\\\\(\\s)")
	queryRe        = regexp.MustCompile("(?:(\\w+):)?((?:(?:\\\\\\s)|[^:\\s])+)")
)

func init() {
	filter.RegisterFactory(func() filter.Filter {
		return &Query{}
	})
}

type nojsonQuery struct {
	Query    string   `json:"query"`
	Untagged []string `json:"untagged"`

	patterns map[string][]*regexp.Regexp
}

// A Query is a compiled query string.
type Query nojsonQuery

// CompileQuery compiles a search query so that it may be used to discriminate
// tracks.
//
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
func CompileQuery(query string, untaggedFields []string) (*Query, error) {
	pat, err := compilePatterns(query)
	if err != nil {
		return nil, err
	}
	if len(pat[""]) > 0 && len(untaggedFields) == 0 {
		return nil, fmt.Errorf("Keywords without property indicators require untaggedFields to be set")
	}
	return &Query{
		Query:    query,
		Untagged: untaggedFields,
		patterns: pat,
	}, nil
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (sq *Query) UnmarshalJSON(data []byte) error {
	if err := json.Unmarshal(data, (*nojsonQuery)(sq)); err != nil {
		return err
	}
	pat, err := compilePatterns(sq.Query)
	if err != nil {
		return err
	}
	sq.patterns = pat
	return nil
}

// Filter implements the filter.Filter interface.
func (sq *Query) Filter(track library.Track) (filter.SearchResult, bool) {
	if sq == nil || len(sq.patterns) == 0 {
		return filter.SearchResult{}, false
	}

	result := filter.SearchResult{
		Track:   track,
		Matches: map[string][]filter.SearchMatch{},
	}
	for property, patterns := range sq.patterns {
		for _, re := range patterns {
			if property == "" {
				foundMatch := false
				for _, prop := range sq.Untagged {
					if val, ok := track.Attr(prop).(string); ok {
						if match := re.FindStringIndex(val); match != nil {
							result.AddMatch(prop, match[0], match[1])
							foundMatch = true
						}
					}
				}
				if !foundMatch {
					return filter.SearchResult{}, false
				}

			} else {
				if val, ok := track.Attr(property).(string); ok {
					if match := re.FindStringIndex(val); match != nil {
						result.AddMatch(property, match[0], match[1])
						continue
					}
				}
				return filter.SearchResult{}, false
			}
		}
	}
	return result, len(result.Matches) > 0
}

func compilePatterns(query string) (map[string][]*regexp.Regexp, error) {
	if query == "" {
		return nil, fmt.Errorf("Query is empty")
	}

	matches := queryRe.FindAllStringSubmatch(query, -1)
	if matches == nil || len(matches) == 0 {
		return nil, fmt.Errorf("Query does not match the expected format")
	}

	patterns := map[string][]*regexp.Regexp{}
	for _, group := range matches {
		property := group[1]
		value := group[2]
		value = escapedWhite.ReplaceAllString(value, "$1")
		value = regexControlRe.ReplaceAllString(value, "\\$1")
		value = strings.Replace(value, "*", ".*", -1)
		re, err := regexp.Compile("(?i)" + value)
		if err != nil {
			return nil, fmt.Errorf("Unable to compile %q for property %q: %v", value, property, err)
		}

		patterns[property] = append(patterns[property], re)
	}
	return patterns, nil
}
