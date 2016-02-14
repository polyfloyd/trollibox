package player

import (
	"fmt"
	"math/rand"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"time"

	"../util"
)

const (
	OP_CONTAINS = "contains"
	OP_EQUALS   = "equals"
	OP_GREATER  = "greater"
	OP_LESS     = "less"
	OP_MATCHES  = "matches"
)

type SelectionRule struct {
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
func (rule SelectionRule) MatchFunc() (func(Track) bool, error) {
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
	inv := func(val bool) bool {
		if rule.Invert {
			return !val
		} else {
			return val
		}
	}

	// Prevent type errors further down.
	typeVal := reflect.ValueOf(rule.Value).Kind()
	typeTrack := reflect.ValueOf((&Track{}).Attr(rule.Attribute)).Kind()
	if typeVal != typeTrack && !(typeVal == reflect.Float64 && typeTrack == reflect.Int) {
		return nil, fmt.Errorf("Value and attribute types do not match (%v, %v)", typeVal, typeTrack)
	}

	// The duration is currently the only integer attribute.
	if float64Val, ok := rule.Value.(float64); ok && rule.Attribute == "duration" {
		durVal := time.Duration(float64Val * float64(time.Second))
		switch rule.Operation {
		case OP_EQUALS:
			return func(track Track) bool {
				return inv(track.Duration == durVal)
			}, nil
		case OP_GREATER:
			return func(track Track) bool {
				return inv(track.Duration > durVal)
			}, nil
		case OP_LESS:
			return func(track Track) bool {
				return inv(track.Duration < durVal)
			}, nil
		}

	} else if strVal, ok := rule.Value.(string); ok {
		switch rule.Operation {
		case OP_CONTAINS:
			return func(track Track) bool {
				return inv(strings.Contains(track.Attr(rule.Attribute).(string), strVal))
			}, nil
		case OP_EQUALS:
			return func(track Track) bool {
				return inv(track.Attr(rule.Attribute).(string) == strVal)
			}, nil
		case OP_GREATER:
			return func(track Track) bool {
				return inv(track.Attr(rule.Attribute).(string) > strVal)
			}, nil
		case OP_LESS:
			return func(track Track) bool {
				return inv(track.Attr(rule.Attribute).(string) < strVal)
			}, nil
		case OP_MATCHES:
			if pat, err := regexp.Compile(strVal); err != nil {
				return nil, err
			} else {
				return func(track Track) bool {
					return inv(pat.MatchString(track.Attr(rule.Attribute).(string)))
				}, nil
			}
		}
	}

	return nil, fmt.Errorf("No implementation defined for op(%v), attr(%v), val(%v)", rule.Operation, rule.Attribute, rule.Value)
}

func (rule *SelectionRule) String() string {
	invStr := ""
	if rule.Invert {
		invStr = " not"
	}
	return fmt.Sprintf("if%v %v %v \"%v\"", invStr, rule.Attribute, rule.Operation, rule.Value)
}

// The Queuer controls which tracks are added to the playlist.
type Queuer struct {
	util.Emitter

	rand *rand.Rand

	storage *util.PersistentStorage
}

func NewQueuer(file string) (queuer *Queuer, err error) {
	queuer = &Queuer{
		rand: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
	if queuer.storage, err = util.NewPersistentStorage(file, &[]SelectionRule{}); err != nil {
		return nil, err
	}
	// Check the integrity of the rules.
	if err := queuer.SetRules(queuer.Rules()); err != nil {
		return nil, err
	}
	return queuer, nil
}

func (queuer *Queuer) Iterator(pl Player) TrackIterator {
	return queuerIterator{
		player: pl,
		queuer: queuer,
	}
}

// Picks a random track from the specified tracklist. Does not apply any of the
// set selection rules.
func (queuer *Queuer) RandomTrack(tracks []Track) *Track {
	if len(tracks) == 0 {
		return nil
	}
	return &tracks[queuer.rand.Intn(len(tracks))]
}

// Select a track based on the rules set. A track must match all rules in order
// to be picked.
func (queuer *Queuer) SelectRandomTrack(tracks []Track) *Track {
	if len(tracks) == 0 {
		return nil
	}

	// Just pick a random track when no rules are set.
	if len(queuer.Rules()) == 0 {
		return queuer.RandomTrack(tracks)
	}

	ruleFuncs, _ := makeRuleFuncs(queuer.Rules())

	const SPLIT = 1000
	var wg sync.WaitGroup
	output := make([][]Track, 0, len(tracks)/SPLIT+1)
	for input := tracks; len(input) != 0; {
		var part []Track
		if len(input) >= SPLIT {
			part = input[0:SPLIT]
			input = input[SPLIT:]
		} else {
			part = input
			input = []Track{}
		}
		output = append(output, make([]Track, 0, SPLIT))

		wg.Add(1)
		go func(in []Track, out *[]Track) {
			defer wg.Done()
		outer:
			for i := range in {
				for _, rule := range ruleFuncs {
					if !rule(in[i]) {
						continue outer
					}
				}
				*out = append(*out, in[i])
			}
		}(part, &output[len(output)-1])
	}
	wg.Wait()

	numPassedTracks := 0
	for _, part := range output {
		numPassedTracks += len(part)
	}
	if numPassedTracks == 0 {
		return nil
	}

	pickIndex := queuer.rand.Intn(numPassedTracks)
	index := 0
	for _, part := range output {
		if index+len(part) > pickIndex {
			return &part[pickIndex-index]
		}
		index += len(part)
	}

	return nil
}

func (queuer *Queuer) Rules() []SelectionRule {
	return *queuer.storage.Value().(*[]SelectionRule)
}

func (queuer *Queuer) SetRules(rules []SelectionRule) error {
	if _, err := makeRuleFuncs(rules); err != nil {
		return err
	}
	queuer.Emit("update")
	return queuer.storage.SetValue(&rules)
}

type RuleError struct {
	OrigErr error
	Rule    SelectionRule
	Index   int
}

func (err RuleError) Error() string {
	return err.OrigErr.Error()
}

func makeRuleFuncs(rules []SelectionRule) ([]func(Track) bool, error) {
	funcs := make([]func(Track) bool, len(rules))
	for i, rule := range rules {
		var err error
		if funcs[i], err = rule.MatchFunc(); err != nil {
			return nil, &RuleError{OrigErr: err, Rule: rule, Index: i}
		}
	}
	return funcs, nil
}

type queuerIterator struct {
	player Player
	queuer *Queuer
}

func (qi queuerIterator) NextTrack() (PlaylistTrack, bool) {
	tracks, err := qi.player.Tracks()
	if err != nil {
		return PlaylistTrack{}, false
	}
	track := qi.queuer.SelectRandomTrack(tracks)
	if track == nil {
		track = qi.queuer.RandomTrack(tracks)
		if track == nil {
			return PlaylistTrack{}, false
		}
	}
	return PlaylistTrack{
		Track:    *track,
		QueuedBy: "system",
	}, true
}
