package index

import "encoding/binary"

// EncodeInt64 encodes v as an 8-byte string whose lexicographic order
// matches v's numeric order, so a sorted term dictionary orders integers
// numerically.
func EncodeInt64(v int64) string {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], uint64(v)^(1<<63))
	return string(b[:])
}
