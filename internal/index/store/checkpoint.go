package store

import (
	"encoding/binary"
	"errors"
	"io"

	"github.com/tsdkv/hoba/internal/binio"
)

type checkpoint struct {
	FirstDoc   uint32
	NumDocs    uint32
	BlockStart uint64
	BlockLen   uint64
}

func (c *checkpoint) Serialize(w io.Writer) error {
	var buf [8]byte

	binary.LittleEndian.PutUint32(buf[:4], c.FirstDoc)
	if _, err := binio.WriteBytes(w, buf[:4]); err != nil {
		return err
	}

	binary.LittleEndian.PutUint32(buf[:4], c.NumDocs)
	if _, err := binio.WriteBytes(w, buf[:4]); err != nil {
		return err
	}

	binary.LittleEndian.PutUint64(buf[:], c.BlockStart)
	if _, err := binio.WriteBytes(w, buf[:]); err != nil {
		return err
	}

	binary.LittleEndian.PutUint64(buf[:], c.BlockLen)
	if _, err := binio.WriteBytes(w, buf[:]); err != nil {
		return err
	}

	return nil
}

func deserializeCheckpoint(data []byte) (*checkpoint, error) {
	if len(data) != 24 {
		return nil, errors.New("write buffer size")
	}

	c := &checkpoint{}
	c.FirstDoc = binary.LittleEndian.Uint32(data)
	c.NumDocs = binary.LittleEndian.Uint32(data[4:])
	c.BlockStart = binary.LittleEndian.Uint64(data[8:])
	c.BlockLen = binary.LittleEndian.Uint64(data[16:])

	return c, nil
}
