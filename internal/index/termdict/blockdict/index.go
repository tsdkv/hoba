package blockdict

import (
	"encoding/binary"
	"errors"
	"io"

	"github.com/tsdkv/hoba/internal/binio"
)

const indexEntrySize = 16

type indexEntry struct {
	blockOffset uint64
	termOffset  uint32
	termLen     uint32
}

func (ie *indexEntry) Serialize(w io.Writer) error {
	var data [indexEntrySize]byte
	binary.LittleEndian.PutUint64(data[0:8], ie.blockOffset)
	binary.LittleEndian.PutUint32(data[8:12], ie.termOffset)
	binary.LittleEndian.PutUint32(data[12:16], ie.termLen)
	_, err := binio.WriteBytes(w, data[:])
	return err
}

func deserializeIndexEntry(data []byte) (*indexEntry, error) {
	if len(data) != indexEntrySize {
		return nil, errors.New("wrong buffer size")
	}
	return &indexEntry{
		blockOffset: binary.LittleEndian.Uint64(data[0:8]),
		termOffset:  binary.LittleEndian.Uint32(data[8:12]),
		termLen:     binary.LittleEndian.Uint32(data[12:16]),
	}, nil
}
