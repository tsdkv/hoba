package query_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/tsdkv/hoba/internal/index"
	"github.com/tsdkv/hoba/internal/pbt"
	"github.com/tsdkv/hoba/internal/query"
	"github.com/tsdkv/hoba/schema"
	"github.com/tsdkv/hoba/tokenizer"
)

func testSchema(t *testing.T) *schema.Schema {
	s, err := schema.NewSchema(
		schema.TextField("title", schema.Indexed(), schema.Stored()),
		schema.TextField("code", schema.Indexed(), schema.Keyword()),
		schema.Int64Field("year", schema.Indexed(), schema.Fast()),
	)
	require.NoError(t, err)

	return s
}

func matchModel(n query.Node, fields map[int]map[string]bool) bool {
	switch n := n.(type) {
	case *query.Term:
		return fields[n.FieldID][n.Text]
	case *query.MatchAll:
		return true
	case *query.Boost:
		return matchModel(n.Node, fields)
	case *query.Boolean:
		var musts, shoulds []query.Node

		for _, ch := range n.Children {
			switch ch.Occur {
			case query.OccurMust:
				musts = append(musts, ch.Node)
			case query.OccurShould:
				shoulds = append(shoulds, ch.Node)
			case query.OccurMustNot:
				if matchModel(ch.Node, fields) {
					return false
				}
			}
		}

		if len(musts) > 0 {
			for _, m := range musts {
				if !matchModel(m, fields) {
					return false
				}
			}

			return true
		}

		for _, sh := range shoulds {
			if matchModel(sh, fields) {
				return true
			}
		}

		return false
	default:
		panic(fmt.Sprintf("model: unexpected node %T", n))
	}
}

func tokenSets(tm *tokenizer.Manager, s *schema.Schema, doc schema.Document) map[int]map[string]bool {
	sets := map[int]map[string]bool{}

	for _, fv := range doc.Fields {
		f, _ := s.FieldByID(fv.Field)
		if f.Type != schema.FieldTypeText {
			continue
		}

		a, ok := tm.Get(f.Tokenizer)
		if !ok {
			continue
		}

		sets[fv.Field] = map[string]bool{}
		for tok := range a.Tokens(fv.Value.Str) {
			sets[fv.Field][tok.Text] = true
		}
	}

	return sets
}

// The model is the oracle: boolean semantics evaluated directly on document
// tokens must agree with the compiled scorer's match set.
func TestBooleanQueryMatchesModel(t *testing.T) {
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

		got := map[index.DocID]bool{}
		if sc := w.Scorer(seg); sc != nil {
			for d := sc.Doc(); d != index.Terminated; d = sc.Advance() {
				got[d] = true
			}
		}

		for i, doc := range docs {
			want := matchModel(q, tokenSets(tm, s, doc))
			require.Equal(rt, want, got[ids[i]], "doc %d %v, query %v", i, doc, q)
		}
	})
}

func TestIntersectionMatchesModel(t *testing.T) {
	checkFlatBoolean(t, query.OccurMust)
}

func TestDisjunctionMatchesModel(t *testing.T) {
	checkFlatBoolean(t, query.OccurShould)
}

// Flat homogeneous booleans over leaf-only subtrees: the boolean wrapper only
// delegates, so the match set comes straight from the scorer under test.
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

		got := map[index.DocID]bool{}
		if sc := w.Scorer(seg); sc != nil {
			for d := sc.Doc(); d != index.Terminated; d = sc.Advance() {
				got[d] = true
			}
		}

		for i, doc := range docs {
			want := matchModel(q, tokenSets(tm, s, doc))
			require.Equal(rt, want, got[ids[i]], "doc %d %v, query %v", i, doc, q)
		}
	})
}
