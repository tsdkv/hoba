package pbt

import (
	"testing"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/tsdkv/hoba/internal/index"
	"github.com/tsdkv/hoba/internal/query"
	"github.com/tsdkv/hoba/schema"
	"github.com/tsdkv/hoba/tokenizer"
)

// If this fails, fix the generator before trusting any property that draws from it.
func TestQueryGeneratorValidity(t *testing.T) {
	s := testSchema(t)
	tm := tokenizer.NewManager()

	rapid.Check(t, func(rt *rapid.T) {
		docs := rapid.SliceOfN(DocumentGenerator(s, Words), 1, 30).Draw(rt, "docs")
		q := QueryGenerator(s, Words, docs).Draw(rt, "query")

		assertValidQuery(rt, s, q)

		seg := index.NewMemSegment(s, tm)
		for _, doc := range docs {
			_, err := seg.Add(doc)
			require.NoError(rt, err)
		}

		w := query.Compile(q, seg)
		if w == nil {
			return
		}

		CheckScorer(rt, seg.NumDocs(), func() query.Scorer { return w.Scorer(seg) })
	})
}

// WithHitRate(100) every Term and Phrase leaf matches at least one indexed doc.
func TestQueryGeneratorHits(t *testing.T) {
	s := testSchema(t)
	tm := tokenizer.NewManager()

	rapid.Check(t, func(rt *rapid.T) {
		docs := rapid.SliceOfN(DocumentGenerator(s, Words, WithFieldProbability(100)), 1, 30).Draw(rt, "docs")
		q := QueryGenerator(s, Words, docs, WithHitRate(100)).Draw(rt, "query")

		present := map[int]map[string]bool{}
		for _, doc := range docs {
			for _, fv := range doc.Fields {
				f, _ := s.FieldByID(fv.Field)
				if f.Type != schema.FieldTypeText || !f.Indexed {
					continue
				}
				if present[fv.Field] == nil {
					present[fv.Field] = map[string]bool{}
				}
				an, _ := tm.Get(f.Tokenizer)
				for tok := range an.Tokens(fv.Value.Str) {
					present[fv.Field][tok.Text] = true
				}
			}
		}

		seg := index.NewMemSegment(s, tm)
		for _, doc := range docs {
			_, err := seg.Add(doc)
			require.NoError(rt, err)
		}

		assertQueryHits(rt, seg, present, q)
	})
}

func TestQueryGeneratorExclusions(t *testing.T) {
	s := testSchema(t)

	cases := []struct {
		name     string
		excluded []NodeKind
	}{
		{"term", []NodeKind{NodeTerm}},
		{"matchAll", []NodeKind{NodeMatchAll}},
		{"boost", []NodeKind{NodeBoost}},
		{"boolean", []NodeKind{NodeBoolean}},
		{"phrase", []NodeKind{NodePhrase}},
		{"term+boost", []NodeKind{NodeTerm, NodeBoost}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gen := QueryGenerator(s, Words, nil, WithExcludedNodes(tc.excluded...))
			rapid.Check(t, func(rt *rapid.T) {
				eachNode(gen.Draw(rt, "query"), func(n query.Node) {
					switch n.(type) {
					case *query.Term:
						require.NotContains(rt, tc.excluded, NodeTerm)
					case *query.MatchAll:
						require.NotContains(rt, tc.excluded, NodeMatchAll)
					case *query.Boost:
						require.NotContains(rt, tc.excluded, NodeBoost)
					case *query.Boolean:
						require.NotContains(rt, tc.excluded, NodeBoolean)
					case *query.Phrase:
						require.NotContains(rt, tc.excluded, NodePhrase)
					}
				})
			})
		})
	}

	// Term and MatchAll cannot both be excluded.
	require.Panics(t, func() {
		QueryGenerator(s, Words, nil, WithExcludedNodes(NodeTerm, NodeMatchAll))
	})
}

func eachNode(n query.Node, fn func(query.Node)) {
	fn(n)
	switch n := n.(type) {
	case *query.Boost:
		eachNode(n.Node, fn)
	case *query.Boolean:
		for _, ch := range n.Children {
			eachNode(ch.Node, fn)
		}
	}
}

func assertValidQuery(t *rapid.T, s *schema.Schema, n query.Node) {
	switch n := n.(type) {
	case *query.Term:
		f, ok := s.FieldByID(n.FieldID)
		require.True(t, ok, "unknown field ID %d", n.FieldID)
		require.True(t, f.Indexed && f.Type == schema.FieldTypeText, "term on non-searchable field %q", f.Name)
		require.NotEmpty(t, n.Text)
	case *query.Phrase:
		f, ok := s.FieldByID(n.FieldID)
		require.True(t, ok, "unknown field ID %d", n.FieldID)
		require.True(t, f.Indexed && f.Type == schema.FieldTypeText && f.Positions, "phrase on field without positions %q", f.Name)
		require.GreaterOrEqual(t, len(n.Terms), 2)
		require.Len(t, n.Positions, len(n.Terms))
		require.Zero(t, n.Positions[0])
		for i, term := range n.Terms {
			require.NotEmpty(t, term)
			if i > 0 {
				require.Greater(t, n.Positions[i], n.Positions[i-1])
			}
		}
	case *query.Boost:
		require.Greater(t, n.Factor, 0.0)
		assertValidQuery(t, s, n.Node)
	case *query.Boolean:
		require.NotEmpty(t, n.Children)
		positive := false
		for _, ch := range n.Children {
			positive = positive || ch.Occur != query.OccurMustNot
			assertValidQuery(t, s, ch.Node)
		}

		require.True(t, positive, "all-MustNot boolean")
	case *query.MatchAll:
	default:
		require.Failf(t, "unexpected node type", "%T", n)
	}
}

func assertQueryHits(t *rapid.T, seg *index.MemSegment, present map[int]map[string]bool, n query.Node) {
	switch n := n.(type) {
	case *query.Term:
		require.True(t, present[n.FieldID][n.Text], "term %q not in corpus", n.Text)
		require.Positive(t, seg.DocFreq(n.FieldID, n.Text), "term %q not indexed", n.Text)
	case *query.Phrase:
		for _, term := range n.Terms {
			require.True(t, present[n.FieldID][term], "term %q not in corpus", term)
			require.Positive(t, seg.DocFreq(n.FieldID, term), "term %q not indexed", term)
		}
		w := query.Compile(n, seg)
		require.NotNil(t, w, "phrase %q has no weight", n.String())
		require.NotNil(t, w.Scorer(seg), "phrase %q has no hits", n.String())
	case *query.Boost:
		assertQueryHits(t, seg, present, n.Node)
	case *query.Boolean:
		for _, ch := range n.Children {
			assertQueryHits(t, seg, present, ch.Node)
		}
	}
}
