package directory

import (
	"errors"
	"os"
)

type pendingFile struct {
	f         *os.File
	tmpPath   string
	path      string
	published bool
}

func (w *pendingFile) Write(p []byte) (int, error) {
	return w.f.Write(p)
}

func (w *pendingFile) Sync() error {
	return w.f.Sync()
}

func (w *pendingFile) Abort() error {
	if w.published {
		return removeTemp(w.tmpPath)
	}

	closeErr := w.f.Close()
	if errors.Is(closeErr, os.ErrClosed) {
		closeErr = nil
	}

	return errors.Join(closeErr, removeTemp(w.tmpPath))
}

func (w *pendingFile) Commit() error {
	if w.published {
		return removeTemp(w.tmpPath)
	}

	if err := w.f.Close(); err != nil {
		return errors.Join(err, removeTemp(w.tmpPath))
	}

	if err := os.Link(w.tmpPath, w.path); err != nil {
		return errors.Join(err, removeTemp(w.tmpPath))
	}

	w.published = true

	return removeTemp(w.tmpPath)
}
