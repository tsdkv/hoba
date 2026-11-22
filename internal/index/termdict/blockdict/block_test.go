package blockdict

import (
	"bytes"
	"encoding/binary"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func TestBlockEmpty(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, newBlock().Flush(&buf))

	br, err := newBlockReader(buf.Bytes())
	require.NoError(t, err)

	for _, term := range [][]byte{nil, []byte("a")} {
		ord, ok, err := br.Get(term)
		assert.NoError(t, err)
		assert.False(t, ok)
		assert.Zero(t, ord)
	}
}

func TestBlockReaderInvalidData(t *testing.T) {
	overflow := append(bytes.Repeat([]byte{0x80}, binary.MaxVarintLen64-1), 0x02)
	cases := []struct {
		name   string
		data   []byte
		lookup bool
		want   error
	}{
		{name: "truncated term count", data: []byte{0x80}, want: io.ErrUnexpectedEOF},
		{name: "overflowing term count", data: overflow, want: errInvalidBlock},
		{name: "missing first term length", data: []byte{0}, want: io.ErrUnexpectedEOF},
		{name: "overflowing first term length", data: append([]byte{0}, overflow...), want: errInvalidBlock},
		{name: "truncated first term", data: []byte{1, 2, 'a'}, want: io.ErrUnexpectedEOF},
		{name: "missing entry", data: []byte{2, 1, 'a'}, lookup: true, want: io.ErrUnexpectedEOF},
		{name: "truncated prefix length", data: []byte{2, 1, 'a', 0x80}, lookup: true, want: io.ErrUnexpectedEOF},
		{name: "overflowing prefix length", data: append([]byte{2, 1, 'a'}, overflow...), lookup: true, want: errInvalidBlock},
		{name: "invalid prefix length", data: []byte{2, 1, 'a', 2, 0}, lookup: true, want: errInvalidBlock},
		{name: "truncated suffix length", data: []byte{2, 1, 'a', 0, 0x80}, lookup: true, want: io.ErrUnexpectedEOF},
		{name: "overflowing suffix length", data: append([]byte{2, 1, 'a', 0}, overflow...), lookup: true, want: errInvalidBlock},
		{name: "truncated suffix", data: []byte{2, 1, 'a', 0, 2, 'b'}, lookup: true, want: io.ErrUnexpectedEOF},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			br, err := newBlockReader(tt.data)
			if !tt.lookup {
				assert.Nil(t, br)
				assert.ErrorIs(t, err, tt.want)
				return
			}
			require.NoError(t, err)

			ord, ok, err := br.Get([]byte("z"))
			assert.Zero(t, ord)
			assert.False(t, ok)
			assert.ErrorIs(t, err, tt.want)
		})
	}
}

func TestBlockRoundTrip(t *testing.T) {
	termsGen := rapid.Custom(func(t *rapid.T) [][]byte {
		numTerms := rapid.IntRange(1, 35).Draw(t, "num_terms")

		alphabet := []byte{0x00, 0x01, 'a', 'b', 0xfe, 0xff}

		prefixLen := rapid.SampledFrom([]int{1, 2, 8, 127, 128}).Draw(t, "prefix_len")
		prefix := rapid.SliceOfN(rapid.SampledFrom(alphabet), prefixLen, prefixLen).Draw(t, "prefix")

		terms := make([][]byte, 0, numTerms)
		var build func([]byte, int)
		build = func(prefix []byte, n int) {
			if n == 1 {
				terms = append(terms, bytes.Clone(prefix))
				return
			}
			if rapid.Bool().Draw(t, "terminal") {
				terms = append(terms, bytes.Clone(prefix))
				n--
			}

			counts := make([]int, len(alphabet))
			for range n {
				child := rapid.IntRange(0, len(alphabet)-1).Draw(t, "child")
				counts[child]++
			}
			for i, count := range counts {
				if count == 0 {
					continue
				}
				build(append(bytes.Clone(prefix), alphabet[i]), count)
			}
		}
		build(prefix, numTerms)
		return terms
	})

	rapid.Check(t, func(rt *rapid.T) {
		bw := newBlock()

		terms := termsGen.Draw(rt, "terms")
		numTerms := len(terms)
		want := make([][]byte, 0, numTerms)
		ids := make([]int, 0, numTerms)

		for i, term := range terms {
			err := bw.Add(term)
			if i+1 > maxBlockTerms {
				require.ErrorIs(rt, err, errBlockFull)
				continue
			}
			require.NoError(rt, err)

			want = append(want, term)
			ids = append(ids, i)
		}

		var buf bytes.Buffer
		require.NoError(rt, bw.Flush(&buf))

		br, err := newBlockReader(buf.Bytes())
		require.NoError(rt, err)

		order := rapid.Permutation(ids).Draw(rt, "order")
		for _, i := range order {
			ord, ok, err := br.Get(want[i])
			require.NoError(rt, err)
			require.True(rt, ok, "i=%d, want[i]=%v", i, want[i])
			require.Equal(rt, uint32(i), ord)
		}

		present := make(map[string]struct{}, len(want))
		for _, term := range want {
			present[string(term)] = struct{}{}
		}

		var misses [][]byte
		if _, ok := present[""]; !ok {
			misses = append(misses, nil)
		}
		misses = append(misses, append(bytes.Clone(want[len(want)-1]), 0))
		for i := 1; i < len(want); i++ {
			candidate := append(bytes.Clone(want[i-1]), 0)
			if bytes.Compare(candidate, want[i]) < 0 {
				misses = append(misses, candidate)
			}
		}
		for _, term := range want {
			if len(term) > 0 {
				prefix := bytes.Clone(term[:len(term)-1])
				if _, ok := present[string(prefix)]; !ok {
					misses = append(misses, prefix)
				}
			}
			extension := append(bytes.Clone(term), 0)
			if _, ok := present[string(extension)]; !ok {
				misses = append(misses, extension)
			}
		}
		for _, term := range misses {
			ord, ok, err := br.Get(term)
			require.NoError(rt, err)
			require.False(rt, ok, "term=%v", term)
			require.Zero(rt, ord)
		}
	})
}
