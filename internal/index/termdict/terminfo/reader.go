package terminfo

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"github.com/tsdkv/hoba/internal/bitpack"
)

var ErrInvalidFormat = errors.New("invalid format")

type Reader struct {
	numTerms uint64
	metas    []byte
	packed   []byte
}

func NewReader(data []byte) (*Reader, error) {
	metasLen, n := binary.Uvarint(data)
	if n == 0 {
		return nil, fmt.Errorf("%w: truncated metadata length", io.ErrUnexpectedEOF)
	}
	if n < 0 {
		return nil, fmt.Errorf("%w: invalid metadata length", ErrInvalidFormat)
	}
	headerLen := n

	numTerms, n := binary.Uvarint(data[headerLen:])
	if n == 0 {
		return nil, fmt.Errorf("%w: truncated term count", io.ErrUnexpectedEOF)
	}
	if n < 0 {
		return nil, fmt.Errorf("%w: invalid term count", ErrInvalidFormat)
	}
	headerLen += n

	remaining := len(data) - headerLen
	if metasLen > uint64(remaining) {
		return nil, fmt.Errorf(
			"%w: metadata length %d exceeds remaining input %d",
			io.ErrUnexpectedEOF, metasLen, remaining,
		)
	}

	numBlocks := numTerms / blockLen
	if numTerms%blockLen != 0 {
		numBlocks++
	}
	expectedMetasLen := numBlocks * blockMetaSize
	if metasLen != expectedMetasLen {
		return nil, fmt.Errorf(
			"%w: metadata length %d, want %d for %d terms",
			ErrInvalidFormat, metasLen, expectedMetasLen, numTerms,
		)
	}

	metasEnd := headerLen + int(metasLen)
	return &Reader{
		numTerms: numTerms,
		metas:    data[headerLen:metasEnd],
		packed:   data[metasEnd:],
	}, nil
}

func (r *Reader) NumTerms() uint64 {
	return r.numTerms
}

func (r *Reader) Get(ord uint64) (TermInfo, error) {
	if ord >= r.numTerms {
		return TermInfo{}, fmt.Errorf(
			"ordinal %d out of range (num terms %d)",
			ord, r.numTerms,
		)
	}

	idx := ord / blockLen
	inner := ord % blockLen

	metaStart := idx * blockMetaSize
	metaEnd := metaStart + blockMetaSize

	meta, err := deserializeBlockMeta(r.metas[metaStart:metaEnd])
	if err != nil {
		return TermInfo{}, err
	}

	if inner == 0 {
		return meta.ref, nil
	}

	packedSize := uint64(meta.docFreqNBits) + uint64(meta.positionsNBits) + uint64(meta.postingsNBits)

	offset := meta.offset*8 + packedSize*(inner-1)
	docFreq, err := bitpack.Extract(r.packed, offset, meta.docFreqNBits)
	if err != nil {
		return TermInfo{}, err
	}

	offset += uint64(meta.docFreqNBits)
	postings, err := bitpack.Extract(r.packed, offset, meta.postingsNBits)
	if err != nil {
		return TermInfo{}, err
	}
	offset += uint64(meta.postingsNBits)

	positions, err := bitpack.Extract(r.packed, offset, meta.positionsNBits)
	if err != nil {
		return TermInfo{}, err
	}
	offset += uint64(meta.positionsNBits)

	lastBlockIdx := r.numTerms / blockLen
	curBlockLen := blockLen
	if idx == lastBlockIdx {
		curBlockLen = int(r.numTerms) % blockLen
		if curBlockLen == 0 {
			curBlockLen = 256
		}
	}
	if inner < uint64(curBlockLen)-1 {
		offset += uint64(meta.docFreqNBits)
	}

	nextPost, err := bitpack.Extract(r.packed, offset, meta.postingsNBits)
	if err != nil {
		return TermInfo{}, err
	}
	offset += uint64(meta.postingsNBits)

	nextPos, err := bitpack.Extract(r.packed, offset, meta.positionsNBits)
	if err != nil {
		return TermInfo{}, err
	}

	return TermInfo{
		DocFreq:         uint32(docFreq),
		PostingsOffset:  postings + meta.ref.PostingsOffset,
		PositionsOffset: positions + meta.ref.PositionsOffset,

		PostingsLength:  uint32(nextPost - postings),
		PositionsLength: uint32(nextPos - positions),
	}, nil
}
