// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package waLog

import (
	"fmt"
	"time"
)

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

// Noop is a no-op logger that silently drops everything.
var Noop Logger = &noopLogger{}

type stdoutLogger struct {
	mod string
}

func (s *stdoutLogger) Outputf(level, msg string, args ...interface{}) {
	fmt.Printf("%s [%s %s] %s\n", time.Now().Format("15:04:05.000"), s.mod, level, fmt.Sprintf(msg, args...))
}

func (s *stdoutLogger) Errorf(msg string, args ...interface{}) { s.Outputf("ERROR", msg, args...) }
func (s *stdoutLogger) Warnf(msg string, args ...interface{})  { s.Outputf("WARN", msg, args...) }
func (s *stdoutLogger) Infof(msg string, args ...interface{})  { s.Outputf("INFO", msg, args...) }
func (s *stdoutLogger) Debugf(msg string, args ...interface{}) { s.Outputf("DEBUG", msg, args...) }
func (s *stdoutLogger) Sub(mod string) Logger {
	return &stdoutLogger{mod: fmt.Sprintf("%s/%s", s.mod, mod)}
}

// Stdout is a simple logger that logs to stdout. The module name given is included in log lines.
func Stdout(module string) Logger {
	return &stdoutLogger{mod: module}
}
