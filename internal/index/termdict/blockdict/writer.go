package blockdict

import (
	"bytes"
	"encoding/binary"
	"io"

	"github.com/tsdkv/hoba/internal/binio"
	"github.com/tsdkv/hoba/internal/directory"
)

type Writer struct {
	w *directory.CountingWriter

	numTerms uint32

	firsts bytes.Buffer
	index  []indexEntry

	savedFirst bool
	cur        *blockWriter
}

func NewWriter(w io.Writer) *Writer {
	return &Writer{
		w:   &directory.CountingWriter{Writer: w},
		cur: newBlock(),
	}
}

func (w *Writer) Insert(term []byte) error {
	if !w.savedFirst {
		w.index = append(w.index, indexEntry{
			blockOffset: uint64(w.w.WrittenBytes()),
			termOffset:  uint32(w.firsts.Len()),
			termLen:     uint32(len(term)),
		})

		if _, err := binio.WriteBytes(&w.firsts, term); err != nil {
			return err
		}

		w.savedFirst = true
		return nil
	}

	if err := w.cur.Add(term); err != nil {
		return err
	}

	if w.cur.Terms() == maxBlockTerms {
		if err := w.cur.Flush(w.w); err != nil {
			return err
		}

		w.savedFirst = false
		w.cur = newBlock()
	}

	return nil
}

func (w *Writer) Finish() error {
	if err := w.cur.Flush(w.w); err != nil {
		return err
	}
	blocksFinish := w.w.WrittenBytes()

	for _, ie := range w.index {
		if err := ie.Serialize(w.w); err != nil {
			return err
		}
	}

	if _, err := binio.WriteBytes(w.w, w.firsts.Bytes()); err != nil {
		return err
	}

	var footer [12]byte
	binary.LittleEndian.PutUint32(footer[:4], uint32(len(w.index)))
	binary.LittleEndian.PutUint64(footer[4:12], blocksFinish)
	if _, err := binio.WriteBytes(w.w, footer[:]); err != nil {
		return err
	}

	return nil
}
