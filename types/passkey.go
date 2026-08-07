// Copyright (c) 2026 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package types

import (
	"go.mau.fi/util/jsonbytes"
)

type WebAuthnPublicKey struct {
	Challenge        jsonbytes.UnpaddedURLBytes `json:"challenge"`
	Timeout          int                        `json:"timeout"`
	RelyingPartID    string                     `json:"rpId"`
	AllowCredentials []AllowedCredential        `json:"allowCredentials"`
	UserVerification string                     `json:"userVerification"`
	Extensions       map[string]any             `json:"extensions"`
}

type AllowedCredential struct {
	ID         jsonbytes.UnpaddedURLBytes `json:"id"`
	Type       string                     `json:"type"`
	Transports []string                   `json:"transports"`
}

type WebAuthnResponse struct {
	ID       string                     `json:"id"`
	RawID    jsonbytes.UnpaddedURLBytes `json:"rawId"`
	Type     string                     `json:"type"`
	Response WebAuthnResponseData       `json:"response"`
}

type WebAuthnResponseData struct {
	ClientDataJSON    jsonbytes.UnpaddedURLBytes  `json:"clientDataJSON"`
	AuthenticatorData jsonbytes.UnpaddedURLBytes  `json:"authenticatorData"`
	Signature         jsonbytes.UnpaddedURLBytes  `json:"signature"`
	UserHandle        *jsonbytes.UnpaddedURLBytes `json:"userHandle"`
}
