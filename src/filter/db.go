package filter

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"reflect"
	"sort"
	"strings"

	"github.com/polyfloyd/trollibox/src/util"
)

type UpdateEvent struct{}

var factories = map[string]func() Filter{}

// RegisterFactory registers a factory function that enables DB to deserialize
// filters of that type.
//
// The returned value should be the zero value of that type.
func RegisterFactory(factory func() Filter) {
	factories[filterType(factory())] = factory
}

type storageFormat struct {
	Type  string           `json:"type"`
	Value *json.RawMessage `json:"value"`
}

// A DB handles storage of filter implemementations to disk.
type DB struct {
	util.Emitter

	directory string
}

// NewDB constructs a new database for storing filters at the specified
// directory.
//
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
	fd, err := os.Open(db.filterFile(name))
	if os.IsNotExist(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	defer fd.Close()

	var ft storageFormat
	if err := json.NewDecoder(fd).Decode(&ft); err != nil {
		return nil, err
	}

	fac, ok := factories[ft.Type]
	if !ok {
		return nil, fmt.Errorf("Unknown filter type: %s", ft.Type)
	}
	filter := fac()
	if err := json.Unmarshal(([]byte)(*ft.Value), filter); err != nil {
		return nil, err
	}
	return filter, nil
}

// Set stores the specified filter under the specified name overwriting any
// pre-existing filter with the same name.
func (db *DB) Set(name string, filter Filter) error {
	if name == "" || strings.Contains(name, "/") {
		return fmt.Errorf("Invalid filter name: %q", name)
	}

	ftVal, err := json.Marshal(filter)
	if err != nil {
		return err
	}

	fd, err := os.Create(db.filterFile(name))
	if err != nil {
		return err
	}
	defer fd.Close()
	if err := json.NewEncoder(fd).Encode(storageFormat{
		Type:  filterType(filter),
		Value: (*json.RawMessage)(&ftVal),
	}); err != nil {
		return err
	}
	db.Emit(UpdateEvent{})
	return nil
}

// Remove removes the named filter from the database.
//
// Removing a non-existent filter is a no-op.
func (db *DB) Remove(name string) error {
	if err := os.Remove(db.filterFile(name)); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	db.Emit(UpdateEvent{})
	return nil
}

func (db *DB) filterFile(name string) string {
	return path.Join(db.directory, name+".json")
}

func filterType(filter Filter) string {
	typ := reflect.TypeOf(filter)
	if typ.Kind() == reflect.Ptr {
		return typ.Elem().PkgPath()
	}
	return typ.PkgPath()
}
