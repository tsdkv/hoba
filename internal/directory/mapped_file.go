package directory

import (
	"errors"
	"os"

	"github.com/edsrzf/mmap-go"
)

type mappedFile struct {
	f     *os.File
	bytes mmap.MMap
}

func (mfh *mappedFile) Len() int {
	return len(mfh.bytes)
}

func (mfh *mappedFile) Bytes() []byte {
	return mfh.bytes
}

func (mfh *mappedFile) Close() error {
	var unmmapErr error

	if len(mfh.bytes) > 0 {
		unmmapErr = mfh.bytes.Unmap()
	}
	closeErr := mfh.f.Close()

	return errors.Join(unmmapErr, closeErr)
}
