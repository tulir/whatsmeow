// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package waLog contains a simple logger interface used by the other whatsmeow packages.
package waLog

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	// Timestamp format
	timeFormat = "15:04:05.000"

	DebugLevel = "DEBUG" // Loggers initialized with DebugLevel will output Debugf(), Infof(), Warnf() and Errorf().
	InfoLevel  = "INFO"  // Loggers initialized with InfoLevel will output Infof(), Warnf() and Errorf().
	WarnLevel  = "WARN"  // Loggers initialized with WarnLevel will output Warnf() and Errorf().
	ErrorLevel = "ERROR" // Loggers initialized with ErrorLevel will output Errorf().
)

// Logger is a simple logger interface that can have subloggers for specific areas.
type Logger interface {
	Warnf(msg string, args ...interface{}) error
	Errorf(msg string, args ...interface{}) error
	Infof(msg string, args ...interface{}) error
	Debugf(msg string, args ...interface{}) error
	Sub(module string) Logger
	Close() error
}

type noopLogger struct{}

func (n *noopLogger) Errorf(_ string, _ ...interface{}) error { return nil }
func (n *noopLogger) Warnf(_ string, _ ...interface{}) error  { return nil }
func (n *noopLogger) Infof(_ string, _ ...interface{}) error  { return nil }
func (n *noopLogger) Debugf(_ string, _ ...interface{}) error { return nil }
func (n *noopLogger) Sub(_ string) Logger                     { return n }
func (n *noopLogger) Close() error                            { return nil }

// Noop is a no-op Logger implementation that silently drops everything.
var Noop Logger = &noopLogger{}

type stdoutLogger struct {
	mod   string
	color bool
	min   int
}

var colors = map[string]string{
	InfoLevel:  "\033[36m",
	WarnLevel:  "\033[33m",
	ErrorLevel: "\033[31m",
}

var levelToInt = map[string]int{
	"":         -1,
	DebugLevel: 0,
	InfoLevel:  1,
	WarnLevel:  2,
	ErrorLevel: 3,
}

func (s *stdoutLogger) outputf(level, msg string, args ...interface{}) {
	if !shouldOutput(s.min, level) {
		return
	}
	var colorStart, colorReset string
	if s.color {
		colorStart = colors[level]
		colorReset = "\033[0m"
	}
	fmt.Printf("%s%s [%s %s] %s%s\n", timestamp(), colorStart, s.mod, level, fmt.Sprintf(msg, args...), colorReset)
}

// Errorf outputs an error message, regardless of the minimum level of the logger.
// The returned error is always nil.
func (s *stdoutLogger) Errorf(msg string, args ...interface{}) error {
	s.outputf(ErrorLevel, msg, args...)
	return nil
}

// Warnf outputs a warning message when the minimum level of the logger is above ErrorLevel.
// The returned error is alwyas nil.
func (s *stdoutLogger) Warnf(msg string, args ...interface{}) error {
	s.outputf(WarnLevel, msg, args...)
	return nil
}

// Infof outputs an informational message when minimum level of the logger is above
// WarnLevel. The returned error is always nil.
func (s *stdoutLogger) Infof(msg string, args ...interface{}) error {
	s.outputf(InfoLevel, msg, args...)
	return nil
}

// Debugf outputs a debug message when the minimum level of the logger is above
// InfoLevel. The returned error is always nil.
func (s *stdoutLogger) Debugf(msg string, args ...interface{}) error {
	s.outputf(DebugLevel, msg, args...)
	return nil
}

// Sub returns a sub-logger which uses the passed-in module name as a tag.
func (s *stdoutLogger) Sub(mod string) Logger {
	return &stdoutLogger{mod: sub(s.mod, mod), color: s.color, min: s.min}
}

// Close is a no-op for the Stdout logger.
func (s *stdoutLogger) Close() error { return nil }

// Stdout is a simple Logger implementation that outputs to stdout. The module name given is
// included in log lines.
//
// If color is true, then info, warn and error logs will be colored cyan, yellow and red
// respectively using ANSI color escape codes.
//
// The minLevel is the minimum level to log and can be DebugLevel, InfoLevel, WarnLevel or
// ErrorLevel.
func Stdout(module string, minLevel string, color bool) Logger {
	return &stdoutLogger{mod: module, color: color, min: levelToInt[strings.ToUpper(minLevel)]}
}

type fileLogger struct {
	// Input parameters
	mod      string // module name
	min      int    // minimal level to log
	filename string // output file (incase of reopening)
	reopen   bool   // when true, the output file is reopened when it disappears
	// Internal and inherited into each sub module
	writer   io.WriteCloser // output channel
	mu       *sync.Mutex    // shared mutex by all copies
	refCount *int           // shared count of loggers using this writer
	openbits int            // how to reopen (create or append)
}

// File returns a Logger implementation that outputs to a file. Similar to the Stdout()
// logger, the module name and timestamp are included in the logs.
// The minLevel is the minimum level to log and can be DebugLevel, InfoLevel, WarnLevel or
// ErrorLevel.
//
// When reopen is true, then a new output file is created incase it disappears. This slows
// down logging, but external scripts like "logrotate" can be used to keep the file size
// in check.
//
// When append is true, then the output file is appended. The default is to overwrite.
func File(module, minLevel, filename string, reopen, append bool) (Logger, error) {
	refCount := 1
	l := &fileLogger{
		mod:      module,
		min:      levelToInt[strings.ToUpper(minLevel)],
		filename: filename,
		reopen:   reopen,
		mu:       &sync.Mutex{},
		refCount: &refCount,
		openbits: os.O_CREATE | os.O_WRONLY,
	}
	if reopen {
		l.openbits |= os.O_APPEND
	}
	var err error
	l.writer, err = os.OpenFile(l.filename, l.openbits, 0644)
	if err != nil {
		return nil, err
	}
	return l, nil
}

// Close shuts down the file logger.
func (l *fileLogger) Close() error {
	refCount := l.refCount
	*refCount--
	if *refCount == 0 {
		return l.writer.Close()
	}
	return nil
}

// Errorf outputs an error message, regardless of the minimum level of the logger.
// The returned error is non-nil when reopening of the log file fails.
func (l *fileLogger) Errorf(msg string, args ...interface{}) error {
	return l.outputf(ErrorLevel, msg, args...)
}

// Warnf outputs a warning message when the minimum level of the logger is aboove ErrorLevel.
// The returned error is non-nil when reopening of the log file fails.
func (l *fileLogger) Warnf(msg string, args ...interface{}) error {
	return l.outputf(WarnLevel, msg, args...)
}

// Infof outputs an informational message when minimum level of the logger is above
// WarnLevel. The returned error is non-nil when reopening of the log file fails.
func (l *fileLogger) Infof(msg string, args ...interface{}) error {
	return l.outputf(InfoLevel, msg, args...)
}

// Debugf outputs a debug message when the minimum level of the logger is above InfoLevel.
// The returned error is non-nil when reopening of the log file fails.
func (l *fileLogger) Debugf(msg string, args ...interface{}) error {
	return l.outputf(DebugLevel, msg, args...)
}

// Sub returns a new file logger with an extended module name, which is used in the logging
// output. Module names of sub loggers are slash-separated appended to the module names of
// "parent" loggers.
//
// Example:
//
//	l1, err := File("", "WARN", "/tmp/wa.log", true, true)
//	l1.Warnf("a warning")  // no module name will be in the log statement
//	l2 := l1.Sub("mod")
//	l2.Warnf("a warning")  // module name "mod" will appear
//	l3 := l2.Sub("sub")
//	l3.Warnf("a warning")  // module name "mod/sub" will appear
func (l *fileLogger) Sub(module string) Logger {
	refCount := l.refCount
	*refCount++
	return &fileLogger{
		mod:      sub(l.mod, module),
		min:      l.min,
		filename: l.filename,
		reopen:   l.reopen,
		writer:   l.writer,
		mu:       l.mu,
		refCount: l.refCount,
		openbits: l.openbits,
	}
}

// outputf is a helper for the file logger to, if necessary, reopen the output and.
func (l *fileLogger) outputf(level, msg string, args ...interface{}) error {
	if !shouldOutput(l.min, level) {
		return nil
	}

	// Don't use closed loggers.
	if refCount := l.refCount; *refCount == 0 {
		return errors.New("logger is closed, cannot send output")
	}

	// Lock all IO to this writer.
	(*l.mu).Lock()
	defer (*l.mu).Unlock()

	// Check that the file is still there.
	_, err := os.Stat(l.filename)
	if err != nil {
		l.writer.Close()
		l.writer, err = os.OpenFile(l.filename, l.openbits, 0644)
		if err != nil {
			return nil
		}
	}
	txt := fmt.Sprintf(msg, args...)
	mod := l.mod
	if mod != "" {
		mod += " "
	}
	l.writer.Write([]byte(fmt.Sprintf("%s [%s%s] %s\n", timestamp(), mod, level, txt)))
	return nil
}

// sub is a helper to consistently propagate the name of a submodule for all loggers.
func sub(existing, new string) string {
	out := existing
	if out != "" && new != "" {
		out += "/"
	}
	out += new
	return out
}

// timestamp is a helper to return a consistently formatted time stamp for all loggers.
func timestamp() string {
	return time.Now().Format(timeFormat)
}

// shouldOutput returns true when the the logger's level vs. the message's level indicates
// that the log should be sent. This is separated-out for consistency across loggers.
func shouldOutput(loggerLevel int, messageLevel string) bool {
	return levelToInt[messageLevel] >= loggerLevel
}
