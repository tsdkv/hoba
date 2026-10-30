package collector

import (
	"slices"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/tsdkv/hoba/internal/index"
)

func TestTopK(t *testing.T) {
	addr := func(doc index.DocID) index.DocAddress {
		return index.DocAddress{SegmentOrd: 0, DocID: doc}
	}

	t.Run("ties ordered by doc id", func(t *testing.T) {
		c := NewTopK(3)
		c.Collect(addr(2), 1.0)
		c.Collect(addr(0), 1.0)
		c.Collect(addr(1), 1.0)
		assert.Equal(t, []Hit{{addr(0), 1.0}, {addr(1), 1.0}, {addr(2), 1.0}}, c.Hits())

		full := NewTopK(3)
		full.Collect(addr(5), 5.0)
		full.Collect(addr(6), 5.0)
		full.Collect(addr(7), 5.0)
		full.Collect(addr(1), 5.0) // equal score, lower docID: evicts doc 7
		assert.Equal(t, []Hit{{addr(1), 5.0}, {addr(5), 5.0}, {addr(6), 5.0}}, full.Hits())
	})

	t.Run("ties ordered by segment ord before doc id", func(t *testing.T) {
		c := NewTopK(2)
		c.Collect(index.DocAddress{SegmentOrd: 1, DocID: 0}, 1.0)
		c.Collect(index.DocAddress{SegmentOrd: 0, DocID: 5}, 1.0)
		assert.Equal(t, []Hit{
			{index.DocAddress{SegmentOrd: 0, DocID: 5}, 1.0},
			{index.DocAddress{SegmentOrd: 1, DocID: 0}, 1.0},
		}, c.Hits())
	})

	t.Run("hits does not alias heap", func(t *testing.T) {
		c := NewTopK(2)
		c.Collect(addr(0), 1.0)
		c.Collect(addr(1), 2.0)
		got := c.Hits()
		require.Len(t, got, 2)
		got[0] = Hit{Doc: addr(99), Score: 99}
		assert.NotEqual(t, addr(99), c.Hits()[0].Doc, "Hits() aliases internal state")
	})
}

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
		n := rapid.IntRange(0, 100).Draw(rt, "n")
		k := rapid.IntRange(-5, n+5).Draw(rt, "k")
		offset := rapid.IntRange(0, n+10).Draw(rt, "offset")
		scores := rapid.SliceOfN(rapid.IntRange(0, 5), n, n).Draw(rt, "scores")
		ords := rapid.SliceOfN(rapid.IntRange(0, 2), n, n).Draw(rt, "segment ords")

		hits := make([]Hit, len(scores))
		for doc, s := range scores {
			hits[doc] = Hit{
				Doc:   index.DocAddress{SegmentOrd: ords[doc], DocID: index.DocID(doc)},
				Score: float64(s),
			}
		}

		want := bruteForceTopK(hits, offset, k)

		c := NewTopKRange(offset, k)
		for _, h := range hits {
			c.Collect(h.Doc, h.Score)
		}
		require.Equal(rt, want, c.Hits(), "k=%d offset=%d scores=%v ords=%v", k, offset, scores, ords)

		// Metamorphic: permuting the input must not change the result.
		pc := NewTopKRange(offset, k)

		perm := rapid.Permutation(hits).Draw(rt, "perm")
		for _, h := range perm {
			pc.Collect(h.Doc, h.Score)
		}
		require.Equal(rt, want, pc.Hits(), "permuted input: k=%d offset=%d scores=%v ords=%v", k, offset, scores, ords)
	})
}
