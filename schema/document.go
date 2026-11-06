package schema

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
)

type Kind uint8

const (
	Text Kind = iota
	Int64
)

type Value struct {
	Kind Kind
	Str  string // set when Kind == Text
	Num  int64  // set when Kind == Int64
}

type FieldValue struct {
	Field int
	Value Value
}

type Document struct {
	Fields []FieldValue
}

func (d *Document) Text(field int, s string) {
	d.Fields = append(d.Fields, FieldValue{Field: field, Value: Value{Kind: Text, Str: s}})
}

func (d *Document) Int64(field int, n int64) {
	d.Fields = append(d.Fields, FieldValue{Field: field, Value: Value{Kind: Int64, Num: n}})
}

func (d *Document) Serialize(w io.Writer, schema *Schema) (int, error) {
	fields := 0
	for _, f := range d.Fields {
		schField, ok := schema.FieldByID(f.Field)
		if !ok {
			return 0, fmt.Errorf("unknown field ID: %d", f.Field)
		}
		if !schField.Stored {
			continue
		}
		switch f.Value.Kind {
		case Text, Int64:
		default:
			return 0, fmt.Errorf("field %d: unknown value kind %d", f.Field, f.Value.Kind)
		}
		fields++
	}

	written, err := writeUvarint(w, uint64(fields))
	if err != nil {
		return written, fmt.Errorf("write document field count: %w", err)
	}

	for _, f := range d.Fields {
		schField, _ := schema.FieldByID(f.Field)
		if !schField.Stored {
			continue
		}

		n, err := writeUvarint(w, uint64(f.Field))
		written += n
		if err != nil {
			return written, fmt.Errorf("write field %d ID: %w", f.Field, err)
		}

		n, err = writeBytes(w, []byte{byte(f.Value.Kind)})
		written += n
		if err != nil {
			return written, fmt.Errorf("write field %d kind: %w", f.Field, err)
		}

		switch f.Value.Kind {
		case Text:
			n, err := writeUvarint(w, uint64(len(f.Value.Str)))
			written += n
			if err != nil {
				return written, fmt.Errorf("write field %d text length: %w", f.Field, err)
			}

			n, err = writeBytes(w, []byte(f.Value.Str))
			written += n
			if err != nil {
				return written, fmt.Errorf("write field %d text: %w", f.Field, err)
			}
		case Int64:
			var buf [binary.MaxVarintLen64]byte
			n := binary.PutVarint(buf[:], f.Value.Num)
			n, err = writeBytes(w, buf[:n])
			written += n
			if err != nil {
				return written, fmt.Errorf("write field %d integer: %w", f.Field, err)
			}
		}
	}

	return written, nil
}

func writeUvarint(w io.Writer, value uint64) (int, error) {
	var buf [binary.MaxVarintLen64]byte
	n := binary.PutUvarint(buf[:], value)
	return writeBytes(w, buf[:n])
}

func writeBytes(w io.Writer, data []byte) (int, error) {
	n, err := w.Write(data)
	if err != nil {
		return n, err
	}
	if n != len(data) {
		return n, io.ErrShortWrite
	}
	return n, nil
}

func DeserializeDocument(data []byte) (*Document, error) {
	r := bytes.NewReader(data)
	fieldCount, err := binary.ReadUvarint(r)
	if err != nil {
		return nil, fmt.Errorf("read document field count: %w", err)
	}

	doc := &Document{
		Fields: make([]FieldValue, 0, fieldCount),
	}

	for i := range fieldCount {
		fieldID, err := binary.ReadUvarint(r)
		if err != nil {
			return nil, fmt.Errorf("read field %d ID: %w", i, err)
		}

		kind, err := r.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("read field %d kind: %w", fieldID, err)
		}
		if kind != byte(Text) && kind != byte(Int64) {
			return nil, fmt.Errorf("field %d: unknown value kind %d", fieldID, kind)
		}

		fv := FieldValue{
			Field: int(fieldID),
			Value: Value{
				Kind: Kind(kind),
			},
		}

		switch fv.Value.Kind {
		case Text:
			size, err := binary.ReadUvarint(r)
			if err != nil {
				return nil, fmt.Errorf("read field %d text length: %w", fieldID, err)
			}
			if size > uint64(r.Len()) {
				return nil, fmt.Errorf("read field %d text: %w", fieldID, io.ErrUnexpectedEOF)
			}

			start := len(data) - r.Len()
			fv.Value.Str = string(data[start : start+int(size)])
			_, err = r.Seek(int64(size), io.SeekCurrent)
			if err != nil {
				return nil, fmt.Errorf("read field %d text: %w", fieldID, err)
			}
		case Int64:
			v, err := binary.ReadVarint(r)
			if err != nil {
				return nil, fmt.Errorf("read field %d integer: %w", fieldID, err)
			}
			fv.Value.Num = v
		}

		doc.Fields = append(doc.Fields, fv)
	}

	if r.Len() != 0 {
		return nil, fmt.Errorf("unexpected trailing data of length %d", r.Len())
	}
	return doc, nil
}
