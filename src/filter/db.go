package filter

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"reflect"
	"strings"

	"github.com/polyfloyd/trollibox/src/util"
)

type storageFormat struct {
	Type  string           `json:"type"`
	Value *json.RawMessage `json:"value"`
}

type DB struct {
	util.Emitter

	directory string
	factories map[string]func() Filter
}

func NewDB(directory string, factories ...func() Filter) (*DB, error) {
	namedFactories := map[string]func() Filter{}
	for _, fac := range factories {
		namedFactories[filterType(fac())] = fac
	}
	if err := os.MkdirAll(directory, 0755); err != nil {
		return nil, err
	}
	db := &DB{
		directory: directory,
		factories: namedFactories,
	}
	return db, nil
}

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
	return names, nil
}

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

	fac, ok := db.factories[ft.Type]
	if !ok {
		return nil, fmt.Errorf("Unknown filter type: %s", ft.Type)
	}
	filter := fac()
	if err := json.Unmarshal(([]byte)(*ft.Value), filter); err != nil {
		return nil, err
	}
	return filter, nil
}

func (db *DB) Store(name string, filter Filter) error {
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
	db.Emit("update")
	return nil
}

func (db *DB) Remove(name string) error {
	if err := os.Remove(db.filterFile(name)); err != nil {
		return err
	}
	db.Emit("update")
	return nil
}

func (db *DB) filterFile(name string) string {
	return path.Join(db.directory, path.Clean(name)+".json")
}

func filterType(filter Filter) string {
	return reflect.TypeOf(filter).Elem().PkgPath()
}
