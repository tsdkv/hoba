package directory

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
)

type CountingWriter struct {
	io.Writer
	offset uint64
}

func (cw *CountingWriter) Write(p []byte) (n int, err error) {
	n, err = cw.Writer.Write(p)
	if err != nil {
		return
	}

	cw.offset += uint64(n)
	return
}

func (cw *CountingWriter) WrittenBytes() uint64 {
	return cw.offset
}

type FileAddr struct {
	FieldID uint32
	Offset  uint64
}

// CompositeWriter packs field payloads into one file:
//
//	[field 0 payload][field 1 payload]...
//	[footer: uvarint field count, (uvarint field ID, uvarint offset delta)...]
//	[footer length: little-endian uint32]
type CompositeWriter struct {
	file *CountingWriter

	offsets []FileAddr
}

func NewCompositeWriter(dst io.Writer) *CompositeWriter {
	return &CompositeWriter{
		file: &CountingWriter{
			Writer: dst,
		},
	}
}

func (cw *CompositeWriter) ForField(field uint32) io.Writer {
	cw.offsets = append(cw.offsets, FileAddr{
		FieldID: field,
		Offset:  cw.file.WrittenBytes(),
	})

	return cw.file
}

func (cw *CompositeWriter) Finalize() error {
	var footer bytes.Buffer

	if err := writeUvarint(&footer, uint64(len(cw.offsets))); err != nil {
		return err
	}

	previousOffset := uint64(0)
	for _, o := range cw.offsets {
		if err := writeUvarint(&footer, uint64(o.FieldID)); err != nil {
			return err
		}

		delta := o.Offset - previousOffset
		if err := writeUvarint(&footer, uint64(delta)); err != nil {
			return err
		}
		previousOffset = o.Offset
	}

	if _, err := cw.file.Write(footer.Bytes()); err != nil {
		return err
	}

	var footerLen [4]byte
	binary.LittleEndian.PutUint32(footerLen[:], uint32(footer.Len()))
	_, err := cw.file.Write(footerLen[:])
	return err
}

func writeUvarint(w io.Writer, value uint64) error {
	var buf [binary.MaxVarintLen64]byte
	n := binary.PutUvarint(buf[:], value)
	_, err := w.Write(buf[:n])
	return err
}

type fieldRange struct {
	start int
	end   int
}

type CompositeReader struct {
	data    []byte
	offsets map[uint32]fieldRange
}

func OpenCompositeFile(fh ReadFile) (*CompositeReader, error) {
	data := fh.Bytes()
	if len(data) < 4 {
		return nil, io.ErrUnexpectedEOF
	}

	footerLen := binary.LittleEndian.Uint32(data[len(data)-4:])
	if uint64(footerLen) > uint64(len(data)-4) {
		return nil, errors.New("invalid footer length")
	}

	footerStart := len(data) - 4 - int(footerLen)
	footer := bytes.NewReader(data[footerStart : len(data)-4])

	numFields, err := binary.ReadUvarint(footer)
	if err != nil {
		return nil, err
	}

	fields := make([]uint32, 0)
	starts := make([]int, 0)
	seen := make(map[uint32]struct{})
	var offset uint64
	for range numFields {
		field, err := binary.ReadUvarint(footer)
		if err != nil {
			return nil, err
		}
		if uint64(uint32(field)) != field {
			return nil, errors.New("field ID overflows uint32")
		}

		delta, err := binary.ReadUvarint(footer)
		if err != nil {
			return nil, err
		}
		if offset > uint64(footerStart) || delta > uint64(footerStart)-offset {
			return nil, errors.New("invalid field offset")
		}
		offset += delta

		fieldID := uint32(field)
		if _, ok := seen[fieldID]; ok {
			return nil, errors.New("duplicate field")
		}

		seen[fieldID] = struct{}{}
		fields = append(fields, fieldID)
		starts = append(starts, int(offset))
	}

	if footer.Len() != 0 {
		return nil, errors.New("trailing bytes in footer")
	}

	cr := &CompositeReader{
		data:    data,
		offsets: make(map[uint32]fieldRange, len(fields)),
	}

	for i, field := range fields {
		end := footerStart
		if i+1 < len(starts) {
			end = starts[i+1]
		}

		cr.offsets[field] = fieldRange{start: starts[i], end: end}
	}

	return cr, nil
}

func (cr *CompositeReader) Field(field uint32) ([]byte, error) {
	offset, ok := cr.offsets[field]
	if !ok {
		return nil, errors.New("unknown field")
	}

	return cr.data[offset.start:offset.end], nil
}
