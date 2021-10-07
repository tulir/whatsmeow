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
)

const (
	DefaultUserServer  = "s.whatsapp.net"
	DefaultGroupServer = "g.us"
)

var (
	GroupServerJID = NewJID("", DefaultGroupServer)
	ServerJID      = NewJID("", DefaultUserServer)
)

type FullJID struct {
	User   string
	Device uint8
	Agent  uint8
	Server string
	AD     bool
}

func (jid FullJID) UserInt() uint64 {
	number, _ := strconv.ParseUint(jid.User, 10, 64)
	return number
}

func NewADJID(user string, device, agent uint8) FullJID {
	return FullJID{
		User:   user,
		Device: device,
		Agent:  agent,
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
