package bitpack

import (
	"encoding/binary"
	"fmt"
	"io"
)

func Extract(data []byte, bitOffset uint64, nBits uint8) (uint64, error) {
	if nBits > 56 {
		return 0, fmt.Errorf("invalid bit width %d", nBits)
	}
	if nBits == 0 {
		return 0, nil
	}

	addr := bitOffset / 8
	shift := bitOffset % 8
	required := (shift + uint64(nBits) + 7) / 8
	if addr > uint64(len(data)) || uint64(len(data))-addr < required {
		return 0, io.ErrUnexpectedEOF
	}

	var buf uint64
	if len(data) >= int(addr)+8 {
		buf = binary.LittleEndian.Uint64(data[addr : addr+8])
	} else {
		var window [8]byte
		copy(window[:], data[addr:])
		buf = binary.LittleEndian.Uint64(window[:])
	}

	mask := uint64(1<<uint64(nBits)) - 1
	return (buf >> shift) & mask, nil
}
