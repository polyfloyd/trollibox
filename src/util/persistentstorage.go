package util

import (
	"encoding/json"
	"os"
	"sync"
)

type PersistentStorage struct {
	value    interface{}
	file     string
	fileLock sync.Mutex
}

func NewPersistentStorage(filename string, typeValue interface{}) (*PersistentStorage, error) {
	store := &PersistentStorage{
		file:  filename,
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
	store.fileLock.Lock()
	defer store.fileLock.Unlock()

	store.value = value
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
