// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package binary

import (
	"fmt"
	"strconv"
	"time"

	"go.mau.fi/whatsmeow/types"
)

// AttrUtility is a helper struct for reading multiple XML attributes and checking for errors afterwards.
//
// The functions return values directly and append any decoding errors to the Errors slice. The
// slice can then be checked after all necessary attributes are read, instead of having to check
// each attribute for errors separately.
type AttrUtility struct {
	Attrs  Attrs
	Errors []error
}

// AttrGetter returns the AttrUtility for this Node.
func (n *Node) AttrGetter() *AttrUtility {
	return &AttrUtility{Attrs: n.Attrs, Errors: make([]error, 0)}
}

func (au *AttrUtility) GetJID(key string, require bool) (jidVal types.JID, ok bool) {
	var val interface{}
	if val, ok = au.Attrs[key]; !ok {
		if require {
			au.Errors = append(au.Errors, fmt.Errorf("didn't find required JID attribute '%s'", key))
		}
	} else if jidVal, ok = val.(types.JID); !ok {
		au.Errors = append(au.Errors, fmt.Errorf("expected attribute '%s' to be JID, but was %T", key, val))
	}
	return
}

// OptionalJID returns the JID under the given key. If there's no valid JID under the given key, this will return nil.
// However, if the attribute is completely missing, this will not store an error.
func (au *AttrUtility) OptionalJID(key string) *types.JID {
	jid, ok := au.GetJID(key, false)
	if ok {
		return &jid
	}
	return nil
}

// OptionalJIDOrEmpty returns the JID under the given key. If there's no valid JID under the given key, this will return an empty JID.
// However, if the attribute is completely missing, this will not store an error.
func (au *AttrUtility) OptionalJIDOrEmpty(key string) types.JID {
	jid, ok := au.GetJID(key, false)
	if ok {
		return jid
	}
	return types.EmptyJID
}

// JID returns the JID under the given key.
// If there's no valid JID under the given key, an error will be stored and a blank JID struct will be returned.
func (au *AttrUtility) JID(key string) types.JID {
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

func (au *AttrUtility) GetBool(key string, require bool) (bool, bool) {
	if strVal, ok := au.GetString(key, require); !ok {
		return false, false
	} else if boolVal, err := strconv.ParseBool(strVal); err != nil {
		au.Errors = append(au.Errors, fmt.Errorf("failed to parse bool in attribute '%s': %w", key, err))
		return false, false
	} else {
		return boolVal, true
	}
}

func (au *AttrUtility) GetUnixTime(key string, require bool) (time.Time, bool) {
	if intVal, ok := au.GetInt64(key, require); !ok {
		return time.Time{}, false
	} else if intVal == 0 {
		return time.Time{}, true
	} else {
		return time.Unix(intVal, 0), true
	}
}

// OptionalString returns the string under the given key.
func (au *AttrUtility) OptionalString(key string) string {
	strVal, _ := au.GetString(key, false)
	return strVal
}

// String returns the string under the given key.
// If there's no valid string under the given key, an error will be stored and an empty string will be returned.
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

func (au *AttrUtility) OptionalBool(key string) bool {
	val, _ := au.GetBool(key, false)
	return val
}

func (au *AttrUtility) Bool(key string) bool {
	val, _ := au.GetBool(key, true)
	return val
}

func (au *AttrUtility) OptionalUnixTime(key string) time.Time {
	val, _ := au.GetUnixTime(key, false)
	return val
}

func (au *AttrUtility) UnixTime(key string) time.Time {
	val, _ := au.GetUnixTime(key, true)
	return val
}

// OK returns true if there are no errors.
func (au *AttrUtility) OK() bool {
	return len(au.Errors) == 0
}

// Error returns the list of errors as a single error interface, or nil if there are no errors.
func (au *AttrUtility) Error() error {
	if au.OK() {
		return nil
	}
	return ErrorList(au.Errors)
}

// ErrorList is a list of errors that implements the error interface itself.
type ErrorList []error

// Error returns all the errors in the list as a string.
func (el ErrorList) Error() string {
	return fmt.Sprintf("%+v", []error(el))
}
