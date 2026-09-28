// Copyright (c) 2026 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package richresponse

import (
	"encoding/json"
	"reflect"
)

type TextEntity struct {
	Key      string             `json:"key"`
	Metadata TextEntityMetadata `json:"metadata"`
}

type textEntityWrapper struct {
	Key      string          `json:"key"`
	Metadata json.RawMessage `json:"metadata"`
}

func (te *TextEntity) UnmarshalJSON(data []byte) error {
	var wrapper textEntityWrapper
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return err
	}
	te.Key = wrapper.Key
	te.Metadata = nil
	if len(wrapper.Metadata) == 0 || string(wrapper.Metadata) == "null" {
		return nil
	}
	val, err := unmarshalWithTypeName[UnknownTextEntityMetadata](wrapper.Metadata, textEntityMetadataTypes)
	if err != nil {
		return err
	}
	te.Metadata = val.(TextEntityMetadata)
	return nil
}

type TextEntityMetadata interface {
	isTextEntityMetadata()
}

var textEntityMetadataTypes = map[string]reflect.Type{
	"GenAISearchCitationItem": reflect.TypeFor[GenAISearchCitationItem](),
	"GenAIInlineLinkItem":     reflect.TypeFor[GenAIInlineLinkItem](),
	"GenAIDeepLinkItem":       reflect.TypeFor[GenAIDeepLinkItem](),
	"GenAILatexItem":          reflect.TypeFor[GenAILatexItem](),
}

func (*GenAISearchCitationItem) isTextEntityMetadata()  {}
func (*GenAIInlineLinkItem) isTextEntityMetadata()      {}
func (*GenAIDeepLinkItem) isTextEntityMetadata()        {}
func (*GenAILatexItem) isTextEntityMetadata()           {}
func (UnknownTextEntityMetadata) isTextEntityMetadata() {}

type GenAISearchCitationItem struct {
	TypeName string `json:"__typename"`
}

type GenAIInlineLinkItem struct {
	TypeName    string `json:"__typename"`
	URL         string `json:"url"`
	DisplayName string `json:"display_name"`
}

type GenAIDeepLinkItem struct {
	TypeName    string `json:"__typename"`
	DeepLinkURL string `json:"deeplink_url"`
	Text        string `json:"text"`
}

type GenAILatexItem struct {
	TypeName        string `json:"__typename"`
	LatexExpression string `json:"latex_expression"`
}

type UnknownTextEntityMetadata json.RawMessage
