// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package waLog contains a simple logger interface used by the other whatsmeow packages.
package waLog

import (
	"fmt"
	"time"
)

// Logger is a simple logger interface that can have subloggers for specific areas.
type Logger interface {
	Warnf(msg string, args ...interface{})
	Errorf(msg string, args ...interface{})
	Infof(msg string, args ...interface{})
	Debugf(msg string, args ...interface{})
	Sub(module string) Logger
}

type noopLogger struct{}

func (n *noopLogger) Errorf(_ string, _ ...interface{}) {}
func (n *noopLogger) Warnf(_ string, _ ...interface{})  {}
func (n *noopLogger) Infof(_ string, _ ...interface{})  {}
func (n *noopLogger) Debugf(_ string, _ ...interface{}) {}
func (n *noopLogger) Sub(_ string) Logger               { return n }

// Noop is a no-op Logger implementation that silently drops everything.
var Noop Logger = &noopLogger{}

type stdoutLogger struct {
	mod   string
	color bool
}

var colors = map[string]string{
	"INFO": "\033[36m",
	"WARN": "\033[33m",
	"ERROR": "\033[31m",
}

func (s *stdoutLogger) outputf(level, msg string, args ...interface{}) {
	var colorStart, colorReset string
	if s.color {
		colorStart = colors[level]
		colorReset = "\033[0m"
	}
	fmt.Printf("%s%s [%s %s] %s%s\n", time.Now().Format("15:04:05.000"), colorStart, s.mod, level, fmt.Sprintf(msg, args...), colorReset)
}

func (s *stdoutLogger) Errorf(msg string, args ...interface{}) { s.outputf("ERROR", msg, args...) }
func (s *stdoutLogger) Warnf(msg string, args ...interface{})  { s.outputf("WARN", msg, args...) }
func (s *stdoutLogger) Infof(msg string, args ...interface{})  { s.outputf("INFO", msg, args...) }
func (s *stdoutLogger) Debugf(msg string, args ...interface{}) { s.outputf("DEBUG", msg, args...) }
func (s *stdoutLogger) Sub(mod string) Logger {
	return &stdoutLogger{mod: fmt.Sprintf("%s/%s", s.mod, mod), color: s.color}
}

// Stdout is a simple Logger implementation that outputs to stdout. The module name given is included in log lines.
func Stdout(module string, color bool) Logger {
	return &stdoutLogger{mod: module, color: color}
}
