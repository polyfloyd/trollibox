package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strings"
	"sync"
)

var storageDir string

func SetStorageDir(dir string) error {
	dir = strings.Replace(dir, "~", os.Getenv("HOME"), 1)

	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	storageDir = dir
	return nil
}

type PersistentStorage struct {
	value    interface{}
	file     string
	fileLock sync.Mutex
}

func NewPersistentStorage(name string, typeValue interface{}) (*PersistentStorage, error) {
	if storageDir == "" {
		return nil, fmt.Errorf("Storage dir unset")
	}

	store := &PersistentStorage{
		file:  path.Join(storageDir, name+".json"),
		value: typeValue,
	}

	ok, err := store.readValue()
	if err != nil {
		return nil, err
	}

	if !ok {
		if err := store.SetValue(typeValue); err != nil {
			return nil, err
		}
	}

	return store, nil
}

func (store *PersistentStorage) Value() interface{} {
	return store.value
}

func (store *PersistentStorage) SetValue(value interface{}) error {
	store.value = value

	store.fileLock.Lock()
	defer store.fileLock.Unlock()

	file, err := os.Create(store.file)
	if err != nil {
		return err
	}
	defer file.Close()

	return json.NewEncoder(file).Encode(store.value)
}

func (store *PersistentStorage) readValue() (bool, error) {
	store.fileLock.Lock()
	defer store.fileLock.Unlock()

	file, err := os.Open(store.file)
	if os.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	defer file.Close()

	if err := json.NewDecoder(file).Decode(store.value); err != nil {
		return false, err
	}
	return true, nil
}
