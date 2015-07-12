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

	this := &PersistentStorage{
		file:  path.Join(storageDir, name+".json"),
		value: typeValue,
	}

	ok, err := this.readValue()
	if err != nil {
		return nil, err
	}

	if !ok {
		if err := this.SetValue(typeValue); err != nil {
			return nil, err
		}
	}

	return this, nil
}

func (this *PersistentStorage) Value() interface{} {
	return this.value
}

func (this *PersistentStorage) SetValue(value interface{}) error {
	this.value = value

	this.fileLock.Lock()
	defer this.fileLock.Unlock()

	file, err := os.Create(this.file)
	if err != nil {
		return err
	}
	defer file.Close()

	return json.NewEncoder(file).Encode(this.value)
}

func (this *PersistentStorage) readValue() (bool, error) {
	this.fileLock.Lock()
	defer this.fileLock.Unlock()

	file, err := os.Open(this.file)
	if os.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	defer file.Close()

	if err := json.NewDecoder(file).Decode(this.value); err != nil {
		return false, err
	}
	return true, nil
}
