package terminfo

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func TestBlockMetaRoundTrip(t *testing.T) {
	blockMetaGen := rapid.Custom(func(rt *rapid.T) blockMeta {
		return blockMeta{
			offset:         rapid.Uint64().Draw(rt, "offset"),
			docFreqNBits:   rapid.Uint8().Draw(rt, "docFreqNBits"),
			postingsNBits:  rapid.Uint8().Draw(rt, "postingsNBits"),
			positionsNBits: rapid.Uint8().Draw(rt, "positionsNBits"),

			ref: TermInfo{
				DocFreq:         rapid.Uint32().Draw(rt, "DocFreq"),
				PostingsOffset:  rapid.Uint64().Draw(rt, "PostingsOffset"),
				PostingsLength:  rapid.Uint32().Draw(rt, "PostingsLength"),
				PositionsOffset: rapid.Uint64().Draw(rt, "PositionsOffset"),
				PositionsLength: rapid.Uint32().Draw(rt, "PostingsLength"),
			},
		}
	})

	rapid.Check(t, func(rt *rapid.T) {
		bm := blockMetaGen.Draw(rt, "blockMeta")

		var buf bytes.Buffer
		require.NoError(rt, bm.Serialize(&buf))
		require.Len(rt, buf.Bytes(), blockMetaSize)

		got, err := deserializeBlockMeta(buf.Bytes())
		require.NoError(rt, err)

		require.Equal(rt, bm, *got)
	})
}

func TestTermInfoRoundTrip(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		numValues := rapid.SampledFrom([]int{
			1, 2, 3,
			blockLen - 1, blockLen, blockLen + 1,
			2*blockLen - 1, 2 * blockLen, 2*blockLen + 1,
			4 * blockLen,
		}).Draw(rt, "numValues")

		termInfo := TermInfo{
			PostingsOffset:  rapid.Uint64Range(0, maxDelta/2).Draw(rt, "PostingsOffset"),
			PositionsOffset: rapid.Uint64Range(0, maxDelta/2).Draw(rt, "PositionsOffset"),
		}

		want := make([]TermInfo, numValues)
		w := NewWriter()
		for i := range numValues {
			termInfo.DocFreq = rapid.Uint32().Draw(rt, "DocFreq")
			termInfo.PostingsLength = rapid.Uint32().Draw(rt, "PostingsLength")
			termInfo.PositionsLength = rapid.Uint32().Draw(rt, "PositionsLength")

			want[i] = termInfo
			require.NoError(rt, w.Write(termInfo))
			termInfo.PostingsOffset += uint64(termInfo.PostingsLength)
			termInfo.PositionsOffset += uint64(termInfo.PositionsLength)
		}

		var buf bytes.Buffer
		require.NoError(rt, w.Finish(&buf))

		r, err := NewReader(buf.Bytes())
		require.NoError(rt, err)
		require.Equal(rt, uint64(numValues), r.NumTerms())

		indices := make([]int, numValues)
		for i := range indices {
			indices[i] = i
		}
		order := rapid.Permutation(indices).Draw(rt, "order")
		for _, i := range order {
			ti, err := r.Get(uint64(i))
			require.NoError(rt, err)
			require.Equal(rt, want[i], ti)
		}
	})
}
