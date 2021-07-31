package keyed

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"trollibox/src/filter"
	"trollibox/src/library"
)

// parser returns the grammar of the query as a parser combinator.
// The grammar is curried with so some arguments required by mapping functions
// can be compiled in.
func parser(untaggedFields []string) ParseFunc {
	digit := pAny(pLiterals("0", "1", "2", "3", "4", "5", "6", "7", "8", "9")...)

	strKey := pAny(pLiterals("uri", "artist", "title", "album")...)
	strOperation := pAny(pLiterals("=", ":")...)
	strMatchValue := pApply(pAtLeastOne(pAny(pWordLit(), pLast(pLiterals("\\", " ")...))), gJoinStrings)

	ordKey := pLit("duration")
	ordOperation := pAny(pLiterals("=", "<", ">")...)
	ordMatchValue := pApply(pAtLeastOne(digit), gJoinStrings)

	keyedMatch := pAny(
		pApply(pAll(strKey, strOperation, strMatchValue), gMapStringRule),
		pApply(pAll(ordKey, ordOperation, ordMatchValue), gMapOrdRule),
	)

	unkeyedMatch := pApply(strMatchValue, gMapUnkeyedRule(untaggedFields))

	condition := pAny(keyedMatch, unkeyedMatch)
	//	query := pApply(pAtLeastOne(condition), gMapRuleSet) // TODO: repetition

	query := pApply(pAtLeastOne(pAny(condition, pLit(" "))), gMapRuleSet)

	return query
}

func pWordLit() ParseFunc {
	re := regexp.MustCompile(`\w`)
	return func(source string) (interface{}, int) {
		if len(source) > 0 && re.MatchString(source[:1]) {
			return source[:1], 1
		}
		return nil, -1
	}
}

type rule interface {
	Match(obj interface{ Attr(string) interface{} }) map[string][]filter.SearchMatch
}

type stringContainsRule struct {
	property string
	needle   string
}

func (rule stringContainsRule) Match(obj interface{ Attr(string) interface{} }) map[string][]filter.SearchMatch {
	s, ok := obj.Attr(rule.property).(string)
	if !ok {
		return nil
	}
	i := strings.Index(strings.ToLower(s), rule.needle)
	if i == -1 {
		return nil
	}
	return map[string][]filter.SearchMatch{
		rule.property: {filter.SearchMatch{Start: i, End: i + len(rule.needle)}},
	}
}

type stringEqualsRule struct {
	property string
	needle   string
}

func (rule stringEqualsRule) Match(obj interface{ Attr(string) interface{} }) map[string][]filter.SearchMatch {
	s, ok := obj.Attr(rule.property).(string)
	if !ok || strings.ToLower(s) != rule.needle {
		return nil
	}
	return map[string][]filter.SearchMatch{
		rule.property: {filter.SearchMatch{Start: 0, End: len(rule.needle)}},
	}
}

type ordEqualsRule struct {
	property string
	ref      int64
}

func (rule ordEqualsRule) Match(obj interface{ Attr(string) interface{} }) map[string][]filter.SearchMatch {
	i, ok := obj.Attr(rule.property).(int64)
	if !ok || i != rule.ref {
		return nil
	}
	return map[string][]filter.SearchMatch{
		rule.property: {filter.SearchMatch{Start: 0, End: 1}},
	}
}

type ordLessThanRule struct {
	property string
	ref      int64
}

func (rule ordLessThanRule) Match(obj interface{ Attr(string) interface{} }) map[string][]filter.SearchMatch {
	i, ok := obj.Attr(rule.property).(int64)
	if !ok || i >= rule.ref {
		return nil
	}
	return map[string][]filter.SearchMatch{
		rule.property: {filter.SearchMatch{Start: 0, End: 1}},
	}
}

type ordGreaterThanRule struct {
	property string
	ref      int64
}

func (rule ordGreaterThanRule) Match(obj interface{ Attr(string) interface{} }) map[string][]filter.SearchMatch {
	i, ok := obj.Attr(rule.property).(int64)
	if !ok || i <= rule.ref {
		return nil
	}
	return map[string][]filter.SearchMatch{
		rule.property: {filter.SearchMatch{Start: 0, End: 1}},
	}
}

type unkeyedRule struct {
	properties []string
	needle     string
}

func (rule unkeyedRule) Match(obj interface{ Attr(string) interface{} }) map[string][]filter.SearchMatch {
	m := map[string][]filter.SearchMatch{}
	for _, prop := range rule.properties {
		s, ok := obj.Attr(prop).(string)
		if !ok {
			continue
		}
		i := strings.Index(strings.ToLower(s), rule.needle)
		if i >= 0 {
			m[prop] = append(m[prop], filter.SearchMatch{Start: i, End: i + len(rule.needle)})
		}
	}
	return m
}

func gJoinStrings(v interface{}) interface{} {
	str := ""
	for _, s := range v.([]interface{}) {
		str += s.(string)
	}
	return str
}

func gMapUnkeyedRule(untaggedFields []string) ApplyFunc {
	return func(v interface{}) interface{} {
		return unkeyedRule{
			properties: untaggedFields,
			needle:     strings.ToLower(v.(string)),
		}
	}
}

func gMapStringRule(v interface{}) interface{} {
	property := v.([]interface{})[0].(string)
	operation := v.([]interface{})[1].(string)
	argument := v.([]interface{})[2].(string)
	switch operation {
	case ":":
		return stringContainsRule{property: property, needle: strings.ToLower(argument)}
	case "=":
		return stringEqualsRule{property: property, needle: strings.ToLower(argument)}
	}
	panic("unreachable")
}

func gMapOrdRule(v interface{}) interface{} {
	property := v.([]interface{})[0].(string)
	operation := v.([]interface{})[1].(string)
	argument, _ := strconv.ParseInt(v.([]interface{})[2].(string), 10, 64)
	switch operation {
	case "=":
		return ordEqualsRule{property: property, ref: argument}
	case "<":
		return ordLessThanRule{property: property, ref: argument}
	case ">":
		return ordGreaterThanRule{property: property, ref: argument}
	}
	panic("unreachable")
}

func gMapRuleSet(vv interface{}) interface{} {
	rules := []rule{}
	for _, v := range vv.([]interface{}) {
		rule, ok := v.(rule)
		if ok {
			rules = append(rules, rule)
		}
	}
	return rules
}

func init() {
	filter.RegisterFactory(func() filter.Filter {
		return &Query{}
	})
}

type nojsonQuery struct {
	Query    string   `json:"query"`
	Untagged []string `json:"untagged"`

	rules []rule
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
	v, r := parser(untaggedFields)(query)
	if r < 0 {
		return nil, fmt.Errorf("parse error")
	}
	rules := v.([]rule)

	if len(untaggedFields) == 0 {
		for _, rule := range rules {
			if _, ok := rule.(unkeyedRule); ok {
				return nil, fmt.Errorf("untaggedFields is required for unkeyed rules")
			}
		}
	}

	return &Query{
		Query:    query,
		Untagged: untaggedFields,
		rules:    rules,
	}, nil
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (sq *Query) UnmarshalJSON(data []byte) error {
	if err := json.Unmarshal(data, (*nojsonQuery)(sq)); err != nil {
		return err
	}
	q, err := CompileQuery(sq.Query, sq.Untagged)
	if err != nil {
		return err
	}
	*sq = *q
	return nil
}

// Filter implements the filter.Filter interface.
func (sq *Query) Filter(track library.Track) (filter.SearchResult, bool) {
	if sq == nil || len(sq.rules) == 0 {
		return filter.SearchResult{}, false
	}

	result := filter.SearchResult{
		Track:   track,
		Matches: map[string][]filter.SearchMatch{},
	}
	for _, rule := range sq.rules {
		matches := rule.Match(&track)
		if len(matches) == 0 {
			return filter.SearchResult{}, false
		}
		for property, m := range matches {
			result.AddMatches(property, m...)
		}
	}
	return result, len(result.Matches) > 0
}
