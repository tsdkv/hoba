package terminfo

import (
	"encoding/binary"
	"errors"
	"io"

	"github.com/tsdkv/hoba/internal/binio"
)

const blockMetaSize = 39

type blockMeta struct {
	offset         uint64
	docFreqNBits   uint8
	postingsNBits  uint8
	positionsNBits uint8

	ref TermInfo
}

func (bm *blockMeta) Serialize(w io.Writer) error {
	var buf [8]byte

	binary.LittleEndian.PutUint64(buf[:], bm.offset)
	if _, err := binio.WriteBytes(w, buf[:]); err != nil {
		return err
	}

	if _, err := binio.WriteBytes(w, []byte{bm.docFreqNBits}); err != nil {
		return err
	}
	if _, err := binio.WriteBytes(w, []byte{bm.postingsNBits}); err != nil {
		return err
	}
	if _, err := binio.WriteBytes(w, []byte{bm.positionsNBits}); err != nil {
		return err
	}

	binary.LittleEndian.PutUint32(buf[:4], bm.ref.DocFreq)
	if _, err := binio.WriteBytes(w, buf[:4]); err != nil {
		return err
	}
	binary.LittleEndian.PutUint64(buf[:], bm.ref.PostingsOffset)
	if _, err := binio.WriteBytes(w, buf[:]); err != nil {
		return err
	}
	binary.LittleEndian.PutUint32(buf[:4], bm.ref.PostingsLength)
	if _, err := binio.WriteBytes(w, buf[:4]); err != nil {
		return err
	}
	binary.LittleEndian.PutUint64(buf[:], bm.ref.PositionsOffset)
	if _, err := binio.WriteBytes(w, buf[:]); err != nil {
		return err
	}
	binary.LittleEndian.PutUint32(buf[:4], bm.ref.PositionsLength)
	if _, err := binio.WriteBytes(w, buf[:4]); err != nil {
		return err
	}

	return nil
}

func deserializeBlockMeta(data []byte) (*blockMeta, error) {
	if len(data) != blockMetaSize {
		return nil, errors.New("wrong buffer size")
	}

	bm := &blockMeta{}

	bm.offset = binary.LittleEndian.Uint64(data)
	bm.docFreqNBits = data[8]
	bm.postingsNBits = data[9]
	bm.positionsNBits = data[10]

	bm.ref.DocFreq = binary.LittleEndian.Uint32(data[11:])
	bm.ref.PostingsOffset = binary.LittleEndian.Uint64(data[15:])
	bm.ref.PostingsLength = binary.LittleEndian.Uint32(data[23:])
	bm.ref.PositionsOffset = binary.LittleEndian.Uint64(data[27:])
	bm.ref.PositionsLength = binary.LittleEndian.Uint32(data[35:])

	return bm, nil
}
