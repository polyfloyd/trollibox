package player

import (
	"fmt"
	"regexp"
)

// ValidListName may be used to check whether the name of a player list entry
// is valid.
var ValidListName = regexp.MustCompile("^\\w+$")

// A List is an immutable collection of named players.
type List interface {
	// Returns a list of all players that are online and able to be controlled
	// or nil and an error.
	//
	// The list is alphabetically sorted and only contains names which match
	// the regular expression ^\w+$.
	PlayerNames() ([]string, error)

	// PlayerByName looks up a player by it's name.
	// An error is returned if something goes wrong while looking up the
	// specified player.
	//
	// If no player with the specified name is found, nil is returned without
	// any error.
	PlayerByName(name string) (Player, error)
}

// A SimpleList provides an easy way to map players to a name.
type SimpleList map[string]Player

// Set maps the specified player to the specified name overwriting any player
// with the same name.
//
// And error is returned if the name does not match the name format.
func (sl *SimpleList) Set(name string, player Player) error {
	if match, _ := regexp.MatchString("^\\w+$", name); !match {
		return fmt.Errorf("invalid player name: %q", name)
	}
	(*sl)[name] = player
	return nil
}

// PlayerNames implements the player.List interface.
func (sl SimpleList) PlayerNames() ([]string, error) {
	names := make([]string, 0, len(sl))
	for name := range sl {
		names = append(names, name)
	}
	return names, nil
}

// PlayerByName implements the player.List interface.
func (sl SimpleList) PlayerByName(name string) (Player, error) {
	if sl == nil {
		return nil, nil
	}
	if pl, ok := sl[name]; ok {
		return pl, nil
	}
	return nil, fmt.Errorf("no player with name %q", name)
}

// A MultiList combines multiple player lists into one.
type MultiList []List

// PlayerNames implements the player.List interface.
func (mp MultiList) PlayerNames() ([]string, error) {
	names := make([]string, 0, 1)
	for _, list := range mp {
		sublist, err := list.PlayerNames()
		if err != nil {
			return nil, err
		}
		names = append(names, sublist...)
	}
	return names, nil
}

// PlayerByName implements the player.List interface.
func (mp MultiList) PlayerByName(name string) (Player, error) {
	for _, list := range mp {
		player, err := list.PlayerByName(name)
		if err != nil {
			return nil, err
		}
		if player != nil {
			return player, nil
		}
	}
	return nil, fmt.Errorf("no player with name %q", name)
}
