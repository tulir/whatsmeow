package iface

import (
	"io"
	"os"
)

type File interface {
	io.Reader
	io.Writer
	io.Seeker
	io.ReaderAt
	io.WriterAt
	Truncate(size int64) error
	Stat() (os.FileInfo, error)
}
