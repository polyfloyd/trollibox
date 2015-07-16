package main

import (
	"fmt"
	"math/rand"
	"reflect"
	"regexp"
	"strings"
	"time"
)

const (
	OP_CONTAINS = "contains"
	OP_EQUALS   = "equals"
	OP_EXISTS   = "exists"
	OP_GREATER  = "greater"
	OP_LESS     = "less"
	OP_REGEX    = "regex"
)


type SelectionRule struct {
	// Name of the track attribute to match.
	Attribute string `json:"attribute"`

	// How to interpret Value. Can be any of the following:
	//   OP_CONTAINS
	//   OP_EQUALS
	//   OP_EXISTS
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
func (this *SelectionRule) MatchFunc() (func(*LocalTrack) bool, error) {
	if this.Attribute == "" {
		return nil, fmt.Errorf("Rule's Attribute is unset (%v)", this)
	}
	if this.Operation == "" {
		return nil, fmt.Errorf("Rule's Operation is unset (%v)", this)
	}
	if this.Value == nil {
		return nil, fmt.Errorf("Rule's Value is unset (%v)", this)
	}

	// We'll use this function to invert the output if nessecary.
	inv := func(val bool) bool {
		if this.Invert {
			return !val
		} else {
			return val
		}
	}

	// Prevent type errors further down.
	typeVal   := reflect.ValueOf(this.Value).Kind()
	typeTrack := reflect.ValueOf((&LocalTrack{}).AttributeByName(this.Attribute)).Kind()
	if typeVal != typeTrack && !(typeVal == reflect.Float64 && typeTrack == reflect.Int) {
		return nil, fmt.Errorf("Value and attribute types do not match (%v, %v)", typeVal, typeTrack)
	}

	// The duration is currently the only integer attribute.
	if float64Val, ok := this.Value.(float64); ok && this.Attribute == "duration" {
		intVal := int(float64Val)
		switch this.Operation {
		case OP_EQUALS:
			return func(track *LocalTrack) bool {
				return inv(track.Duration == intVal)
			}, nil
		case OP_GREATER:
			return func(track *LocalTrack) bool {
				return inv(track.Duration > intVal)
			}, nil
		case OP_LESS:
			return func(track *LocalTrack) bool {
				return inv(track.Duration < intVal)
			}, nil
		}

	} else if strVal, ok := this.Value.(string); ok {
		switch this.Operation {
		case OP_CONTAINS:
			return func(track *LocalTrack) bool {
				return inv(strings.Contains(track.AttributeByName(this.Attribute).(string), strVal))
			}, nil
		case OP_EQUALS:
			return func(track *LocalTrack) bool {
				return inv(track.AttributeByName(this.Attribute).(string) == strVal)
			}, nil
		case OP_EXISTS:
			return func(track *LocalTrack) bool {
				return inv(track.AttributeByName(this.Attribute).(string) != "")
			}, nil
		case OP_GREATER:
			return func(track *LocalTrack) bool {
				return inv(track.AttributeByName(this.Attribute).(string) > strVal)
			}, nil
		case OP_LESS:
			return func(track *LocalTrack) bool {
				return inv(track.AttributeByName(this.Attribute).(string) < strVal)
			}, nil
		case OP_REGEX:
			if pat, err := regexp.Compile(strVal); err != nil {
				return nil, err
			} else {
				return func(track *LocalTrack) bool {
					return inv(pat.MatchString(track.AttributeByName(this.Attribute).(string)))
				}, nil
			}
		}
	}

	return nil, fmt.Errorf("No implementation defined for op(%v), attr(%v), val(%v)", this.Operation, this.Attribute, this.Value)
}

func (this *SelectionRule) String() string {
	invStr := ""
	if this.Invert {
		invStr = " not"
	}
	return fmt.Sprintf("if%v %v %v \"%v\"", invStr, this.Attribute, this.Operation, this.Value)
}


// The Queuer controls which tracks are added to the playlist.
type Queuer struct {
	rules []SelectionRule

	rand *rand.Rand

	// Rules translated into functions. Cached to improve performance. Call
	// updateRuleFuncs to synchronize with the actual rules.
	ruleFuncs []func(*LocalTrack) bool

	storage *PersistentStorage
}

func NewQueuer(file string) (this *Queuer, err error) {
	this = &Queuer{
		rules: []SelectionRule{},
		rand:  rand.New(rand.NewSource(time.Now().UnixNano())),
	}
	this.storage, err = NewPersistentStorage(file, &this.rules)
	if err := this.updateRuleFuncs(); err != nil {
		return nil, err
	}
	return this, nil
}

// Select a track based on the rules set. A track must match all rules in order
// to be picked.
func (this *Queuer) SelectTrack(tracks []LocalTrack) *LocalTrack {
	// Just pick a random track when no rules are set.
	if len(this.rules) == 0 {
		return &tracks[this.rand.Intn(len(tracks))]
	}

	selection := make([]*LocalTrack, len(tracks))[0:0]
	outer: for i := range tracks {
		for _, rule := range this.ruleFuncs {
			if !rule(&tracks[i]) {
				continue outer
			}
		}
		selection = append(selection, &tracks[i])
	}

	if len(selection) == 0 {
		return nil
	}
	return selection[this.rand.Intn(len(selection))]
}

func (this *Queuer) Rules() []SelectionRule {
	return this.rules
}

func (this *Queuer) AddRule(rule SelectionRule) error {
	if len(this.rules) != len(this.ruleFuncs) {
		if err := this.updateRuleFuncs(); err != nil {
			return err
		}
	}

	if fn, err := rule.MatchFunc(); err != nil {
		return err
	} else {
		this.rules     = append(this.rules, rule)
		this.ruleFuncs = append(this.ruleFuncs, fn)
		return nil
	}
}

func (this *Queuer) RemoveRule(index int) error {
	if err := this.SetRules(append(this.rules[:index], this.rules[index+1:]...)); err != nil {
		return err
	}
	return this.storage.SetValue(&this.rules)
}

func (this *Queuer) SetRules(rules []SelectionRule) error {
	ruleFuncs, err := makeRuleFuncs(rules)
	if err != nil {
		return err
	}
	this.ruleFuncs = ruleFuncs

	this.rules = rules
	return this.storage.SetValue(&this.rules)
}

func (this *Queuer) updateRuleFuncs() (err error) {
	this.ruleFuncs, err = makeRuleFuncs(this.rules)
	return
}

func makeRuleFuncs(rules []SelectionRule) (funcs []func(*LocalTrack) bool, err error) {
	funcs = make([]func(*LocalTrack) bool, len(rules))
	for i, rule := range rules {
		if funcs[i], err = rule.MatchFunc(); err != nil {
			funcs = []func(*LocalTrack) bool{}
			return
		}
	}
	return
}
