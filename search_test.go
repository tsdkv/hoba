package hoba

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tsdkv/hoba/internal/index"
	"github.com/tsdkv/hoba/schema"
	"github.com/tsdkv/hoba/tokenizer"
)

func testSegment(t *testing.T) *index.MemSegment {
	t.Helper()
	s, err := schema.NewSchema(schema.TextField("body", schema.Indexed()))
	require.NoError(t, err)
	seg := index.NewMemSegment(s, tokenizer.NewManager())
	for _, text := range []string{"foo bar", "foo", "bar"} {
		var doc schema.Document
		doc.Text(0, text)
		_, err := seg.Add(doc)
		require.NoError(t, err)
	}
	return seg
}

func TestSearch(t *testing.T) {
	seg := testSegment(t)

	hits, err := Search(seg, "body:foo", 10)
	require.NoError(t, err)
	require.Len(t, hits, 2)

	// N=3, df=2, avgdl=4/3: idf = ln(1 + 1.5/2.5) = ln(1.6) ≈ 0.470004
	// doc 1: tf=1, len=1 → idf * 2.2/1.975
	// doc 0: tf=1, len=2 → idf * 2.2/2.65
	assert.Equal(t, index.DocID(1), hits[0].Doc)
	assert.InDelta(t, 0.523548, hits[0].Score, 1e-6)
	assert.Equal(t, index.DocID(0), hits[1].Doc)
	assert.InDelta(t, 0.390192, hits[1].Score, 1e-6)
}

func TestSearchTopK(t *testing.T) {
	seg := testSegment(t)

	hits, err := Search(seg, "body:foo", 1)
	require.NoError(t, err)
	require.Len(t, hits, 1)
	assert.Equal(t, index.DocID(1), hits[0].Doc)
}

func TestSearchDefaultField(t *testing.T) {
	seg := testSegment(t)

	fieldless, err := Search(seg, "foo", 10)
	require.NoError(t, err)
	qualified, err := Search(seg, "body:foo", 10)
	require.NoError(t, err)
	assert.Equal(t, qualified, fieldless)
}

func TestSearchNoMatch(t *testing.T) {
	seg := testSegment(t)

	hits, err := Search(seg, "body:zzz", 10)
	require.NoError(t, err)
	assert.Empty(t, hits)
}
