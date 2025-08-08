package argo

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/beeper/argo-go/wire"
	"github.com/beeper/argo-go/wirecodec"
)

var Store map[string]wire.Type

var QueryIDToMessageName map[string]string

func init() {
	raw, err := os.ReadFile("argo-wire-type-store.argo")
	if err != nil {
		log.Fatalf("read file: %v", err)
	}
	Store, err = wirecodec.DecodeWireTypeStoreFile(raw)
	if err != nil {
		log.Fatalf("decode wire-type store: %v", err)
	}

	raw2, err := os.ReadFile("name-to-queryids.json")
	if err != nil {
		log.Fatalf("read file: %v", err)
	}
	var src map[string]string
	if err = json.Unmarshal(raw2, &src); err != nil {
		fmt.Errorf("input must be a JSON object of string->string: %w", err)
	}

	QueryIDToMessageName = make(map[string]string, len(src))
	for k, v := range src {
		QueryIDToMessageName[v] = k
	}
}
