package argo

import (
	_ "embed"
	"encoding/json"
	"sync"

	"github.com/beeper/argo-go/wire"
	"github.com/beeper/argo-go/wirecodec"
)

var (
	Store                map[string]wire.Type
	QueryIDToMessageName map[string]string

	//go:embed argo-wire-type-store.argo
	wireTypeStoreBytes []byte

	//go:embed name-to-queryids.json
	jsonMapBytes []byte

	loadOnce sync.Once
	initErr  error
)

func Init() error {
	loadOnce.Do(func() {
		var err error

		Store, err = wirecodec.DecodeWireTypeStoreFile(wireTypeStoreBytes)
		if err != nil {
			initErr = err
			return
		}

		var src map[string]string
		if err := json.Unmarshal(jsonMapBytes, &src); err != nil {
			initErr = err
			return
		}

		m := make(map[string]string, len(src))
		for name, id := range src {
			m[id] = name
		}
		QueryIDToMessageName = m
	})
	return initErr
}

func GetStore() (map[string]wire.Type, error) {
	if err := Init(); err != nil {
		return nil, err
	}
	return Store, nil
}
func GetQueryIDToMessageName() (map[string]string, error) {
	if err := Init(); err != nil {
		return nil, err
	}
	return QueryIDToMessageName, nil
}
