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
	MessengerServer   = "msgr"
	InteropServer     = "interop"
	NewsletterServer  = "newsletter"
	HostedServer      = "hosted"
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
	User       string
	RawAgent   uint8
	Device     uint16
	Integrator uint16
	Server     string
}

func (jid JID) ActualAgent() uint8 {
	switch jid.Server {
	case DefaultUserServer:
		return 0
	case HiddenUserServer:
		return 1
	default:
		return jid.RawAgent
	}
}

// UserInt returns the user as an integer. This is only safe to run on normal users, not on groups or broadcast lists.
func (jid JID) UserInt() uint64 {
	number, _ := strconv.ParseUint(jid.User, 10, 64)
	return number
}

// ToNonAD returns a version of the JID struct that doesn't have the agent and device set.
func (jid JID) ToNonAD() JID {
	return JID{
		User:       jid.User,
		Server:     jid.Server,
		Integrator: jid.Integrator,
	}
}

// SignalAddress returns the Signal protocol address for the user.
func (jid JID) SignalAddress() *signalProtocol.SignalAddress {
	user := jid.User
	agent := jid.ActualAgent()
	if agent != 0 {
		user = fmt.Sprintf("%s_%d", jid.User, agent)
	}
	return signalProtocol.NewSignalAddress(user, uint32(jid.Device))
	// TODO use @lid suffix instead of agent?
	//suffix := ""
	//if jid.Server == HiddenUserServer {
	//	suffix = "@lid"
	//}
	//return signalProtocol.NewSignalAddress(user, uint32(jid.Device), suffix)
}

// IsBroadcastList returns true if the JID is a broadcast list, but not the status broadcast.
func (jid JID) IsBroadcastList() bool {
	return jid.Server == BroadcastServer && jid.User != StatusBroadcastJID.User
}

// NewADJID creates a new AD JID.
func NewADJID(user string, agent, device uint8) JID {
	var server string
	switch agent {
	case 0:
		server = DefaultUserServer
	case 1:
		server = HiddenUserServer
		agent = 0
	default:
		if (agent&0x01) != 0 || (agent&0x80) == 0 { // agent % 2 == 0 || agent < 128?
			// TODO invalid JID?
		}
		server = HostedServer
	}
	return JID{
		User:     user,
		RawAgent: agent,
		Device:   uint16(device),
		Server:   server,
	}
}

// ParseJID parses a JID out of the given string. It supports both regular and AD JIDs.
func ParseJID(jid string) (JID, error) {
	parts := strings.Split(jid, "@")
	if len(parts) == 1 {
		return NewJID("", parts[0]), nil
	}
	parsedJID := JID{User: parts[0], Server: parts[1]}
	if strings.ContainsRune(parsedJID.User, '.') {
		parts = strings.Split(parsedJID.User, ".")
		if len(parts) != 2 {
			return parsedJID, fmt.Errorf("unexpected number of dots in JID")
		}
		parsedJID.User = parts[0]
		ad := parts[1]
		parts = strings.Split(ad, ":")
		if len(parts) > 2 {
			return parsedJID, fmt.Errorf("unexpected number of colons in JID")
		}
		agent, err := strconv.Atoi(parts[0])
		if err != nil {
			return parsedJID, fmt.Errorf("failed to parse device from JID: %w", err)
		}
		parsedJID.RawAgent = uint8(agent)
		if len(parts) == 2 {
			device, err := strconv.Atoi(parts[1])
			if err != nil {
				return parsedJID, fmt.Errorf("failed to parse device from JID: %w", err)
			}
			parsedJID.Device = uint16(device)
		}
	} else if strings.ContainsRune(parsedJID.User, ':') {
		parts = strings.Split(parsedJID.User, ":")
		if len(parts) != 2 {
			return parsedJID, fmt.Errorf("unexpected number of colons in JID")
		}
		parsedJID.User = parts[0]
		device, err := strconv.Atoi(parts[1])
		if err != nil {
			return parsedJID, fmt.Errorf("failed to parse device from JID: %w", err)
		}
		parsedJID.Device = uint16(device)
	}
	return parsedJID, nil
}

// NewJID creates a new regular JID.
func NewJID(user, server string) JID {
	return JID{
		User:   user,
		Server: server,
	}
}

func (jid JID) ADString() string {
	return fmt.Sprintf("%s.%d:%d@%s", jid.User, jid.RawAgent, jid.Device, jid.Server)
}

// String converts the JID to a string representation.
// The output string can be parsed with ParseJID.
func (jid JID) String() string {
	if jid.RawAgent > 0 {
		return fmt.Sprintf("%s.%d:%d@%s", jid.User, jid.RawAgent, jid.Device, jid.Server)
	} else if jid.Device > 0 {
		return fmt.Sprintf("%s:%d@%s", jid.User, jid.Device, jid.Server)
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
