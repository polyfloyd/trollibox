package ruled

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"time"

	"trollibox/src/filter"
	"trollibox/src/library"
)

const (
	opContains = "contains"
	opEquals   = "equals"
	opGreater  = "greater"
	opLess     = "less"
	opMatches  = "matches"
)

func init() {
	filter.RegisterFactory(func() filter.Filter {
		return &RuleFilter{}
	})
}

// A Rule represents an expression that compares some Track's attribute
// according to an operation with a reference value.
type Rule struct {
	// Name of the track attribute to match.
	Attribute string `json:"attribute"`

	// How to interpret Value. Can be any of the following:
	//   opContains
	//   opEquals
	//   opGreater
	//   opLess
	//   opRegex
	Operation string `json:"operation"`

	// Invert this rule's operation.
	Invert bool `json:"invert"`

	// The value to use with the operation.
	Value interface{} `json:"value"`
}

// MatchFunc Creates a function that matches a track based on this rules criteria.
func (rule Rule) MatchFunc() (func(library.Track) ([]filter.SearchMatch, bool), error) {
	if rule.Attribute == "" {
		return nil, fmt.Errorf("rule's Attribute is unset (%v)", rule)
	}
	if rule.Operation == "" {
		return nil, fmt.Errorf("rule's Operation is unset (%v)", rule)
	}
	if rule.Value == nil {
		return nil, fmt.Errorf("rule's Value is unset (%v)", rule)
	}

	// We'll use rule function to invert the output if necessary.
	var inv func(bool) bool
	if rule.Invert {
		inv = func(val bool) bool { return !val }
	} else {
		inv = func(val bool) bool { return val }
	}

	// Prevent type errors further down.
	typeVal := reflect.ValueOf(rule.Value).Kind()
	typeTrack := reflect.ValueOf((&library.Track{}).Attr(rule.Attribute)).Kind()
	if typeVal != typeTrack && !(typeVal == reflect.Float64 && typeTrack == reflect.Int64) {
		return nil, fmt.Errorf("value and attribute types do not match (%v, %v)", typeVal, typeTrack)
	}

	// The duration is currently the only integer attribute.
	if rule.Attribute == "duration" {
		var durVal time.Duration
		f64Val, okF64 := rule.Value.(float64)
		i64Val, okI64 := rule.Value.(int64)
		if okF64 {
			durVal = time.Duration(f64Val) * time.Second
		} else if okI64 {
			durVal = time.Duration(i64Val) * time.Second
		}
		if okF64 || okI64 {
			switch rule.Operation {
			case opEquals:
				return func(track library.Track) ([]filter.SearchMatch, bool) {
					return nil, inv(track.Duration == durVal)
				}, nil
			case opGreater:
				return func(track library.Track) ([]filter.SearchMatch, bool) {
					return nil, inv(track.Duration > durVal)
				}, nil
			case opLess:
				return func(track library.Track) ([]filter.SearchMatch, bool) {
					return nil, inv(track.Duration < durVal)
				}, nil
			}
		}

	} else if strVal, ok := rule.Value.(string); ok {
		switch rule.Operation {
		case opContains:
			return func(track library.Track) ([]filter.SearchMatch, bool) {
				idx := strings.Index(track.Attr(rule.Attribute).(string), strVal)
				if idx == -1 {
					return nil, inv(false)
				}
				return []filter.SearchMatch{{
					Start: idx, End: idx + len(strVal),
				}}, inv(true)
			}, nil
		case opEquals:
			return func(track library.Track) ([]filter.SearchMatch, bool) {
				if inv(track.Attr(rule.Attribute).(string) == strVal) {
					return []filter.SearchMatch{{
						Start: 0, End: len(strVal),
					}}, true
				}
				return nil, false
			}, nil
		case opGreater:
			return func(track library.Track) ([]filter.SearchMatch, bool) {
				return nil, inv(track.Attr(rule.Attribute).(string) > strVal)
			}, nil
		case opLess:
			return func(track library.Track) ([]filter.SearchMatch, bool) {
				return nil, inv(track.Attr(rule.Attribute).(string) < strVal)
			}, nil
		case opMatches:
			pat, err := regexp.Compile(strVal)
			if err != nil {
				return nil, err
			}
			return func(track library.Track) ([]filter.SearchMatch, bool) {
				indices := pat.FindAllStringIndex(track.Attr(rule.Attribute).(string), -1)
				if indices == nil {
					return nil, inv(false)
				}
				matches := make([]filter.SearchMatch, 0, len(indices)/2)
				for _, ix := range indices {
					matches = append(matches, filter.SearchMatch{
						Start: ix[0],
						End:   ix[1],
					})
				}
				return matches, inv(true)
			}, nil
		}
	}

	return nil, fmt.Errorf("no implementation defined for op(%v), attr(%v), val(%v)", rule.Operation, rule.Attribute, rule.Value)
}

func (rule *Rule) String() string {
	invStr := ""
	if rule.Invert {
		invStr = " not"
	}
	return fmt.Sprintf("if%s %s %s %q", invStr, rule.Attribute, rule.Operation, rule.Value)
}

// A RuleError is an error returned when compiling a set of rules into a
// filter.
type RuleError struct {
	OrigErr error `json:"-"`
	Rule    Rule  `json:"rule"`
	Index   int   `json:"index"`
}

func (err RuleError) Error() string {
	return err.OrigErr.Error()
}

type nojsonRuleFilter struct {
	Rules []Rule `json:"rules"`

	funcs []func(library.Track) ([]filter.SearchMatch, bool)
}

// A RuleFilter is a compiled set of rules.
type RuleFilter nojsonRuleFilter

// BuildFilter builds a filter from a set of rules.
func BuildFilter(rules []Rule) (filter.Filter, error) {
	ft := &RuleFilter{
		Rules: rules,
	}
	var err error
	ft.funcs, err = compileFuncs(rules)
	if err != nil {
		return nil, err
	}
	return ft, nil
}

// Filter implements the filter.Filter interface.
func (ft RuleFilter) Filter(track library.Track) (filter.SearchResult, bool) {
	if len(ft.funcs) == 0 {
		// No rules, match everything.
		return filter.SearchResult{Track: track}, true
	}
	result := filter.SearchResult{Track: track}
	for i, rule := range ft.funcs {
		matches, ok := rule(track)
		if !ok {
			return filter.SearchResult{}, false
		}
		result.AddMatches(ft.Rules[i].Attribute, matches...)
	}
	return result, true
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (ft *RuleFilter) UnmarshalJSON(data []byte) error {
	err := json.Unmarshal(data, (*nojsonRuleFilter)(ft))
	if err != nil {
		return err
	}
	ft.funcs, err = compileFuncs(ft.Rules)
	return err
}

func compileFuncs(rules []Rule) ([]func(library.Track) ([]filter.SearchMatch, bool), error) {
	funcs := make([]func(library.Track) ([]filter.SearchMatch, bool), len(rules))
	for i, rule := range rules {
		var err error
		if funcs[i], err = rule.MatchFunc(); err != nil {
			return nil, &RuleError{OrigErr: err, Rule: rule, Index: i}
		}
	}
	return funcs, nil
}
