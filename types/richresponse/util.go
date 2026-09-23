// Copyright (c) 2026 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package richresponse

import (
	"bytes"
	"encoding/json"
	"reflect"
)

type TypeNameContainer struct {
	TypeName string `json:"__typename"`
}

func unmarshalWithTypeName[Unknown ~[]byte](data []byte, types map[string]reflect.Type) (any, error) {
	var tnc TypeNameContainer
	if err := json.Unmarshal(data, &tnc); err != nil {
		return nil, err
	}
	typ, ok := types[tnc.TypeName]
	if !ok {
		return Unknown(bytes.Clone(data)), nil
	}
	val := reflect.New(typ).Interface()
	if err := json.Unmarshal(data, val); err != nil {
		return nil, err
	}
	return val, nil
}
