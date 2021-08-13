package filter

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"sort"
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"

	"trollibox/src/util"
)

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
	fd, err := os.Open(db.directory)
	if err != nil {
		return nil, err
	}
	defer fd.Close()

	files, err := fd.Readdir(0)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(files))
	for _, file := range files {
		if path.Ext(file.Name()) == ".json" {
			name := strings.TrimSuffix(path.Base(file.Name()), path.Ext(file.Name()))
			names = append(names, name)
		}
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
		return nil, nil
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
	defer db.Emit(UpdateEvent{Name: name, Filter: filter})

	fd, err := os.Create(db.filterFile(name))
	if err != nil {
		log.Errorf("%v", err)
		return nil
	}
	defer fd.Close()
	if err := json.NewEncoder(fd).Encode(filter); err != nil {
		log.Errorf("%v", err)
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
		return nil
	} else if err != nil {
		return err
	}
	defer db.Emit(UpdateEvent{Name: name, Filter: nil})
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
