package store

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"

	"github.com/pierrec/lz4/v4"

	"github.com/tsdkv/hoba/internal/binio"
	"github.com/tsdkv/hoba/schema"
)

const maxBlockSize = 16 * 1024

type CompressorKind uint8

const (
	LZ4 = CompressorKind(iota)
)

// Store file layout:
// [compressed blocks...][24-byte checkpoints...]
// [checkpoint index offset: big-endian uint64][compressor: uint8]
//
// Compressed block layout:
// [uncompressed size: little-endian uint32][LZ4 payload]
type Writer struct {
	dst    io.Writer
	schema *schema.Schema

	docsInBlock  int
	currentBlock *bytes.Buffer
	docPos       []int

	compressor   lz4.Compressor
	compressBuff []byte

	firstDoc        uint32
	flushedBlockPos uint64
	checkpoints     []checkpoint
}

func NewWriter(schema *schema.Schema, dst io.Writer) *Writer {
	buf := make([]byte, 0, maxBlockSize)
	return &Writer{
		dst:          dst,
		schema:       schema,
		currentBlock: bytes.NewBuffer(buf),
	}
}

func (w *Writer) Store(doc schema.Document) error {
	offset := w.currentBlock.Len()

	_, err := doc.Serialize(w.currentBlock, w.schema)
	if err != nil {
		return err
	}

	w.docPos = append(w.docPos, offset)
	w.docsInBlock++

	return w.maybeFlushBlock()
}

// Flushing happens after storing a document, so a block may exceed maxBlockSize.
func (w *Writer) maybeFlushBlock() error {
	if w.currentBlock.Len()+len(w.docPos) > maxBlockSize {
		return w.flushBlock()
	}
	return nil
}

func (w *Writer) flushBlock() error {
	if w.docsInBlock == 0 {
		return nil
	}

	if err := w.appendBlockFooter(); err != nil {
		return err
	}

	block := w.currentBlock.Bytes()
	bound := lz4.CompressBlockBound(len(block))

	if len(w.compressBuff) < bound+4 {
		w.compressBuff = make([]byte, bound+4)
	}
	binary.LittleEndian.PutUint32(w.compressBuff[:4], uint32(len(block)))

	n, err := w.compressor.CompressBlock(block, w.compressBuff[4:])
	if err != nil {
		return err
	}
	if n == 0 {
		return errors.New("lz4: compression failed with bound-sized destination")
	}

	if _, err := binio.WriteBytes(w.dst, w.compressBuff[:n+4]); err != nil {
		return err
	}

	w.checkpoints = append(w.checkpoints, checkpoint{
		FirstDoc:   w.firstDoc,
		NumDocs:    uint32(w.docsInBlock),
		BlockStart: w.flushedBlockPos,
		BlockLen:   uint64(n + 4),
	})
	w.firstDoc += uint32(w.docsInBlock)
	w.flushedBlockPos += uint64(n + 4)

	w.currentBlock.Reset()
	w.docPos = w.docPos[:0]
	w.docsInBlock = 0
	return nil
}

// Uncompressed block layout:
// [documents...][absolute document offsets: little-endian uint32...]
// [document count: little-endian uint32]
func (w *Writer) appendBlockFooter() error {
	var buf [4]byte
	for _, pos := range w.docPos {
		binary.LittleEndian.PutUint32(buf[:], uint32(pos))
		if _, err := binio.WriteBytes(w.currentBlock, buf[:]); err != nil {
			return err
		}
	}

	var footerLen [4]byte
	binary.LittleEndian.PutUint32(footerLen[:], uint32(len(w.docPos)))
	if _, err := binio.WriteBytes(w.currentBlock, footerLen[:]); err != nil {
		return err
	}

	return nil
}

func (w *Writer) Finalize() error {
	if err := w.flushBlock(); err != nil {
		return err
	}

	for _, c := range w.checkpoints {
		if err := c.Serialize(w.dst); err != nil {
			return err
		}
	}

	var footerOffset [8]byte
	binary.BigEndian.PutUint64(footerOffset[:], w.flushedBlockPos)
	if _, err := binio.WriteBytes(w.dst, footerOffset[:]); err != nil {
		return err
	}

	if _, err := binio.WriteBytes(w.dst, []byte{byte(LZ4)}); err != nil {
		return err
	}

	return nil
}
