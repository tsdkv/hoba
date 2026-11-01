package query_test

import (
	"fmt"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tsdkv/hoba/internal/query"
	"github.com/tsdkv/hoba/schema"
	"github.com/tsdkv/hoba/tokenizer"
)

// oracle decides query semantics directly on document tokens, without
// touching the engine's scorers. It feeds its own corpus stats (tf, docLen,
// df, totals) into the engine's BM25 — pinned by TestBM25HandComputed — so a
// property failure means broken plumbing, not a restated formula. The
// numbers are pinned by TestOracleHandComputed.
type oracle struct {
	docs   []oracleDoc
	dfs    map[int]map[string]int
	totals map[int]uint64
}

type oracleDoc struct {
	tfs       map[int]map[string]uint32 // field → term → frequency
	positions map[int]map[string][]int  // field → term → positions
	lens      map[int]uint32            // field → token count
}

func newOracle(tm *tokenizer.Manager, s *schema.Schema, docs []schema.Document) *oracle {
	o := &oracle{dfs: map[int]map[string]int{}, totals: map[int]uint64{}}

	for _, doc := range docs {
		od := oracleDoc{
			tfs:       map[int]map[string]uint32{},
			positions: map[int]map[string][]int{},
			lens:      map[int]uint32{},
		}

		for _, fv := range doc.Fields {
			f, ok := s.FieldByID(fv.Field)
			if !ok || f.Type != schema.FieldTypeText || !f.Indexed {
				continue
			}

			a, ok := tm.Get(f.Tokenizer)
			if !ok {
				continue
			}

			tfs := map[string]uint32{}
			positions := map[string][]int{}
			var n uint32
			for tok := range a.Tokens(fv.Value.Str) {
				tfs[tok.Text]++
				positions[tok.Text] = append(positions[tok.Text], tok.Position)
				n++
			}

			od.tfs[fv.Field] = tfs
			od.positions[fv.Field] = positions
			od.lens[fv.Field] = n
			o.totals[fv.Field] += uint64(n)

			if o.dfs[fv.Field] == nil {
				o.dfs[fv.Field] = map[string]int{}
			}
			for term := range tfs {
				o.dfs[fv.Field][term]++
			}
		}

		o.docs = append(o.docs, od)
	}

	return o
}

func (o *oracle) eval(n query.Node, d oracleDoc) (bool, float64) {
	switch n := n.(type) {
	case *query.Term:
		tf := d.tfs[n.FieldID][n.Text]
		if tf == 0 {
			return false, 0
		}

		return true, query.BM25Score(len(o.docs), o.dfs[n.FieldID][n.Text], o.totals[n.FieldID], tf, d.lens[n.FieldID])
	case *query.Phrase:
		if len(n.Terms) == 0 {
			return false, 0
		}
		if len(n.Terms) == 1 {
			return o.eval(&query.Term{FieldID: n.FieldID, Text: n.Terms[0]}, d)
		}

		var phraseCount uint32
		for _, start := range d.positions[n.FieldID][n.Terms[0]] {
			matched := true
			for i, term := range n.Terms[1:] {
				position := i + 1
				if i+1 < len(n.Positions) {
					position = n.Positions[i+1]
				}
				if !slices.Contains(d.positions[n.FieldID][term], start+position) {
					matched = false
					break
				}
			}
			if matched {
				phraseCount++
			}
		}
		if phraseCount == 0 {
			return false, 0
		}

		var score float64
		for _, term := range n.Terms {
			score += query.BM25Score(len(o.docs), o.dfs[n.FieldID][term], o.totals[n.FieldID], phraseCount, d.lens[n.FieldID])
		}
		return true, score
	case *query.MatchAll:
		return true, 1
	case *query.Boost:
		matched, score := o.eval(n.Node, d)
		return matched, score * n.Factor
	case *query.Boolean:
		// musts and shoulds accumulate separately (musts first), matching
		// booleanScorer's summation order.
		var mustScore, shouldScore float64
		hasMust, matchedShould := false, false

		for _, ch := range n.Children {
			matched, score := o.eval(ch.Node, d)
			switch ch.Occur {
			case query.OccurMustNot:
				if matched {
					return false, 0
				}
			case query.OccurMust:
				if !matched {
					return false, 0
				}
				hasMust = true
				mustScore += score
			case query.OccurShould:
				if matched {
					matchedShould = true
					shouldScore += score
				}
			}
		}

		if hasMust {
			return true, mustScore + shouldScore
		}
		if matchedShould {
			return true, shouldScore
		}
		return false, 0
	default:
		panic(fmt.Sprintf("oracle: unexpected node %T", n))
	}
}

// Docs "a b a", "b", "a b c" on one field: lens 3/1/3, avgdl 7/3, df(a)=2,
// df(b)=3. idf(a) = ln(1.6), same corpus shape as TestBM25HandComputed.
func TestOracleHandComputed(t *testing.T) {
	s, err := schema.NewSchema(schema.TextField("body", schema.Indexed()))
	require.NoError(t, err)
	tm := tokenizer.NewManager()

	var docs []schema.Document
	for _, text := range []string{"a b a", "b", "a b c"} {
		var d schema.Document
		d.Text(0, text)
		docs = append(docs, d)
	}

	o := newOracle(tm, s, docs)
	a := &query.Term{FieldID: 0, Text: "a"}
	b := &query.Term{FieldID: 0, Text: "b"}

	matched, score := o.eval(a, o.docs[0])
	require.True(t, matched)
	assert.InDelta(t, 0.59818643722184528, score, 1e-12)

	q := &query.Boolean{Children: []query.OccurNode{
		{Occur: query.OccurMust, Node: a},
		{Occur: query.OccurShould, Node: b},
	}}

	matched, score = o.eval(q, o.docs[0])
	require.True(t, matched)
	assert.InDelta(t, 0.71774361433915035, score, 1e-12)

	matched, _ = o.eval(q, o.docs[1])
	require.False(t, matched)

	matched, score = o.eval(q, o.docs[2])
	require.True(t, matched)
	assert.InDelta(t, 0.54037438004662641, score, 1e-12)

	phrase := &query.Phrase{FieldID: 0, Terms: []string{"a", "b"}, Positions: []int{0, 1}}
	matched, score = o.eval(phrase, o.docs[0])
	require.True(t, matched)
	assert.InDelta(t, 0.54037438004662641, score, 1e-12)

	matched, _ = o.eval(phrase, o.docs[1])
	require.False(t, matched)

	matched, score = o.eval(phrase, o.docs[2])
	require.True(t, matched)
	assert.InDelta(t, 0.54037438004662641, score, 1e-12)

	matched, score = o.eval(&query.Boost{Node: b, Factor: 2}, o.docs[0])
	require.True(t, matched)
	assert.InDelta(t, 0.23911435423461019, score, 1e-12)
}
