// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package log

type Logger interface {
	Warn(msg string, args ...interface{})
	Error(msg string, args ...interface{})
}

type noopLogger struct{}

var _ Logger = (*noopLogger)(nil)

func (n noopLogger) Error(_ string, _ ...interface{}) {}
func (n noopLogger) Warn(_ string, _ ...interface{})  {}

// Noop is a no-op logger
var Noop = &noopLogger{}
