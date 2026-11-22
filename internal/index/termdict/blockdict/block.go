package blockdict

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"slices"

	"github.com/tsdkv/hoba/internal/binio"
)

const maxBlockTerms = 32

var (
	errBlockFull    = errors.New("block is full")
	errInvalidBlock = errors.New("invalid block")
)

type blockWriter struct {
	numTerms  int
	lastTerm  []byte
	buf       bytes.Buffer
	firstTerm []byte

	varIBuf [binary.MaxVarintLen64]byte
}

func newBlock() *blockWriter {
	return &blockWriter{}
}

func (b *blockWriter) Add(term []byte) error {
	if b.numTerms >= maxBlockTerms {
		return errBlockFull
	}
	if b.numTerms == 0 {
		b.firstTerm = term
		b.lastTerm = term
		b.numTerms++
		return nil
	}

	preffLen := 0
	for preffLen < len(b.lastTerm) && preffLen < len(term) && b.lastTerm[preffLen] == term[preffLen] {
		preffLen++
	}
	suff := term[preffLen:]

	if _, err := binio.WriteUvarint(&b.buf, uint64(preffLen), b.varIBuf[:]); err != nil {
		return err
	}
	if _, err := binio.WriteUvarint(&b.buf, uint64(len(suff)), b.varIBuf[:]); err != nil {
		return err
	}
	if _, err := binio.WriteBytes(&b.buf, suff); err != nil {
		return err
	}

	b.numTerms++
	b.lastTerm = term

	return nil
}

func (b *blockWriter) Flush(w io.Writer) error {
	if _, err := binio.WriteUvarint(w, uint64(b.numTerms), b.varIBuf[:]); err != nil {
		return err
	}
	if _, err := binio.WriteUvarint(w, uint64(len(b.firstTerm)), b.varIBuf[:]); err != nil {
		return err
	}
	if _, err := binio.WriteBytes(w, b.firstTerm); err != nil {
		return err
	}
	if _, err := binio.WriteBytes(w, b.buf.Bytes()); err != nil {
		return err
	}
	return nil
}

func (b *blockWriter) Terms() int {
	return b.numTerms
}

type blockReader struct {
	data []byte

	offset int

	firstBytes []byte
	numTerms   uint64
}

func newBlockReader(data []byte) (*blockReader, error) {
	numTerms, n := binary.Uvarint(data)
	if n == 0 {
		return nil, fmt.Errorf("%w: term count", io.ErrUnexpectedEOF)
	}
	if n < 0 {
		return nil, fmt.Errorf("%w: term count", errInvalidBlock)
	}
	offset := n

	firstLen, n := binary.Uvarint(data[offset:])
	if n == 0 {
		return nil, fmt.Errorf("%w: first term length", io.ErrUnexpectedEOF)
	}
	if n < 0 {
		return nil, fmt.Errorf("%w: first term length", errInvalidBlock)
	}
	offset += n
	if firstLen > uint64(len(data)-offset) {
		return nil, fmt.Errorf("%w: first term", io.ErrUnexpectedEOF)
	}

	firstBytes := data[offset : offset+int(firstLen)]
	offset += int(firstLen)

	return &blockReader{
		data: data,

		offset: offset,

		numTerms:   numTerms,
		firstBytes: firstBytes,
	}, nil
}

func (be *blockReader) Get(term []byte) (uint32, bool, error) {
	if be.numTerms == 0 {
		return 0, false, nil
	}
	if slices.Equal(be.firstBytes, term) {
		return 0, true, nil
	}

	var buf bytes.Buffer
	_, _ = buf.Write(be.firstBytes)

	offset := be.offset
	for i := uint64(1); i < be.numTerms; i++ {
		prefLen, n := binary.Uvarint(be.data[offset:])
		if n == 0 {
			return 0, false, fmt.Errorf("%w: prefix length", io.ErrUnexpectedEOF)
		}
		if n < 0 {
			return 0, false, fmt.Errorf("%w: prefix length", errInvalidBlock)
		}
		offset += n

		if prefLen > uint64(buf.Len()) {
			return 0, false, fmt.Errorf("%w: prefix length exceeds previous term", errInvalidBlock)
		}
		buf.Truncate(int(prefLen))

		sufLen, n := binary.Uvarint(be.data[offset:])
		if n == 0 {
			return 0, false, fmt.Errorf("%w: suffix length", io.ErrUnexpectedEOF)
		}
		if n < 0 {
			return 0, false, fmt.Errorf("%w: suffix length", errInvalidBlock)
		}
		offset += n
		if sufLen > uint64(len(be.data)-offset) {
			return 0, false, fmt.Errorf("%w: suffix", io.ErrUnexpectedEOF)
		}

		_, _ = buf.Write(be.data[offset : offset+int(sufLen)])
		offset += int(sufLen)

		if slices.Equal(buf.Bytes(), term) {
			return uint32(i), true, nil
		}
	}

	return 0, false, nil
}
