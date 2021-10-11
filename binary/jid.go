// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package binary

import (
	"fmt"
	"strconv"
	"strings"

	signalProtocol "github.com/RadicalApp/libsignal-protocol-go/protocol"
)

const (
	DefaultUserServer = "s.whatsapp.net"
	GroupServer       = "g.us"
	UserServer        = "c.us"
	BroadcastServer   = "broadcast"
)

var (
	GroupServerJID      = NewJID("", GroupServer)
	ServerJID           = NewJID("", DefaultUserServer)
	BroadcastServerJID  = NewJID("", BroadcastServer)
	StatusBroadcastJID  = NewJID("status", BroadcastServer)
	PSAJID              = NewJID("0", UserServer)
	OfficialBusinessJID = NewJID("16505361212", UserServer)
)

type FullJID struct {
	User   string
	Agent  uint8
	Device uint8
	Server string
	AD     bool
}

func (jid FullJID) UserInt() uint64 {
	number, _ := strconv.ParseUint(jid.User, 10, 64)
	return number
}

func (jid FullJID) SignalAddress() *signalProtocol.SignalAddress {
	user := jid.User
	if jid.Agent != 0 {
		user = fmt.Sprintf("%s_%d", jid.User, jid.Agent)
	}
	return signalProtocol.NewSignalAddress(user, uint32(jid.Device))
}

func NewADJID(user string, agent, device uint8) FullJID {
	return FullJID{
		User:   user,
		Agent:  agent,
		Device: device,
		Server: DefaultUserServer,
		AD:     true,
	}
}

func ParseJID(jid string) FullJID {
	parts := strings.Split(jid, "@")
	return NewJID(parts[0], parts[1])
}

func NewJID(user, server string) FullJID {
	return FullJID{
		User:   user,
		Server: server,
	}
}

func (jid FullJID) String() string {
	if jid.Agent != 0 || jid.Device != 0 {
		if jid.Agent == 0 {
			return fmt.Sprintf("%s:%d@%s", jid.User, jid.Device, jid.Server)
		} else if jid.Device == 0 {
			return fmt.Sprintf("%s.%d@%s", jid.User, jid.Agent, jid.Server)
		} else {
			return fmt.Sprintf("%s.%d:%d@%s", jid.User, jid.Agent, jid.Device, jid.Server)
		}
	} else if len(jid.User) > 0 {
		return fmt.Sprintf("%s@%s", jid.User, jid.Server)
	} else {
		return jid.Server
	}
}
