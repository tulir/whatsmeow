// Copyright (c) 2026 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package richresponse

import (
	"encoding/json"
	"reflect"
	"regexp"
	"strings"

	"go.mau.fi/util/exslices"
)

type PrimitiveContainer struct {
	Value Primitive
}

func (pc *PrimitiveContainer) UnmarshalJSON(data []byte) error {
	val, err := unmarshalWithTypeName[UnknownPrimitive](data, primitiveTypes)
	if err == nil {
		pc.Value = val.(Primitive)
	}
	return nil
}

type Primitive interface {
	isPrimitive()
	String() string
}

var primitiveTypes = map[string]reflect.Type{
	"GenAICodeUXPrimitive":            reflect.TypeFor[GenAICodeUXPrimitive](),
	"GenAIMarkdownTextUXPrimitive":    reflect.TypeFor[GenAIMarkdownTextUXPrimitive](),
	"GenATableUXPrimitive":            reflect.TypeFor[GenATableUXPrimitive](),
	"GenAIBotProgressStatusPrimitive": reflect.TypeFor[GenAIBotProgressStatusPrimitive](),
	"GenAILatexUXPrimitive":           reflect.TypeFor[GenAILatexUXPrimitive](),
	"FOATextPrimitive":                reflect.TypeFor[FOATextPrimitive](),
	"GenAIMetadataTextPrimitive":      reflect.TypeFor[GenAIMetadataTextPrimitive](),
	"GenAIBotThinkingStatusPrimitive": reflect.TypeFor[GenAIBotThinkingStatusPrimitive](),
	"GenAIProductItemCardPrimitive":   reflect.TypeFor[GenAIProductItemCardPrimitive](),
	"GenAIImagePrimitive":             reflect.TypeFor[GenAIImagePrimitive](),
	"GenAITaskPrimitive":              reflect.TypeFor[GenAITaskPrimitive](),
	"GenAIReelPrimitive":              reflect.TypeFor[GenAIReelPrimitive](),
	"GenAIPostPrimitive":              reflect.TypeFor[GenAIPostPrimitive](),
	"GenAIImaginePrimitive":           reflect.TypeFor[GenAIImaginePrimitive](),
	"GenAISearchResultPrimitive":      reflect.TypeFor[GenAISearchResultPrimitive](),
	"FOABloksPrimitive":               reflect.TypeFor[FOABloksPrimitive](),
	"GenAIDividerPrimitive":           reflect.TypeFor[GenAIDividerPrimitive](),
	"GenAISpacerPrimitive":            reflect.TypeFor[GenAISpacerPrimitive](),
}

var (
	_ Primitive = (*GenAICodeUXPrimitive)(nil)
	_ Primitive = (*GenAIMarkdownTextUXPrimitive)(nil)
	_ Primitive = (*GenATableUXPrimitive)(nil)
	_ Primitive = (*GenAIBotProgressStatusPrimitive)(nil)
	_ Primitive = (*GenAILatexUXPrimitive)(nil)
	_ Primitive = (*FOATextPrimitive)(nil)
	_ Primitive = (*GenAIMetadataTextPrimitive)(nil)
	_ Primitive = (*GenAIBotThinkingStatusPrimitive)(nil)
	_ Primitive = (*GenAIProductItemCardPrimitive)(nil)
	_ Primitive = (*GenAIImagePrimitive)(nil)
	_ Primitive = (*GenAITaskPrimitive)(nil)
	_ Primitive = (*GenAIReelPrimitive)(nil)
	_ Primitive = (*GenAIPostPrimitive)(nil)
	_ Primitive = (*GenAIImaginePrimitive)(nil)
	_ Primitive = (*GenAISearchResultPrimitive)(nil)
	_ Primitive = (*FOABloksPrimitive)(nil)
	_ Primitive = (*GenAIDividerPrimitive)(nil)
	_ Primitive = (*GenAISpacerPrimitive)(nil)
	_ Primitive = (UnknownPrimitive)(nil)
)

func (*GenAICodeUXPrimitive) isPrimitive()            {}
func (*GenAIMarkdownTextUXPrimitive) isPrimitive()    {}
func (*GenATableUXPrimitive) isPrimitive()            {}
func (*GenAILatexUXPrimitive) isPrimitive()           {}
func (*FOATextPrimitive) isPrimitive()                {}
func (*GenAIMetadataTextPrimitive) isPrimitive()      {}
func (*GenAIBotThinkingStatusPrimitive) isPrimitive() {}
func (*GenAIProductItemCardPrimitive) isPrimitive()   {}
func (*GenAIImagePrimitive) isPrimitive()             {}
func (*GenAITaskPrimitive) isPrimitive()              {}
func (*GenAIReelPrimitive) isPrimitive()              {}
func (*GenAIPostPrimitive) isPrimitive()              {}
func (*GenAIImaginePrimitive) isPrimitive()           {}
func (*GenAISearchResultPrimitive) isPrimitive()      {}
func (*FOABloksPrimitive) isPrimitive()               {}
func (*GenAIDividerPrimitive) isPrimitive()           {}
func (*GenAISpacerPrimitive) isPrimitive()            {}
func (UnknownPrimitive) isPrimitive()                 {}

type CodeBlockType string

const (
	CodeBlockTypeComment CodeBlockType = "COMMENT"
	CodeBlockTypeDefault CodeBlockType = "DEFAULT"
	CodeBlockTypeKeyword CodeBlockType = "KEYWORD"
	CodeBlockTypeMethod  CodeBlockType = "METHOD"
	CodeBlockTypeNumber  CodeBlockType = "NUMBER"
	CodeBlockTypeString  CodeBlockType = "STR"
)

type CodeBlock struct {
	Content string        `json:"content"`
	Type    CodeBlockType `json:"type"`
}

func (cb CodeBlock) GetContent() string {
	return cb.Content
}

type GenAICodeUXPrimitive struct {
	Language   string      `json:"language"`
	CodeBlocks []CodeBlock `json:"code_blocks"`
}

func (p *GenAICodeUXPrimitive) String() string {
	return strings.Join(exslices.CastFunc(p.CodeBlocks, CodeBlock.GetContent), "")
}

type GenAIMarkdownTextUXPrimitive struct {
	Text           string       `json:"text"`
	InlineEntities []TextEntity `json:"inline_entities,omitempty"`
}

var placeholderCleaner = regexp.MustCompile(`\{\{([^\s}]+)}}([\s\S]*?)\{\{/[^\s}]+}}`)

func cleanPlaceholders(text string) string {
	return placeholderCleaner.ReplaceAllString(text, "$2")
}

func (p *GenAIMarkdownTextUXPrimitive) String() string {
	return cleanPlaceholders(p.Text)
}

type TableRow struct {
	Cells         []string                       `json:"cells"`
	MarkdownCells []GenAIMarkdownTextUXPrimitive `json:"markdown_cells,omitempty"`
	IsHeader      bool                           `json:"is_header,omitzero"`
}

func (tr *TableRow) String() string {
	return strings.Join(tr.Cells, " | ")
}

type GenATableUXPrimitive struct {
	Rows []*TableRow `json:"rows"`
}

func (p *GenATableUXPrimitive) String() string {
	return strings.Join(exslices.CastFunc(p.Rows, (*TableRow).String), "\n")
}

type GenAIBotProgressStatusPrimitive = GenAIBotThinkingStatusPrimitive

type GenAILatexUXPrimitive struct {
	LatexExpression string `json:"latex_expression,omitzero"`
	Item            struct {
		LatexExpression string `json:"latex_expression,omitzero"`
	} `json:"item,omitzero"`
}

func (p *GenAILatexUXPrimitive) String() string {
	return p.GetLatexExpression()
}

func (p *GenAILatexUXPrimitive) GetLatexExpression() string {
	if p.Item.LatexExpression != "" {
		return p.Item.LatexExpression
	}
	return p.LatexExpression
}

type FOATextPrimitive struct {
	Text string `json:"text,omitempty"` // Markdown text
}

func (p *FOATextPrimitive) String() string {
	return cleanPlaceholders(p.Text)
}

type GenAIMetadataTextPrimitive struct {
	Text string `json:"text"`
}

func (p *GenAIMetadataTextPrimitive) String() string {
	return p.Text
}

type GenAIBotThinkingStatusPrimitive struct {
	Title        string `json:"title"`
	IsInProgress bool   `json:"is_in_progress"`
	// icon
	// meta_search_apps
	// target_secondary_screen_id
	// target_secondary_screen_tab_id
}

func (p *GenAIBotThinkingStatusPrimitive) String() string {
	return p.Title
}

type GenAIProductItemCardPrimitive struct {
	Title string `json:"title"`
}

func (p *GenAIProductItemCardPrimitive) String() string {
	return p.Title
}

type MediaItem struct {
	URL         string `json:"url,omitempty"`
	URLFallback string `json:"url_fallback,omitempty"`
	Width       int    `json:"width,omitzero"`
	Height      int    `json:"height,omitzero"`
	MimeType    string `json:"mime_type,omitempty"`
}

type GenAIImagePrimitive struct {
	FullImage    *MediaItem `json:"full_image"`
	PreviewImage *MediaItem `json:"preview_image"`
}

func (p *GenAIImagePrimitive) String() string {
	return ""
}

type GenAITaskPrimitive struct {
	TaskID   string `json:"task_id"`
	Title    string `json:"title"`
	Subtitle string `json:"subtitle"`
	Status   string `json:"status"`
}

func (p *GenAITaskPrimitive) String() string {
	return p.Title
}

type GenAIReelPrimitive struct {
	// reels_url, thumbnail_url, avatar_url, creator, reels_title
}

func (p *GenAIReelPrimitive) String() string {
	return ""
}

type GenAIPostPrimitive struct {
	// post_url, post_deeplink, post_caption, post_type, orientation, source_app,
	// title, subtitle, username, is_verified, is_carousel, profile_picture_url,
	// thumbnail_url, footer_icon, footer_label, likes_count, comments_count,
	// shares_count
}

func (p *GenAIPostPrimitive) String() string {
	return ""
}

type ImagineType string

const (
	ImagineTypeAnimate ImagineType = "ANIMATE"
	ImagineTypeImagine ImagineType = "IMAGINE"
	ImagineTypeMemu    ImagineType = "MEMU"
)

type ImagineStatus string

const (
	ImagineStatusFailed     ImagineStatus = "FAILED"
	ImagineStatusGenerating ImagineStatus = "GENERATING"
	ImagineStatusReady      ImagineStatus = "READY"
)

type GenAIImaginePrimitive struct {
	ImagineType ImagineType `json:"imagine_type"`
	Media       *MediaItem  `json:"media"`
	Status      struct {
		Status     ImagineStatus `json:"status"`
		UpdateText string        `json:"update_text,omitempty"`
	} `json:"status,omitzero"`
}

func (p *GenAIImaginePrimitive) String() string {
	return ""
}

type GenAISearchResultPrimitive struct {
}

func (p *GenAISearchResultPrimitive) String() string {
	return ""
}

type FOABloksPrimitive struct {
	Type string `json:"type"`
	Data string `json:"data"`
	UUID string `json:"uuid"`
	// initial_response
	// versioning_id
}

func (p *FOABloksPrimitive) String() string {
	return ""
}

type DividerType string

const (
	DividerTypeDot            DividerType = "DOT"
	DividerTypeHorizontalLine DividerType = "HORIZONTAL_LINE"
)

type GenAIDividerPrimitive struct {
	Type DividerType `json:"type"`
}

func (p *GenAIDividerPrimitive) String() string {
	return ""
}

type GenAISpacerPrimitive struct {
	Spacing int `json:"spacing"`
}

func (p *GenAISpacerPrimitive) String() string {
	return ""
}

type UnknownPrimitive json.RawMessage

func (UnknownPrimitive) String() string { return "" }
