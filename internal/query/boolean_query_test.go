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

func TestBooleanQueryMatchesOracle(t *testing.T) {
	s := testSchema(t)
	tm := tokenizer.NewManager()

	rapid.Check(t, func(rt *rapid.T) {
		docs := rapid.SliceOfN(pbt.DocumentGenerator(s, pbt.Words), 1, 50).Draw(rt, "docs")
		q := pbt.QueryGenerator(s, pbt.Words, docs).Draw(rt, "query")

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

func TestIntersectionMatchesOracle(t *testing.T) {
	checkFlatBoolean(t, query.OccurMust)
}

func TestDisjunctionMatchesOracle(t *testing.T) {
	checkFlatBoolean(t, query.OccurShould)
}

func checkFlatBoolean(t *testing.T, occur query.Occur) {
	s := testSchema(t)
	tm := tokenizer.NewManager()

	rapid.Check(t, func(rt *rapid.T) {
		docs := rapid.SliceOfN(pbt.DocumentGenerator(s, pbt.Words), 1, 30).Draw(rt, "docs")
		leaves := rapid.SliceOfN(
			pbt.QueryGenerator(s, pbt.Words, docs, pbt.WithExcludedNodes(pbt.NodeBoolean)),
			1, 6,
		).Draw(rt, "leaves")

		children := make([]query.OccurNode, len(leaves))
		for i, l := range leaves {
			children[i] = query.OccurNode{Occur: occur, Node: l}
		}
		q := &query.Boolean{Children: children}

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

func testSchema(t *testing.T) *schema.Schema {
	s, err := schema.NewSchema(
		schema.TextField("title", schema.Indexed(), schema.Stored()),
		schema.TextField("code", schema.Indexed(), schema.Keyword()),
		schema.Int64Field("year", schema.Indexed(), schema.Fast()),
	)
	require.NoError(t, err)

	return s
}
