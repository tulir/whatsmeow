// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package appstate

import "errors"

// Errors that this package can return.
var (
	ErrMissingPreviousSetValueOperation = errors.New("missing value MAC of previous SET operation")
	ErrMismatchingLTHash                = errors.New("mismatching LTHash")
	ErrMismatchingPatchMAC              = errors.New("mismatching patch MAC")
	ErrMismatchingContentMAC            = errors.New("mismatching content MAC")
	ErrMismatchingIndexMAC              = errors.New("mismatching index MAC")
	ErrKeyNotFound                      = errors.New("didn't find app state key")
)
