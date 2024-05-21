// Copyright (c) 2024 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package waLog

import (
	"fmt"

	"github.com/rs/zerolog"
)

type zeroLogger struct {
	mod string
	zerolog.Logger
}

// Zerolog wraps a [zerolog.Logger] to implement the [Logger] interface.
//
// Subloggers will be created by setting the `sublogger` field in the log context.
func Zerolog(log zerolog.Logger) Logger {
	return &zeroLogger{Logger: log}
}

func (z *zeroLogger) Warnf(msg string, args ...any)  { z.Warn().Msgf(msg, args...) }
func (z *zeroLogger) Errorf(msg string, args ...any) { z.Error().Msgf(msg, args...) }
func (z *zeroLogger) Infof(msg string, args ...any)  { z.Info().Msgf(msg, args...) }
func (z *zeroLogger) Debugf(msg string, args ...any) { z.Debug().Msgf(msg, args...) }
func (z *zeroLogger) Sub(module string) Logger {
	if z.mod != "" {
		module = fmt.Sprintf("%s/%s", z.mod, module)
	}
	return &zeroLogger{mod: module, Logger: z.Logger.With().Str("sublogger", module).Logger()}
}

var _ Logger = &zeroLogger{}
