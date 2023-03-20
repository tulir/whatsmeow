// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package types contains various structs and other types used by whatsmeow.
package types

import (
	"database/sql"
	"database/sql/driver"
	"fmt"
	"strconv"
	"strings"

	signalProtocol "go.mau.fi/libsignal/protocol"
)

// Known JID servers on WhatsApp
const (
	DefaultUserServer = "s.whatsapp.net"
	GroupServer       = "g.us"
	LegacyUserServer  = "c.us"
	BroadcastServer   = "broadcast"
	HiddenUserServer  = "lid"
)

// Some JIDs that are contacted often.
var (
	EmptyJID            = JID{}
	GroupServerJID      = NewJID("", GroupServer)
	ServerJID           = NewJID("", DefaultUserServer)
	BroadcastServerJID  = NewJID("", BroadcastServer)
	StatusBroadcastJID  = NewJID("status", BroadcastServer)
	PSAJID              = NewJID("0", LegacyUserServer)
	OfficialBusinessJID = NewJID("16505361212", LegacyUserServer)
)

// MessageID is the internal ID of a WhatsApp message.
type MessageID = string

// JID represents a WhatsApp user ID.
//
// There are two types of JIDs: regular JID pairs (user and server) and AD-JIDs (user, agent and device).
// AD JIDs are only used to refer to specific devices of users, so the server is always s.whatsapp.net (DefaultUserServer).
// Regular JIDs can be used for entities on any servers (users, groups, broadcasts).
type JID struct {
	User   string
	Agent  uint8
	Device uint8
	Server string
	AD     bool
}

// UserInt returns the user as an integer. This is only safe to run on normal users, not on groups or broadcast lists.
func (jid JID) UserInt() uint64 {
	number, _ := strconv.ParseUint(jid.User, 10, 64)
	return number
}

// ToNonAD returns a version of the JID struct that doesn't have the agent and device set.
func (jid JID) ToNonAD() JID {
	if jid.AD {
		return JID{
			User:   jid.User,
			Server: DefaultUserServer,
		}
	} else {
		return jid
	}
}

// SignalAddress returns the Signal protocol address for the user.
func (jid JID) SignalAddress() *signalProtocol.SignalAddress {
	user := jid.User
	if jid.Agent != 0 {
		user = fmt.Sprintf("%s_%d", jid.User, jid.Agent)
	}
	return signalProtocol.NewSignalAddress(user, uint32(jid.Device))
}

// IsBroadcastList returns true if the JID is a broadcast list, but not the status broadcast.
func (jid JID) IsBroadcastList() bool {
	return jid.Server == BroadcastServer && jid.User != StatusBroadcastJID.User
}

// NewADJID creates a new AD JID.
func NewADJID(user string, agent, device uint8) JID {
	return JID{
		User:   user,
		Agent:  agent,
		Device: device,
		Server: DefaultUserServer,
		AD:     true,
	}
}

func parseADJID(user string) (JID, error) {
	var fullJID JID
	fullJID.AD = true
	fullJID.Server = DefaultUserServer

	dotIndex := strings.IndexRune(user, '.')
	colonIndex := strings.IndexRune(user, ':')
	if dotIndex < 0 || colonIndex < 0 || colonIndex+1 <= dotIndex {
		return fullJID, fmt.Errorf("failed to parse ADJID: missing separators")
	}

	fullJID.User = user[:dotIndex]
	agent, err := strconv.Atoi(user[dotIndex+1 : colonIndex])
	if err != nil {
		return fullJID, fmt.Errorf("failed to parse agent from JID: %w", err)
	} else if agent < 0 || agent > 255 {
		return fullJID, fmt.Errorf("failed to parse agent from JID: invalid value (%d)", agent)
	}
	device, err := strconv.Atoi(user[colonIndex+1:])
	if err != nil {
		return fullJID, fmt.Errorf("failed to parse device from JID: %w", err)
	} else if device < 0 || device > 255 {
		return fullJID, fmt.Errorf("failed to parse device from JID: invalid value (%d)", device)
	}
	fullJID.Agent = uint8(agent)
	fullJID.Device = uint8(device)
	return fullJID, nil
}

// ParseJID parses a JID out of the given string. It supports both regular and AD JIDs.
func ParseJID(jid string) (JID, error) {
	parts := strings.Split(jid, "@")
	if len(parts) == 1 {
		return NewJID("", parts[0]), nil
	} else if strings.ContainsRune(parts[0], ':') && strings.ContainsRune(parts[0], '.') && parts[1] == DefaultUserServer {
		return parseADJID(parts[0])
	}
	return NewJID(parts[0], parts[1]), nil
}

// NewJID creates a new regular JID.
func NewJID(user, server string) JID {
	return JID{
		User:   user,
		Server: server,
	}
}

// String converts the JID to a string representation.
// The output string can be parsed with ParseJID, except for JIDs with no User part specified.
func (jid JID) String() string {
	if jid.AD {
		return fmt.Sprintf("%s.%d:%d@%s", jid.User, jid.Agent, jid.Device, jid.Server)
	} else if len(jid.User) > 0 {
		return fmt.Sprintf("%s@%s", jid.User, jid.Server)
	} else {
		return jid.Server
	}
}

// MarshalText implements encoding.TextMarshaler for JID
func (jid JID) MarshalText() ([]byte, error) {
	return []byte(jid.String()), nil
}

// UnmarshalText implements encoding.TextUnmarshaler for JID
func (jid *JID) UnmarshalText(val []byte) error {
	out, err := ParseJID(string(val))
	if err != nil {
		return err
	}
	*jid = out
	return nil
}

// IsEmpty returns true if the JID has no server (which is required for all JIDs).
func (jid JID) IsEmpty() bool {
	return len(jid.Server) == 0
}

var _ sql.Scanner = (*JID)(nil)

// Scan scans the given SQL value into this JID.
func (jid *JID) Scan(src interface{}) error {
	if src == nil {
		return nil
	}
	var out JID
	var err error
	switch val := src.(type) {
	case string:
		out, err = ParseJID(val)
	case []byte:
		out, err = ParseJID(string(val))
	default:
		err = fmt.Errorf("unsupported type %T for scanning JID", val)
	}
	if err != nil {
		return err
	}
	*jid = out
	return nil
}

// Value returns the string representation of the JID as a value that the SQL package can use.
func (jid JID) Value() (driver.Value, error) {
	if len(jid.Server) == 0 {
		return nil, nil
	}
	return jid.String(), nil
}
