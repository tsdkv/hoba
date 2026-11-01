package pbt

import (
	"fmt"

	"pgregory.net/rapid"

	"github.com/tsdkv/hoba/internal/query"
	"github.com/tsdkv/hoba/schema"
	"github.com/tsdkv/hoba/tokenizer"
)

var occurs = []query.Occur{query.OccurShould, query.OccurMust, query.OccurMustNot}

type queryConfig struct {
	maxDepth int
	hitPct   int

	noTerm, noMatchAll, noBoost,
	noBoolean, noPhrase bool
}

type QueryOption func(*queryConfig)

func WithMaxDepth(n int) QueryOption {
	return func(c *queryConfig) { c.maxDepth = n }
}

// WithHitRate sets how often (in %) a term or phrase leaf is drawn from the
// actual docs, making it a guaranteed hit; the rest come from vocab or are misses.
func WithHitRate(pct int) QueryOption {
	return func(c *queryConfig) { c.hitPct = pct }
}

type NodeKind int

const (
	NodeTerm NodeKind = iota
	NodeMatchAll
	NodeBoost
	NodeBoolean
	NodePhrase
)

func WithExcludedNodes(kinds ...NodeKind) QueryOption {
	return func(c *queryConfig) {
		for _, k := range kinds {
			switch k {
			case NodeTerm:
				c.noTerm = true
			case NodeMatchAll:
				c.noMatchAll = true
			case NodeBoost:
				c.noBoost = true
			case NodeBoolean:
				c.noBoolean = true
			case NodePhrase:
				c.noPhrase = true
			}
		}
	}
}

func QueryGenerator(sch *schema.Schema, vocab []string, docs []schema.Document, opts ...QueryOption) *rapid.Generator[query.Node] {
	cfg := queryConfig{maxDepth: 3, hitPct: 80}
	for _, o := range opts {
		o(&cfg)
	}
	if cfg.noTerm && cfg.noMatchAll {
		panic("pbt: WithExcludedNodes cannot exclude both Term and MatchAll")
	}

	g := &queryGen{
		cfg:          cfg,
		vocab:        vocab,
		corpus:       map[int][]string{},
		phraseCorpus: map[int][][]tokenizer.Token{},
	}
	tm := tokenizer.NewManager()

	for _, f := range sch.Fields {
		if f.Type == schema.FieldTypeText && f.Indexed {
			g.fields = append(g.fields, f)
			if f.Positions {
				g.phraseFields = append(g.phraseFields, f)
			}
		}
	}

	for _, doc := range docs {
		for _, fv := range doc.Fields {
			f, ok := sch.FieldByID(fv.Field)
			if !ok || f.Type != schema.FieldTypeText || !f.Indexed {
				continue
			}
			an, ok := tm.Get(f.Tokenizer)
			if !ok {
				continue
			}

			var tokens []tokenizer.Token
			for tok := range an.Tokens(fv.Value.Str) {
				g.corpus[fv.Field] = append(g.corpus[fv.Field], tok.Text)
				tokens = append(tokens, tok)
			}
			if f.Positions && len(tokens) >= 2 {
				g.phraseCorpus[fv.Field] = append(g.phraseCorpus[fv.Field], tokens)
			}
		}
	}

	for _, f := range g.fields {
		if len(g.corpus[f.ID]) > 0 {
			g.hit = append(g.hit, f)
		}
		if len(g.phraseCorpus[f.ID]) > 0 {
			g.phraseHit = append(g.phraseHit, f)
		}
	}

	g.maxClauses = max(5, len(g.fields)+1)

	return rapid.Custom(g.top)
}

type queryGen struct {
	cfg        queryConfig
	fields     []schema.Field // Term can only target indexed text fields
	hit        []schema.Field // subset with corpus tokens
	corpus     map[int][]string
	vocab      []string
	maxClauses int

	phraseFields []schema.Field
	phraseHit    []schema.Field
	phraseCorpus map[int][][]tokenizer.Token
}

// Flat conjunctions and wide disjunctions exercise the merge paths hardest,
// so they get extra probability mass over the generic tree.
func (g *queryGen) top(rt *rapid.T) query.Node {
	if g.cfg.noBoolean {
		return g.leaf(rt)
	}

	switch {
	case CoinFlip(33).Draw(rt, "conj"):
		return g.flat(rt, query.OccurMust)
	case CoinFlip(12).Draw(rt, "disj"):
		return g.flat(rt, query.OccurShould)
	default:
		return g.node(rt, g.cfg.maxDepth)
	}
}

func (g *queryGen) flat(rt *rapid.T, occur query.Occur) query.Node {
	n := rapid.IntRange(2, 2*g.maxClauses).Draw(rt, "clauses")
	children := make([]query.OccurNode, n)
	for i := range children {
		children[i] = query.OccurNode{Occur: occur, Node: g.leaf(rt)}
	}
	return &query.Boolean{Children: children}
}

func (g *queryGen) node(rt *rapid.T, depth int) query.Node {
	if depth == 0 || CoinFlip(30).Draw(rt, "leaf") {
		return g.leaf(rt)
	}

	n := rapid.IntRange(1, g.maxClauses).Draw(rt, "clauses")
	children := make([]query.OccurNode, n)
	negative := 0

	for i := range children {
		occur := occurs[dieRoll(len(occurs)).Draw(rt, "occur")]
		if occur == query.OccurMustNot {
			negative++
		}
		children[i] = query.OccurNode{Occur: occur, Node: g.node(rt, depth-1)}
	}

	if negative == n {
		// an all-MustNot boolean matches nothing
		children[dieRoll(n).Draw(rt, "flip")].Occur = query.OccurMust
	}

	return &query.Boolean{Children: children}
}

func (g *queryGen) leaf(rt *rapid.T) query.Node {
	var n query.Node
	switch {
	case !g.cfg.noPhrase && len(g.phraseFields) > 0 &&
		(g.cfg.hitPct < 100 || len(g.phraseHit) > 0) && CoinFlip(20).Draw(rt, "phrase"):
		n = g.phrase(rt)
	case g.cfg.noTerm || len(g.fields) == 0:
		n = &query.MatchAll{}
	case g.cfg.noMatchAll:
		n = g.term(rt)
	case CoinFlip(5).Draw(rt, "matchAll"):
		n = &query.MatchAll{}
	default:
		n = g.term(rt)
	}

	if g.cfg.noBoost {
		return n
	}

	if CoinFlip(10).Draw(rt, "boost") {
		n = &query.Boost{Node: n, Factor: rapid.SampledFrom([]float64{0.2, 0.5, 2, 4}).Draw(rt, "factor")}
	}

	return n
}

func (g *queryGen) term(rt *rapid.T) query.Node {
	if CoinFlip(g.cfg.hitPct).Draw(rt, "hit") && len(g.hit) > 0 {
		f := rapid.SampledFrom(g.hit).Draw(rt, "hitField")
		return &query.Term{FieldID: f.ID, Text: rapid.SampledFrom(g.corpus[f.ID]).Draw(rt, "hitTerm")}
	}

	f := rapid.SampledFrom(g.fields).Draw(rt, "field")

	var text string
	if CoinFlip(80).Draw(rt, "invocab") {
		text = rapid.SampledFrom(g.vocab).Draw(rt, "vocabTerm")
	} else {
		text = fmt.Sprintf("zz%d", rapid.IntRange(0, 99).Draw(rt, "missTerm"))
	}

	return &query.Term{FieldID: f.ID, Text: text}
}

func (g *queryGen) phrase(rt *rapid.T) query.Node {
	if CoinFlip(g.cfg.hitPct).Draw(rt, "phraseHit") && len(g.phraseHit) > 0 {
		f := rapid.SampledFrom(g.phraseHit).Draw(rt, "phraseHitField")
		tokens := rapid.SampledFrom(g.phraseCorpus[f.ID]).Draw(rt, "phraseValue")
		length := rapid.IntRange(2, min(20, len(tokens))).Draw(rt, "phraseLength")
		start := rapid.IntRange(0, len(tokens)-length).Draw(rt, "phraseStart")

		terms := make([]string, length)
		positions := make([]int, length)
		base := tokens[start].Position
		for i, tok := range tokens[start : start+length] {
			terms[i] = tok.Text
			positions[i] = tok.Position - base
		}
		return &query.Phrase{FieldID: f.ID, Terms: terms, Positions: positions}
	}

	f := rapid.SampledFrom(g.phraseFields).Draw(rt, "phraseField")
	length := rapid.IntRange(2, 5).Draw(rt, "phraseLength")
	terms := make([]string, length)
	positions := make([]int, length)
	position := 0
	for i := range terms {
		if CoinFlip(80).Draw(rt, "phraseInVocab") {
			terms[i] = rapid.SampledFrom(g.vocab).Draw(rt, "phraseVocabTerm")
		} else {
			terms[i] = fmt.Sprintf("zz%d", rapid.IntRange(0, 99).Draw(rt, "phraseMissTerm"))
		}
		positions[i] = position
		position += rapid.IntRange(1, 3).Draw(rt, "phraseGap")
	}
	return &query.Phrase{FieldID: f.ID, Terms: terms, Positions: positions}
}
