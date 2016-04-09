package ruled

import (
	"../../util"
)

type DB struct {
	util.Emitter
	storage *util.PersistentStorage
}

func NewDB(file string) (*DB, error) {
	db := &DB{}
	var err error
	if db.storage, err = util.NewPersistentStorage(file, &[]Rule{}); err != nil {
		return nil, err
	}
	// Check the integrity of the rules.
	if err := db.SetRules(db.Rules()); err != nil {
		return nil, err
	}
	return db, nil
}

func (db *DB) Rules() []Rule {
	return *db.storage.Value().(*[]Rule)
}

func (db *DB) SetRules(rules []Rule) error {
	if _, err := BuildFilter(rules); err != nil {
		return err
	}
	db.Emit("update")
	return db.storage.SetValue(&rules)
}
