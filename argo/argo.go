package argo

import (
	_ "embed"
	"encoding/json"
	"fmt"

	"github.com/beeper/argo-go/wire"
	"github.com/beeper/argo-go/wirecodec"
)

var Store map[string]wire.Type

var QueryIDToMessageName map[string]string

//go:embed argo-wire-type-store.argo
var wireTypeStoreBytes []byte

//go:embed name-to-queryids.json
var jsonMapBytes []byte

func init() {
	var err error
	Store, err = wirecodec.DecodeWireTypeStoreFile(wireTypeStoreBytes)
	if err != nil {
		fmt.Errorf("decode wire-type store: %v", err)
	}

	var src map[string]string
	if err = json.Unmarshal(jsonMapBytes, &src); err != nil {
		fmt.Errorf("input must be a JSON object of string->string: %w", err)
	}

	QueryIDToMessageName = make(map[string]string, len(src))
	for k, v := range src {
		QueryIDToMessageName[v] = k
	}
}
