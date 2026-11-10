package terminfo

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math/bits"

	"github.com/tsdkv/hoba/internal/binio"
	"github.com/tsdkv/hoba/internal/bitpack"
)

const (
	blockLen = 256
	maxDelta = uint64(1)<<56 - 1
)

type TermInfo struct {
	DocFreq         uint32
	PostingsOffset  uint64
	PostingsLength  uint32
	PositionsOffset uint64
	PositionsLength uint32
}

// Term info file layout:
// [metadata length: uvarint][term count: uvarint]
// [39-byte block metadata...][bit-packed blocks...]
//
// Each block contains up to 256 terms. Its metadata stores the first TermInfo
// verbatim, the packed byte offset, and the bit widths shared by the block.
type Writer struct {
	metas    bytes.Buffer
	packed   bytes.Buffer
	block    []TermInfo
	numTerms uint64
	finished bool
}

func NewWriter() *Writer {
	return &Writer{}
}

// Write requires PostingsOffset and PositionsOffset to be non-decreasing
func (w *Writer) Write(info TermInfo) error {
	w.block = append(w.block, info)
	w.numTerms++

	if len(w.block) == 256 {
		return w.flushBlock()
	}
	return nil
}

// Packed block layout:
// [DocFreq][postings delta][positions delta]...
// [terminal postings end delta][terminal positions end delta]
//
// Entry offsets are relative to the block's first TermInfo. Terminal deltas
// allow the reader to recover the final entry's lengths.
func (w *Writer) flushBlock() error {
	if len(w.block) == 0 {
		return nil
	}

	maxDocFreq := uint32(0)
	for _, ti := range w.block[1:] {
		maxDocFreq = max(maxDocFreq, ti.DocFreq)
	}

	ref := w.block[0]
	last := w.block[len(w.block)-1]

	maxPostingsDelta := last.PostingsOffset + uint64(last.PostingsLength) - ref.PostingsOffset
	if maxPostingsDelta > maxDelta {
		return fmt.Errorf("postings delta %d exceeds 56-bit limit", maxPostingsDelta)
	}

	maxPositionsDelta := last.PositionsOffset + uint64(last.PositionsLength) - ref.PositionsOffset
	if maxPositionsDelta > maxDelta {
		return fmt.Errorf("positions delta %d exceeds 56-bit limit", maxPositionsDelta)
	}

	meta := blockMeta{
		offset:         uint64(w.packed.Len()),
		docFreqNBits:   uint8(bits.Len32(maxDocFreq)),
		postingsNBits:  uint8(bits.Len64(maxPostingsDelta)),
		positionsNBits: uint8(bits.Len64(maxPositionsDelta)),
		ref:            ref,
	}
	if err := meta.Serialize(&w.metas); err != nil {
		return err
	}

	p := bitpack.NewPacker(&w.packed)
	for i := 1; i < len(w.block); i++ {
		ti := w.block[i]

		if err := p.Write(uint64(ti.DocFreq), int(meta.docFreqNBits)); err != nil {
			return err
		}
		if err := p.Write(ti.PostingsOffset-ref.PostingsOffset, int(meta.postingsNBits)); err != nil {
			return err
		}
		if err := p.Write(ti.PositionsOffset-ref.PositionsOffset, int(meta.positionsNBits)); err != nil {
			return err
		}
	}

	if err := p.Write(maxPostingsDelta, int(meta.postingsNBits)); err != nil {
		return err
	}
	if err := p.Write(maxPositionsDelta, int(meta.positionsNBits)); err != nil {
		return err
	}

	if err := p.Flush(); err != nil {
		return err
	}

	w.block = w.block[:0]
	return nil
}

func (w *Writer) Finish(dst io.Writer) error {
	if err := w.flushBlock(); err != nil {
		return err
	}

	var buf [binary.MaxVarintLen64]byte
	if _, err := binio.WriteUvarint(dst, uint64(w.metas.Len()), buf[:]); err != nil {
		return err
	}
	if _, err := binio.WriteUvarint(dst, w.numTerms, buf[:]); err != nil {
		return err
	}

	if _, err := binio.WriteBytes(dst, w.metas.Bytes()); err != nil {
		return err
	}
	if _, err := binio.WriteBytes(dst, w.packed.Bytes()); err != nil {
		return err
	}

	return nil
}
