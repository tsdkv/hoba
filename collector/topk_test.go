package collector

import (
	"slices"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tsdkv/hoba/index"
	"pgregory.net/rapid"
)

func TestTopK(t *testing.T) {
	t.Run("ties ordered by doc id", func(t *testing.T) {
		c := NewTopK(3)
		c.Collect(2, 1.0)
		c.Collect(0, 1.0)
		c.Collect(1, 1.0)
		assert.Equal(t, []Hit{{0, 1.0}, {1, 1.0}, {2, 1.0}}, c.Hits())

		full := NewTopK(3)
		full.Collect(5, 5.0)
		full.Collect(6, 5.0)
		full.Collect(7, 5.0)
		full.Collect(1, 5.0) // equal score, lower docID: evicts doc 7
		assert.Equal(t, []Hit{{1, 5.0}, {5, 5.0}, {6, 5.0}}, full.Hits())
	})

	t.Run("hits does not alias heap", func(t *testing.T) {
		c := NewTopK(2)
		c.Collect(0, 1.0)
		c.Collect(1, 2.0)
		got := c.Hits()
		require.Len(t, got, 2)
		got[0] = Hit{Doc: 99, Score: 99}
		assert.NotEqual(t, index.DocID(99), c.Hits()[0].Doc, "Hits() aliases internal state")
	})
}

// bruteForceTopK is the oracle: sort everything, take hits [offset, offset+k).
func bruteForceTopK(hits []Hit, offset, k int) []Hit {
	offset = max(offset, 0)
	k = max(k, 0)
	sorted := slices.Clone(hits)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].better(sorted[j]) })
	sorted = sorted[:min(offset+k, len(sorted))]
	return sorted[min(offset, len(sorted)):]
}

func TestTopKPBT(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		scores := rapid.SliceOfN(rapid.IntRange(0, 5), 0, 100).Draw(rt, "scores")
		k := rapid.IntRange(-5, 20).Draw(rt, "k")
		offset := rapid.IntRange(0, 30).Draw(rt, "offset")

		hits := make([]Hit, len(scores))
		for doc, s := range scores {
			hits[doc] = Hit{Doc: index.DocID(doc), Score: float64(s)}
		}

		want := bruteForceTopK(hits, offset, k)

		c := NewTopKRange(offset, k)
		for _, h := range hits {
			c.Collect(h.Doc, h.Score)
		}
		require.Equal(rt, want, c.Hits(), "k=%d offset=%d scores=%v", k, offset, scores)

		// Metamorphic: permuting the input must not change the result.
		pc := NewTopKRange(offset, k)

		perm := rapid.Permutation(hits).Draw(rt, "perm")
		for _, h := range perm {
			pc.Collect(h.Doc, h.Score)
		}
		require.Equal(rt, want, pc.Hits(), "permuted input: k=%d offset=%d scores=%v", k, offset, scores)
	})
}
