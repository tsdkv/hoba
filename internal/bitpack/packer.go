package bitpack

import (
	"encoding/binary"
	"io"

	"github.com/tsdkv/hoba/internal/binio"
)

type Packer struct {
	w io.Writer

	buf  uint64
	used uint64

	bbuf [8]byte
}

func NewPacker(w io.Writer) *Packer {
	return &Packer{
		w:    w,
		bbuf: [8]byte{},
	}
}

func (p *Packer) Write(val uint64, numBits int) error {
	uNumbBits := uint64(numBits)
	if p.used+uNumbBits <= 64 {
		val <<= p.used
		p.buf |= val
		p.used += uNumbBits

		if p.used == 64 {
			if err := p.emit(8); err != nil {
				return err
			}
			p.buf = 0
			p.used = 0
		}
		return nil
	}

	shift := 64 - p.used
	p.buf |= val << p.used
	if err := p.emit(8); err != nil {
		return err
	}

	p.buf = val >> shift
	p.used = uNumbBits - shift

	return nil
}

func (p *Packer) Flush() error {
	if p.used == 0 {
		return nil
	}
	if err := p.emit(int((p.used + 7) / 8)); err != nil {
		return err
	}
	p.buf = 0
	p.used = 0
	return nil
}

func (p *Packer) emit(numBytes int) error {
	binary.LittleEndian.PutUint64(p.bbuf[:], p.buf)

	if _, err := binio.WriteBytes(p.w, p.bbuf[:numBytes]); err != nil {
		return err
	}
	return nil
}
