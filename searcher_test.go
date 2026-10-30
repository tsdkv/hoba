package hoba

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tsdkv/hoba/internal/index"
	"github.com/tsdkv/hoba/internal/query"
	"github.com/tsdkv/hoba/schema"
	"github.com/tsdkv/hoba/tokenizer"
)

// "apple" appears in 3 of 4 docs, field lengths 1/1/2/1. Scores are
// hand-computed BM25 (k1=1.2, b=0.75) against snapshot-wide stats:
// N=4, df=3, totalTokens=5, avgdl=1.25.
func TestSearcherSearch(t *testing.T) {
	tm := tokenizer.NewManager()
	s, err := schema.NewSchema(schema.TextField("body", schema.Indexed()))
	require.NoError(t, err)

	newSeg := func(texts ...string) *index.MemSegment {
		m := index.NewMemSegment(s, tm)
		for _, text := range texts {
			var d schema.Document
			d.Text(0, text)
			_, err := m.Add(d)
			require.NoError(t, err)
		}
		return m
	}
	seg0 := newSeg("apple", "banana")
	seg1 := newSeg("apple pie", "apple")

	parser, err := query.NewParser(s, query.DefaultFields("body"))
	require.NoError(t, err)
	sch := newSearcher(parser, newSnapshot(0, []index.Segment{seg0, seg1}))

	hits, err := sch.Search("apple", 10)
	require.NoError(t, err)
	require.Len(t, hits, 3)

	// tf=1, doclen=1
	assert.Equal(t, DocAddress{SegmentOrd: 0, DocID: 0}, hits[0].Doc)
	assert.InDelta(t, 0.38845785973525315, hits[0].Score, 1e-12)

	// same score as hits[0]; tie broken by segment ord, then doc id
	assert.Equal(t, DocAddress{SegmentOrd: 1, DocID: 1}, hits[1].Doc)
	assert.InDelta(t, 0.38845785973525315, hits[1].Score, 1e-12)

	// tf=1, doclen=2
	assert.Equal(t, DocAddress{SegmentOrd: 1, DocID: 0}, hits[2].Doc)
	assert.InDelta(t, 0.28638134184861724, hits[2].Score, 1e-12)

	doc, err := sch.Doc(hits[0].Doc)
	require.NoError(t, err)
	require.Len(t, doc.Fields, 1)
	assert.Equal(t, "apple", doc.Fields[0].Value.Str)

	hits, err = sch.Search("apple", 2)
	require.NoError(t, err)
	assert.Len(t, hits, 2)

	hits, err = sch.Search("kiwi", 10)
	require.NoError(t, err)
	assert.Empty(t, hits)
}
