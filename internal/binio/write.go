package binio

import (
	"encoding/binary"
	"io"
)

func WriteUvarint(w io.Writer, value uint64, buf []byte) (int, error) {
	n := binary.PutUvarint(buf, value)
	return WriteBytes(w, buf[:n])
}

func WriteVarint(w io.Writer, value int64, buf []byte) (int, error) {
	n := binary.PutVarint(buf, value)
	return WriteBytes(w, buf[:n])
}

func WriteBytes(w io.Writer, data []byte) (int, error) {
	n, err := w.Write(data)
	if err != nil {
		return n, err
	}
	if n != len(data) {
		return n, io.ErrShortWrite
	}
	return n, nil
}
