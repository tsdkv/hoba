package index

import "encoding/binary"

func EncodeInt64(v int64) string {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], uint64(v)^(1<<63))
	return string(b[:])
}
