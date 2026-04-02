// Copyright (c) 2025 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"crypto/hmac"
	"crypto/sha256"
	_ "embed"
	"encoding/binary"
	"encoding/json"
	"sort"
	"sync"

	"go.mau.fi/util/exerrors"
	"go.mau.fi/util/exstrings"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
)

//go:embed reportingfields.json
var reportingFieldsJSON string
var getReportingFields = sync.OnceValue(func() (output []reportingField) {
	exerrors.PanicIfNotNil(json.Unmarshal(exstrings.UnsafeBytes(reportingFieldsJSON), &output))
	return
})

type reportingField struct {
	FieldNumber int              `json:"f"`
	IsMessage   bool             `json:"m,omitempty"`
	Subfields   []reportingField `json:"s,omitempty"`
}

func (cli *Client) shouldIncludeReportingToken(message *waE2E.Message) bool {
	if !cli.SendReportingTokens {
		return false
	}
	return message.ReactionMessage == nil &&
		message.EncReactionMessage == nil &&
		message.EncEventResponseMessage == nil &&
		message.PollUpdateMessage == nil
}

func (cli *Client) getMessageReportingToken(
	msgProtobuf []byte,
	msg *waE2E.Message,
	senderJID, remoteJID types.JID,
	messageID types.MessageID,
) waBinary.Node {
	reportingSecret, _ := generateMsgSecretKey(
		EncSecretReportToken, remoteJID, messageID, senderJID,
		msg.GetMessageContextInfo().GetMessageSecret(),
	)
	hasher := hmac.New(sha256.New, reportingSecret)
	hasher.Write(getReportingToken(msgProtobuf))
	return waBinary.Node{
		Tag: "reporting",
		Content: []waBinary.Node{{
			Tag:     "reporting_token",
			Attrs:   waBinary.Attrs{"v": "2"},
			Content: hasher.Sum(nil)[:16],
		}},
	}
}

func getReportingToken(messageProtobuf []byte) []byte {
	return extractReportingTokenContent(messageProtobuf, getReportingFields())
}

// Helper to find config for a field number
func getConfigForField(fields []reportingField, fieldNum int) *reportingField {
	for i := range fields {
		if fields[i].FieldNumber == fieldNum {
			return &fields[i]
		}
	}
	return nil
}

// Protobuf wire types
const (
	wireVarint = 0
	wire64bit  = 1
	wireBytes  = 2
	wire32bit  = 5
)

// Extracts the reporting token content recursively
func extractReportingTokenContent(data []byte, config []reportingField) []byte {
	type field struct {
		Num   int
		Bytes []byte
	}
	var fields []field
	i := 0
	for i < len(data) {
		// Read tag (varint)
		tag, tagLen := binary.Uvarint(data[i:])
		if tagLen <= 0 {
			break // malformed
		}
		fieldNum := int(tag >> 3)
		wireType := int(tag & 0x7)
		fieldCfg := getConfigForField(config, fieldNum)
		fieldStart := i
		i += tagLen
		if fieldCfg == nil {
			// Skip field
			switch wireType {
			case wireVarint:
				_, n := binary.Uvarint(data[i:])
				i += n
			case wire64bit:
				i += 8
			case wireBytes:
				l, n := binary.Uvarint(data[i:])
				i += n + int(l)
			case wire32bit:
				i += 4
			default:
				return nil
			}
			continue
		}
		switch wireType {
		case wireVarint:
			_, n := binary.Uvarint(data[i:])
			i += n
			fields = append(fields, field{Num: fieldNum, Bytes: data[fieldStart:i]})
		case wire64bit:
			i += 8
			fields = append(fields, field{Num: fieldNum, Bytes: data[fieldStart:i]})
		case wireBytes:
			l, n := binary.Uvarint(data[i:])
			valStart := i + n
			valEnd := valStart + int(l)
			if fieldCfg.IsMessage || len(fieldCfg.Subfields) > 0 {
				// Recursively extract subfields
				sub := extractReportingTokenContent(data[valStart:valEnd], fieldCfg.Subfields)
				if len(sub) > 0 {
					// Re-encode tag and length
					buf := make([]byte, 0, tagLen+n+len(sub))
					tagBuf := make([]byte, binary.MaxVarintLen64)
					tagN := binary.PutUvarint(tagBuf, tag)
					lenBuf := make([]byte, binary.MaxVarintLen64)
					lenN := binary.PutUvarint(lenBuf, uint64(len(sub)))
					buf = append(buf, tagBuf[:tagN]...)
					buf = append(buf, lenBuf[:lenN]...)
					buf = append(buf, sub...)
					fields = append(fields, field{Num: fieldNum, Bytes: buf})
				}
			} else {
				fields = append(fields, field{Num: fieldNum, Bytes: data[fieldStart:valEnd]})
			}
			i = valEnd
		case wire32bit:
			i += 4
			fields = append(fields, field{Num: fieldNum, Bytes: data[fieldStart:i]})
		default:
			return nil
		}
	}
	// Sort by field number
	sort.Slice(fields, func(i, j int) bool { return fields[i].Num < fields[j].Num })
	// Concatenate
	var out []byte
	for _, f := range fields {
		out = append(out, f.Bytes...)
	}
	return out
}
