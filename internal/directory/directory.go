package directory

import "io"

type ReadFile interface {
	Len() int
	Bytes() []byte // whole file, no copy, read-only
	Close() error
}

type Directory interface {
	Create(name string) (WriteFile, error)
	Open(name string) (ReadFile, error)

	List() ([]string, error)
	ReadAll(name string) ([]byte, error)
	AtomicWrite(name string, data []byte) error

	SyncDir() error
}

type WriteFile interface {
	io.Writer
	Sync() error
	Abort() error
	Commit() error
}
