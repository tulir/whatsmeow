// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package binary

import (
	"fmt"
	"strconv"
)

type AttrUtility struct {
	Attrs  map[string]interface{}
	Errors []error
}

func (n *Node) AttrGetter() *AttrUtility {
	return &AttrUtility{Attrs: n.Attrs, Errors: make([]error, 0)}
}

func (au *AttrUtility) GetJID(key string, require bool) (jidVal FullJID, ok bool) {
	var val interface{}
	if val, ok = au.Attrs[key]; !ok {
		if require {
			au.Errors = append(au.Errors, fmt.Errorf("didn't find required JID attribute '%s'", key))
		}
	} else if jidVal, ok = val.(FullJID); !ok {
		au.Errors = append(au.Errors, fmt.Errorf("expected attribute '%s' to be JID, but was %T", key, val))
	}
	return
}

func (au *AttrUtility) OptionalJID(key string) *FullJID {
	jid, ok := au.GetJID(key, false)
	if ok {
		return &jid
	}
	return nil
}

func (au *AttrUtility) JID(key string) FullJID {
	jid, _ := au.GetJID(key, true)
	return jid
}

func (au *AttrUtility) GetString(key string, require bool) (strVal string, ok bool) {
	var val interface{}
	if val, ok = au.Attrs[key]; !ok {
		if require {
			au.Errors = append(au.Errors, fmt.Errorf("didn't find required attribute '%s'", key))
		}
	} else if strVal, ok = val.(string); !ok {
		au.Errors = append(au.Errors, fmt.Errorf("expected attribute '%s' to be string, but was %T", key, val))
	}
	return
}

func (au *AttrUtility) GetInt64(key string, require bool) (int64, bool) {
	if strVal, ok := au.GetString(key, require); !ok {
		return 0, false
	} else if intVal, err := strconv.ParseInt(strVal, 10, 64); err != nil {
		au.Errors = append(au.Errors, fmt.Errorf("failed to parse int in attribute '%s': %w", key, err))
		return 0, false
	} else {
		return intVal, true
	}
}

func (au *AttrUtility) GetUint64(key string, require bool) (uint64, bool) {
	if strVal, ok := au.GetString(key, require); !ok {
		return 0, false
	} else if intVal, err := strconv.ParseUint(strVal, 10, 64); err != nil {
		au.Errors = append(au.Errors, fmt.Errorf("failed to parse uint in attribute '%s': %w", key, err))
		return 0, false
	} else {
		return intVal, true
	}
}

func (au *AttrUtility) OptionalString(key string) string {
	strVal, _ := au.GetString(key, false)
	return strVal
}

func (au *AttrUtility) String(key string) string {
	strVal, _ := au.GetString(key, true)
	return strVal
}

func (au *AttrUtility) OptionalInt(key string) int {
	val, _ := au.GetInt64(key, false)
	return int(val)
}

func (au *AttrUtility) Int(key string) int {
	val, _ := au.GetInt64(key, true)
	return int(val)
}

func (au *AttrUtility) Int64(key string) int64 {
	val, _ := au.GetInt64(key, true)
	return val
}

func (au *AttrUtility) Uint64(key string) uint64 {
	val, _ := au.GetUint64(key, true)
	return val
}

func (au *AttrUtility) OK() bool {
	return len(au.Errors) == 0
}
