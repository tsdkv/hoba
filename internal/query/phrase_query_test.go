package query_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/tsdkv/hoba/internal/index"
	"github.com/tsdkv/hoba/internal/pbt"
	"github.com/tsdkv/hoba/internal/query"
	"github.com/tsdkv/hoba/schema"
	"github.com/tsdkv/hoba/tokenizer"
)

func TestPhraseQueryMatchesOracle(t *testing.T) {
	s, err := schema.NewSchema(schema.TextField("body", schema.Indexed(), schema.WithPositions()))
	require.NoError(t, err)
	tm := tokenizer.NewManager()

	rapid.Check(t, func(rt *rapid.T) {
		docs := rapid.SliceOfN(
			pbt.DocumentGenerator(s, pbt.Words, pbt.WithFieldProbability(100)),
			1, 30,
		).Draw(rt, "docs")
		q := pbt.PhraseGenerator(s, pbt.Words, docs).Draw(rt, "phrase")

		seg := index.NewMemSegment(s, tm)
		ids := make([]index.DocID, len(docs))
		for i, doc := range docs {
			id, err := seg.Add(doc)
			require.NoError(rt, err)
			ids[i] = id
		}

		w := query.Compile(q, seg)
		require.NotNil(rt, w)

		got := map[index.DocID]float64{}
		hits := pbt.CheckScorer(rt, seg.NumDocs(), func() query.Scorer { return w.Scorer(seg) })
		for _, h := range hits {
			got[h.Doc] = h.Score
		}

		o := newOracle(tm, s, docs)
		for i, doc := range docs {
			matched, score := o.eval(q, o.docs[i])
			gotScore, ok := got[ids[i]]
			assert.Equal(rt, matched, ok, "doc %d %v, query %v", i, doc, q)
			if matched && ok {
				assert.InDelta(rt, score, gotScore, 1e-9, "doc %d %v, query %v", i, doc, q)
			}
		}
	})
}
