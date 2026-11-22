package blockdict

import (
	"bytes"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func TestBlockDictRoundTrip(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		numTerms := rapid.Uint32Range(0, 1000).Draw(rt, "numTerms")

		term := rapid.SliceOfN(rapid.Byte(), 1, 100)
		terms := rapid.SliceOfNDistinct(term, int(numTerms), int(numTerms), func(e []byte) string {
			return string(e)
		}).Draw(rt, "terms")
		slices.SortFunc(terms, bytes.Compare)

		var buf bytes.Buffer
		bw := NewWriter(&buf)

		want := make([][]byte, 0, numTerms)
		ids := make([]int, 0, numTerms)
		for i := range numTerms {
			require.NoError(rt, bw.Insert(terms[i]))
			want = append(want, terms[i])
			ids = append(ids, int(i))
		}
		require.NoError(rt, bw.Finish())

		br, err := NewReader(buf.Bytes())
		require.NoError(rt, err)
		order := rapid.Permutation(ids).Draw(rt, "order")

		for _, i := range order {
			ord, ok, err := br.Get(want[i])
			require.NoError(rt, err)
			require.True(rt, ok)
			require.Equal(rt, uint32(i), ord)
		}
	})
}
