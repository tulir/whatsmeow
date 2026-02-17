//go:build !js || !wasm

package shared

import "go.mau.fi/whatsmeow"

// registerWASMBridge is a no-op on non-WASM platforms.
func RegisterWASMBridge(cli *whatsmeow.Client) {
	// No-op
}
