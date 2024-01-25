package filter

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path"
	"sort"
	"strings"
	"sync"

	"trollibox/src/util"
)

var ErrNotFound = errors.New("filter not found")

// A ListEvent is emitted when a filter is removed or added.
type ListEvent struct {
	Names []string
}

// An UpdateEvent is emitted when the database has changed.
type UpdateEvent struct {
	Name   string
	Filter Filter
}

var factories = map[string]func() Filter{}

// RegisterFactory registers a factory function that enables DB to deserialize
// filters of that type.
//
// The returned value should be the zero value of that type.
func RegisterFactory(factory func() Filter) {
	t := filterType(factory())
	factories[t] = factory
}

type storageFormat struct {
	Type string `json:"type"`
}

func UnmarshalJSON(b []byte) (Filter, error) {
	var ft storageFormat
	if err := json.Unmarshal(b, &ft); err != nil {
		return nil, err
	}

	fac, ok := factories[ft.Type]
	if !ok {
		return nil, fmt.Errorf("unknown filter type: %s", ft.Type)
	}
	filter := fac()
	if err := json.Unmarshal(b, filter); err != nil {
		return nil, err
	}
	return filter, nil
}

// A DB handles storage of filter implemementations to disk.
type DB struct {
	util.Emitter

	cache     sync.Map // map[string]Filter
	directory string
}

// NewDB constructs a new database for storing filters at the specified
// directory.
func NewDB(directory string) (*DB, error) {
	if err := os.MkdirAll(directory, 0755); err != nil {
		return nil, err
	}
	return &DB{directory: directory}, nil
}

// Names lists all filters that this DB has stored.
func (db *DB) Names() ([]string, error) {
	names := []string{}
	db.cache.Range(func(k, v any) bool {
		names = append(names, k.(string))
		return true
	})

	files, err := os.ReadDir(db.directory)
	if err != nil {
		return nil, err
	}
outer:
	for _, file := range files {
		if file.IsDir() || path.Ext(file.Name()) != ".json" {
			continue
		}
		name := strings.TrimSuffix(path.Base(file.Name()), path.Ext(file.Name()))
		for _, n := range names { // Deduplicate entries found in the cache.
			if n == name {
				continue outer
			}
		}
		names = append(names, name)
	}

	sort.Strings(names)
	return names, nil
}

// Get retrieves a filter with the specified name or nil if no such filter exists.
//
// An error is returned if the database is not able to instantiate the filter,
// which could be caused by a missing factory.
func (db *DB) Get(name string) (Filter, error) {
	if f, ok := db.cache.Load(name); ok {
		return f.(Filter), nil
	}

	b, err := os.ReadFile(db.filterFile(name))
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("%w, no filter with name %q", ErrNotFound, name)
	} else if err != nil {
		return nil, err
	}
	return UnmarshalJSON(b)
}

// Set stores the specified filter under the specified name overwriting any
// pre-existing filter with the same name.
func (db *DB) Set(name string, filter Filter) error {
	if name == "" || strings.Contains(name, "/") {
		return fmt.Errorf("invalid filter name: %q", name)
	}

	db.cache.Store(name, filter)

	filename := db.filterFile(name)
	// Emit a ListEvent if the target file does not exist yet.
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		names, err := db.Names()
		if err != nil {
			db.cache.Delete(name)
			return err
		}
		defer db.Emit(ListEvent{Names: names})
	}

	defer db.Emit(UpdateEvent{Name: name, Filter: filter})

	fd, err := os.Create(filename)
	if err != nil {
		slog.Error("Could not create filter db file", "error", err, "path", filename)
		return nil
	}
	defer fd.Close()
	if err := json.NewEncoder(fd).Encode(filter); err != nil {
		slog.Error("Could not read filter db", "error", err)
		return nil
	}
	return nil
}

// Remove removes the named filter from the database.
//
// Removing a non-existent filter is a no-op.
func (db *DB) Remove(name string) error {
	db.cache.Delete(name)
	if err := os.Remove(db.filterFile(name)); os.IsNotExist(err) {
		return fmt.Errorf("%w, no filter with name %q", ErrNotFound, name)
	} else if err != nil {
		return err
	}

	defer db.Emit(UpdateEvent{Name: name, Filter: nil})

	names, err := db.Names()
	if err != nil {
		return err
	}
	defer db.Emit(ListEvent{Names: names})

	return nil
}

// Events implements the util.Eventer interface.
func (db *DB) Events() *util.Emitter {
	return &db.Emitter
}

func (db *DB) filterFile(name string) string {
	return path.Join(db.directory, name+".json")
}

func filterType(filter Filter) string {
	b, err := json.Marshal(filter)
	if err != nil {
		panic(err)
	}
	var data storageFormat
	if err := json.Unmarshal(b, &data); err != nil {
		panic(err)
	}
	if data.Type == "" {
		panic("no type in filter JSON")
	}
	return data.Type
}
