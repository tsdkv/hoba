package bitpack

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func TestExtractRoundTrip(t *testing.T) {
	type item struct {
		value  uint64
		offset uint64
		nBits  uint8
	}

	rapid.Check(t, func(rt *rapid.T) {
		numValues := rapid.SampledFrom([]int{0, 1, 2, 3, 7, 8, 16, 64, 257}).Draw(rt, "numValues")
		items := make([]item, 0, numValues)
		var encoded bytes.Buffer
		packer := NewPacker(&encoded)
		var bitOffset uint64

		for i := 0; i < numValues; i++ {
			var nBits uint8
			switch rapid.SampledFrom([]int{0, 1, 2, 3}).Draw(rt, "widthRegime") {
			case 0:
				nBits = rapid.SampledFrom([]uint8{0, 1, 2, 7, 8, 9, 56, 57, 63, 64}).Draw(rt, "boundaryWidth")
			case 1:
				nBits = uint8(rapid.IntRange(0, 16).Draw(rt, "smallWidth"))
			case 2:
				nBits = uint8(rapid.IntRange(17, 56).Draw(rt, "mediumWidth"))
			case 3:
				nBits = uint8(rapid.IntRange(57, 64).Draw(rt, "wideWidth"))
			}

			var value uint64
			if nBits > 0 {
				maxValue := ^uint64(0)
				if nBits < 64 {
					maxValue = (uint64(1) << nBits) - 1
				}
				switch rapid.SampledFrom([]int{0, 1, 2}).Draw(rt, "valueRegime") {
				case 1:
					value = maxValue
				case 2:
					if nBits == 64 {
						value = rapid.Uint64().Draw(rt, "value64")
					} else {
						value = rapid.Uint64Range(0, maxValue).Draw(rt, "value")
					}
				}
			}
			require.NoError(rt, packer.Write(value, int(nBits)))

			items = append(items, item{value: value, offset: bitOffset, nBits: nBits})
			bitOffset += uint64(nBits)
		}
		require.NoError(rt, packer.Flush())

		for i, item := range items {
			got, err := Extract(encoded.Bytes(), item.offset, item.nBits)
			require.NoError(rt, err)
			assert.Equal(rt, item.value, got, "value %d at bit offset %d with width %d", i, item.offset, item.nBits)
		}
	})
}

var benchmarkExtractResult uint64

func BenchmarkExtract(b *testing.B) {
	for _, nBits := range []uint8{8, 32, 59, 64} {
		b.Run(fmt.Sprintf("bits=%d", nBits), func(b *testing.B) {
			const numValues = 10000
			values := make([]uint64, numValues)
			mask := ^uint64(0)
			if nBits < 64 {
				mask = (uint64(1) << nBits) - 1
			}

			var encoded bytes.Buffer
			packer := NewPacker(&encoded)
			for i := range values {
				values[i] = uint64(i) * 0x9E3779B97F4A7C15 & mask
				if err := packer.Write(values[i], int(nBits)); err != nil {
					b.Fatal(err)
				}
			}
			if err := packer.Flush(); err != nil {
				b.Fatal(err)
			}

			data := encoded.Bytes()
			b.SetBytes(int64(len(data)))
			b.ResetTimer()
			var result uint64
			for b.Loop() {
				for i := range values {
					value, err := Extract(data, uint64(i)*uint64(nBits), nBits)
					if err != nil {
						b.Fatal(err)
					}
					result ^= value
				}
			}
			benchmarkExtractResult = result
		})
	}
}
