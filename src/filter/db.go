package filter

import (
	"encoding/json"
	"fmt"
	"reflect"

	"../util"
)

type storageFormat struct {
	Type  string           `json:"type"`
	Value *json.RawMessage `json:"value"`
}

type DB struct {
	util.Emitter
	storage   *util.PersistentStorage
	factories map[string]func() Filter
}

func NewDB(file string, factories ...func() Filter) (*DB, error) {
	namedFactories := map[string]func() Filter{}
	for _, fac := range factories {
		namedFactories[filterType(fac())] = fac
	}

	db := &DB{factories: namedFactories}
	var err error
	db.storage, err = util.NewPersistentStorage(file, &map[string]storageFormat{})
	if err != nil {
		return nil, err
	}
	return db, nil
}

func (db *DB) Filters() map[string]Filter {
	filters := map[string]Filter{}
	for name, data := range *db.storage.Value().(*map[string]storageFormat) {
		fac, ok := db.factories[data.Type]
		if !ok {
			panic(fmt.Errorf("Unknown filter type %q", data.Type))
		}
		ft := fac()
		if err := json.Unmarshal(*data.Value, ft); err != nil {
			panic(err)
		}
		filters[name] = ft
	}
	return filters
}

func (db *DB) Set(name string, filter Filter) error {
	val := db.storage.Value().(*map[string]storageFormat)
	ftVal, err := json.Marshal(filter)
	if err != nil {
		return err
	}
	(*val)[name] = storageFormat{
		Type:  filterType(filter),
		Value: (*json.RawMessage)(&ftVal),
	}
	if err := db.storage.SetValue(val); err != nil {
		return err
	}
	db.Emit("update")
	return nil
}

func (db *DB) Remove(name string) error {
	val := db.storage.Value().(*map[string]storageFormat)
	delete(*val, name)
	if err := db.storage.SetValue(val); err != nil {
		return err
	}
	db.Emit("update")
	return nil
}

func filterType(filter Filter) string {
	return reflect.TypeOf(filter).Elem().PkgPath()
}
