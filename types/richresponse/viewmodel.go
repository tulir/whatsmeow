// Copyright (c) 2026 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package richresponse

import (
	"encoding/json"
	"reflect"
	"strings"

	"go.mau.fi/util/exslices"
)

type RichResponse struct {
	ResponseID      string               `json:"response_id"`
	Sections        []ViewModelContainer `json:"sections"`
	FooterSections  []ViewModelContainer `json:"footer_sections,omitempty"`
	EmbeddedScreens []json.RawMessage    `json:"embedded_screens,omitempty"`
}

type ViewModelContainer struct {
	Model ViewModel
}

func (vmc *ViewModelContainer) UnmarshalJSON(data []byte) error {
	val, err := unmarshalWithTypeName[UnknownViewModel](data, viewModelTypes)
	if err == nil {
		vmc.Model = val.(ViewModel)
	}
	return nil
}

type ViewModel interface {
	isViewModel()
	GetPrimitives() []Primitive
	String() string
}

var viewModelTypes = map[string]reflect.Type{
	"GenAISingleLayoutViewModel":  reflect.TypeFor[GenAISingleLayoutViewModel](),
	"GenAIGridLayoutViewModel":    reflect.TypeFor[GenAIGridLayoutViewModel](),
	"GenAIHScrollLayoutViewModel": reflect.TypeFor[GenAIHScrollLayoutViewModel](),
	"GenAIVStackLayoutViewModel":  reflect.TypeFor[GenAIVStackLayoutViewModel](),
}

var (
	_ ViewModel = (*GenAISingleLayoutViewModel)(nil)
	_ ViewModel = (*MultiLayoutViewModel)(nil)
	_ ViewModel = (UnknownViewModel)(nil)
)

func (*GenAISingleLayoutViewModel) isViewModel() {}
func (*MultiLayoutViewModel) isViewModel()       {}
func (UnknownViewModel) isViewModel()            {}

type GenAISingleLayoutViewModel struct {
	Primitive PrimitiveContainer `json:"primitive"`
}

func (vm *GenAISingleLayoutViewModel) String() string {
	return vm.Primitive.Value.String()
}

func (vm *GenAISingleLayoutViewModel) GetPrimitives() []Primitive {
	return []Primitive{vm.Primitive.Value}
}

type MultiLayoutViewModel struct {
	Primitives []PrimitiveContainer `json:"primitives"`
}

func (vm *MultiLayoutViewModel) String() string {
	return strings.Join(exslices.CastFunc(vm.Primitives, func(from PrimitiveContainer) string {
		return from.Value.String()
	}), "\n")
}

func (vm *MultiLayoutViewModel) GetPrimitives() []Primitive {
	return exslices.CastFunc(vm.Primitives, func(from PrimitiveContainer) Primitive {
		return from.Value
	})
}

type GenAIGridLayoutViewModel = MultiLayoutViewModel
type GenAIHScrollLayoutViewModel = MultiLayoutViewModel
type GenAIVStackLayoutViewModel = MultiLayoutViewModel

type UnknownViewModel json.RawMessage

func (vm UnknownViewModel) GetPrimitives() []Primitive {
	return nil
}

func (vm UnknownViewModel) String() string {
	return ""
}
