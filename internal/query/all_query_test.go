package query

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tsdkv/hoba/internal/index"
	"github.com/tsdkv/hoba/schema"
	"github.com/tsdkv/hoba/tokenizer"
)

// Regression lock: doc IDs are 0-based, so a 3-doc segment must terminate
// right after doc 2 — the old guard emitted a phantom doc 3.
func TestMatchAllScorer(t *testing.T) {
	tm := tokenizer.NewManager()
	s, err := schema.NewSchema(schema.TextField("body", schema.Indexed()))
	require.NoError(t, err)

	m := index.NewMemSegment(s, tm)
	assert.Nil(t, matchAllWeight{}.Scorer(m))

	for range 3 {
		var d schema.Document
		d.Text(0, "x")
		_, err = m.Add(d)
		require.NoError(t, err)
	}

	sc := matchAllWeight{}.Scorer(m)
	require.NotNil(t, sc)
	for doc := index.DocID(0); doc < 3; doc++ {
		require.Equal(t, doc, sc.Doc())
		assert.Equal(t, 1.0, sc.Score())
		sc.Advance()
	}
	assert.Equal(t, index.Terminated, sc.Doc())
}
