package ruled

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"time"

	filter "../"
	"../../player"
)

const (
	OP_CONTAINS = "contains"
	OP_EQUALS   = "equals"
	OP_GREATER  = "greater"
	OP_LESS     = "less"
	OP_MATCHES  = "matches"
)

type Rule struct {
	// Name of the track attribute to match.
	Attribute string `json:"attribute"`

	// How to interpret Value. Can be any of the following:
	//   OP_CONTAINS
	//   OP_EQUALS
	//   OP_GREATER
	//   OP_LESS
	//   OP_REGEX
	Operation string `json:"operation"`

	// Invert this rule's operation.
	Invert bool `json:"invert"`

	// The value to use with the operation.
	Value interface{} `json:"value"`
}

// Creates a function that matches a track based on this rules criteria.
func (rule Rule) MatchFunc() (func(player.Track) bool, error) {
	if rule.Attribute == "" {
		return nil, fmt.Errorf("Rule's Attribute is unset (%v)", rule)
	}
	if rule.Operation == "" {
		return nil, fmt.Errorf("Rule's Operation is unset (%v)", rule)
	}
	if rule.Value == nil {
		return nil, fmt.Errorf("Rule's Value is unset (%v)", rule)
	}

	// We'll use rule function to invert the output if nessecary.
	var inv func(bool) bool
	if rule.Invert {
		inv = func(val bool) bool { return !val }
	} else {
		inv = func(val bool) bool { return val }
	}

	// Prevent type errors further down.
	typeVal := reflect.ValueOf(rule.Value).Kind()
	typeTrack := reflect.ValueOf((&player.Track{}).Attr(rule.Attribute)).Kind()
	if typeVal != typeTrack && !(typeVal == reflect.Float64 && typeTrack == reflect.Int) {
		return nil, fmt.Errorf("Value and attribute types do not match (%v, %v)", typeVal, typeTrack)
	}

	// The duration is currently the only integer attribute.
	if float64Val, ok := rule.Value.(float64); ok && rule.Attribute == "duration" {
		durVal := time.Duration(float64Val * float64(time.Second))
		switch rule.Operation {
		case OP_EQUALS:
			return func(track player.Track) bool {
				return inv(track.Duration == durVal)
			}, nil
		case OP_GREATER:
			return func(track player.Track) bool {
				return inv(track.Duration > durVal)
			}, nil
		case OP_LESS:
			return func(track player.Track) bool {
				return inv(track.Duration < durVal)
			}, nil
		}

	} else if strVal, ok := rule.Value.(string); ok {
		switch rule.Operation {
		case OP_CONTAINS:
			return func(track player.Track) bool {
				return inv(strings.Contains(track.Attr(rule.Attribute).(string), strVal))
			}, nil
		case OP_EQUALS:
			return func(track player.Track) bool {
				return inv(track.Attr(rule.Attribute).(string) == strVal)
			}, nil
		case OP_GREATER:
			return func(track player.Track) bool {
				return inv(track.Attr(rule.Attribute).(string) > strVal)
			}, nil
		case OP_LESS:
			return func(track player.Track) bool {
				return inv(track.Attr(rule.Attribute).(string) < strVal)
			}, nil
		case OP_MATCHES:
			if pat, err := regexp.Compile(strVal); err != nil {
				return nil, err
			} else {
				return func(track player.Track) bool {
					return inv(pat.MatchString(track.Attr(rule.Attribute).(string)))
				}, nil
			}
		}
	}

	return nil, fmt.Errorf("No implementation defined for op(%v), attr(%v), val(%v)", rule.Operation, rule.Attribute, rule.Value)
}

func (rule *Rule) String() string {
	invStr := ""
	if rule.Invert {
		invStr = " not"
	}
	return fmt.Sprintf("if%s %s %s \"%v\"", invStr, rule.Attribute, rule.Operation, rule.Value)
}

type RuleError struct {
	OrigErr error
	Rule    Rule
	Index   int
}

func (err RuleError) Error() string {
	return err.OrigErr.Error()
}

type ruleFuncs []func(player.Track) bool

func (funcs ruleFuncs) Filter(track player.Track) (filter.SearchResult, bool) {
	if len(funcs) == 0 {
		// No rules, match everything.
		return filter.SearchResult{Track: track}, true
	}
	for _, rule := range funcs {
		if !rule(track) {
			return filter.SearchResult{}, false
		}
	}
	// TODO: Return a proper search result.
	return filter.SearchResult{Track: track}, true
}

func BuildFilter(rules []Rule) (filter.Filter, error) {
	funcs := make([]func(player.Track) bool, len(rules))
	for i, rule := range rules {
		var err error
		if funcs[i], err = rule.MatchFunc(); err != nil {
			return nil, &RuleError{OrigErr: err, Rule: rule, Index: i}
		}
	}
	return ruleFuncs(funcs), nil
}
