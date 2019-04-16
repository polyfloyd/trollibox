package keyed

import (
	"strings"
)

type (
	// ParseFunc is an internal parser combinator primitive.
	ParseFunc func(source string) (value interface{}, remainderIndex int)
	// ApplyFunc is an internal parser combinator primitive.
	ApplyFunc func(interface{}) interface{}
)

func pLit(lit string) ParseFunc {
	return func(source string) (interface{}, int) {
		if strings.HasPrefix(source, lit) {
			return source[:len(lit)], len(lit)
		}
		return nil, -1
	}
}

func pLiterals(literals ...string) []ParseFunc {
	parsers := make([]ParseFunc, len(literals))
	for i, lit := range literals {
		parsers[i] = pLit(lit)
	}
	return parsers
}

func pAll(parsers ...ParseFunc) ParseFunc {
	return func(source string) (interface{}, int) {
		accumValue, prevRemainder := []interface{}{}, 0
		for _, p := range parsers {
			v, r := p(source[prevRemainder:])
			if r < 0 {
				return nil, -1
			}
			accumValue = append(accumValue, v)
			prevRemainder += r
		}
		return accumValue, prevRemainder
	}
}

func pAny(parsers ...ParseFunc) ParseFunc {
	return func(source string) (interface{}, int) {
		for _, p := range parsers {
			if v, r := p(source); r >= 0 {
				return v, r
			}
		}
		return nil, -1
	}
}

func pLast(parsers ...ParseFunc) ParseFunc {
	return func(source string) (interface{}, int) {
		accumValue, prevRemainder := []interface{}{}, 0
		for _, p := range parsers {
			v, r := p(source[prevRemainder:])
			if r < 0 {
				return nil, -1
			}
			accumValue = append(accumValue, v)
			prevRemainder += r
		}
		if len(accumValue) == 0 {
			return nil, -1
		}
		return accumValue[len(accumValue)-1], prevRemainder
	}
}

func pAtLeastOne(parser ParseFunc) ParseFunc {
	return func(source string) (interface{}, int) {
		vv, rr := []interface{}{}, 0
		for {
			v, r := parser(source[rr:])
			if r < 0 {
				break
			}
			rr += r
			vv = append(vv, v)
		}
		if rr == 0 {
			return nil, -1
		}
		return vv, rr
	}
}

func pApply(p ParseFunc, fn ApplyFunc) ParseFunc {
	return func(source string) (interface{}, int) {
		v, r := p(source)
		if r < 0 {
			return nil, -1
		}
		return fn(v), r
	}
}
