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

type TextEntity struct {
	Key      string             `json:"key"`
	Metadata TextEntityMetadata `json:"metadata"`
}

type textEntityMetaType struct {
	TextEntity
	Metadata TypeNameContainer `json:"metadata"`
}

func (te *TextEntity) UnmarshalJSON(data []byte) error {
	var metaType textEntityMetaType
	if err := json.Unmarshal(data, &metaType); err != nil {
		return err
	}

	typ, ok := textEntityMetadataTypes[metaType.Metadata.TypeName]
	if !ok {
		*te = metaType.TextEntity
		te.Metadata = UnknownTextEntityMetadata(bytes.Clone(data))
		return nil
	}
	te.Metadata = reflect.New(typ).Interface().(TextEntityMetadata)
	return json.Unmarshal(data, &te)
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
}

type GenAIInlineLinkItem struct {
	URL         string `json:"url"`
	DisplayName string `json:"display_name"`
}

type GenAIDeepLinkItem struct {
	DeepLinkURL string `json:"deeplink_url"`
	Text        string `json:"text"`
}

type GenAILatexItem struct {
	LatexExpression string `json:"latex_expression"`
}

type UnknownTextEntityMetadata json.RawMessage
