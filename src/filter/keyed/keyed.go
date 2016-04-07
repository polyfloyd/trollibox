package keyed

import (
	"fmt"
	"regexp"
	"strings"

	filter "../"
	"../../player"
)

// Track attributes available for searching.
var trackAttrs = map[string]bool{
	"uri":         true,
	"artist":      true,
	"title":       true,
	"genre":       true,
	"album":       true,
	"albumartist": true,
	"albumtrack":  true,
	"albumdisc":   true,
}

type Query struct {
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
func CompileQuery(query string, untaggedFields []string) (*Query, error) {
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

	compiled := &Query{
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

func (sq *Query) Filter(track player.Track) (filter.SearchResult, bool) {
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
				for _, prop := range sq.untagged {
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
