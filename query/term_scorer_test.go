package query

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tsdkv/hoba/index"
	"github.com/tsdkv/hoba/schema"
	"github.com/tsdkv/hoba/tokenizer"
)

// Same layout as TestBM25HandComputed: 3 docs, term in 2, lengths 1/2/5,
// so the end-to-end scores must equal the hand-computed bm25 values.
func TestTermScorerScore(t *testing.T) {
	tm := tokenizer.NewManager()
	s, err := schema.NewSchema(schema.TextField("body", schema.Indexed()))
	require.NoError(t, err)

	m := index.NewMemSegment(s, tm)
	for _, text := range []string{"apple", "apple apple", "banana banana banana banana banana"} {
		var d schema.Document
		d.Text(0, text)
		_, err = m.Add(d)
		require.NoError(t, err)
	}

	ts := newTermScorer(m, 0, "apple")
	require.NotNil(t, ts)

	require.Equal(t, index.DocID(0), ts.Advance())
	assert.InDelta(t, 0.6314552576125915, ts.Score(), 1e-12)

	require.Equal(t, index.DocID(1), ts.Advance())
	assert.InDelta(t, 0.6951314180441133, ts.Score(), 1e-12)

	assert.Equal(t, index.Terminated, ts.Advance())
}

func TestTermScorerMissingTerm(t *testing.T) {
	tm := tokenizer.NewManager()
	s, err := schema.NewSchema(schema.TextField("body", schema.Indexed()))
	require.NoError(t, err)

	assert.Nil(t, newTermScorer(index.NewMemSegment(s, tm), 0, "apple"))
}
