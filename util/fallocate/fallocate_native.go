//go:build !wasm

package fallocate

import (
	"os"

	goFallocate "go.mau.fi/util/fallocate"
	"go.mau.fi/whatsmeow/iface"
)

func Fallocate(w iface.File, size int64) error {
	file, ok := w.(*os.File)

	if !ok {
		return nil
	}

	return goFallocate.Fallocate(file, int(size))
}
