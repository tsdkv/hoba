package query

import (
	"errors"
	"fmt"

	"github.com/tsdkv/hoba/query/parser"
	"github.com/tsdkv/hoba/schema"
	"github.com/tsdkv/hoba/tokenizer"
)

// resolver carries per-call state: the running clause count checked
// against the limit.
type resolver struct {
	p       *Parser
	clauses int
}

// node resolves one AST node. A nil Node means the subtree was dropped
// because all its leaves analyzed to zero tokens.
func (r *resolver) node(n parser.Node) (Node, error) {
	switch t := n.(type) {
	case *parser.Term:
		return r.leaf(t.Start, t.Field, t.Text, false)
	case *parser.Phrase:
		return r.leaf(t.Start, t.Field, t.Text, true)
	case *parser.Boost:
		inner, err := r.node(t.Node)
		if err != nil {
			return nil, err
		}
		if inner == nil {
			return nil, nil // a boost of nothing is nothing
		}
		return &Boost{Node: inner, Factor: t.Factor}, nil
	case *parser.Clause:
		return r.clause(t)
	default:
		return nil, fmt.Errorf("query: unknown AST node %T", n)
	}
}

// clause resolves a boolean combination and applies the bottom-up rewrite:
// dropped children disappear, an empty clause is itself dropped, an
// all-negative clause gains a MatchAll Must child ("everything except"),
// and a single Must/Should child is unwrapped.
func (r *resolver) clause(c *parser.Clause) (Node, error) {
	children := make([]OccurNode, 0, len(c.Children))
	for _, ch := range c.Children {
		n, err := r.node(ch.Node)
		if err != nil {
			return nil, err
		}
		if n == nil {
			continue
		}
		children = append(children, OccurNode{Occur: r.occur(ch.Occur), Node: n})
	}
	if len(children) == 0 {
		return nil, nil
	}
	positive := false
	for _, ch := range children {
		if ch.Occur != OccurMustNot {
			positive = true
			break
		}
	}
	if !positive {
		children = append([]OccurNode{{Occur: OccurMust, Node: &MatchAll{}}}, children...)
	}
	if len(children) == 1 && children[0].Occur != OccurMustNot {
		return children[0].Node, nil
	}
	return &Boolean{Children: children}, nil
}

func (r *resolver) occur(o parser.Occur) Occur {
	switch o {
	case parser.OccurMust:
		return OccurMust
	case parser.OccurMustNot:
		return OccurMustNot
	case parser.OccurShould:
		return OccurShould
	default:
		return r.p.defaultOccur
	}
}

// leaf resolves one term or phrase leaf, expanding it over the default
// fields when it carries no explicit field prefix.
func (r *resolver) leaf(pos int, fieldName, text string, isPhrase bool) (Node, error) {
	if fieldName != "" {
		f, ok := r.p.s.Field(fieldName)
		if !ok {
			return nil, &Error{Pos: pos, Field: fieldName, Err: ErrUnknownField}
		}
		if f.Type != schema.FieldTypeText {
			return nil, &Error{Pos: pos, Field: fieldName, Err: ErrNotText}
		}
		if !f.Indexed {
			return nil, &Error{Pos: pos, Field: fieldName, Err: ErrNotIndexed}
		}
		return r.leafOnField(pos, f, text, isPhrase)
	}
	if len(r.p.defaultIDs) == 0 {
		return nil, &Error{Pos: pos, Err: ErrNoField}
	}
	// OR the per-default-field results, skipping fields that cannot take
	// the leaf; it is an error only if no default field can.
	var children []Node
	var firstNoPos error
	for _, id := range r.p.defaultIDs {
		n, err := r.leafOnField(pos, r.p.s.Fields[id], text, isPhrase)
		if err != nil {
			if errors.Is(err, ErrNoPositions) {
				if firstNoPos == nil {
					firstNoPos = err
				}
				continue
			}
			return nil, err
		}
		if n != nil {
			children = append(children, n)
		}
	}
	switch len(children) {
	case 0:
		if firstNoPos != nil {
			return nil, firstNoPos
		}
		return nil, nil
	case 1:
		return children[0], nil
	}
	or := make([]OccurNode, len(children))
	for i, n := range children {
		or[i] = OccurNode{Occur: OccurShould, Node: n}
	}
	return &Boolean{Children: or}, nil
}

// leafOnField analyzes text with the field's analyzer and builds the leaf:
// zero tokens drop it, one token makes a Term, several make a Phrase when
// the field has positions — otherwise an AND of the tokens for a bare
// term, ErrNoPositions for a phrase.
func (r *resolver) leafOnField(pos int, f schema.Field, text string, isPhrase bool) (Node, error) {
	var toks []tokenizer.Token
	for tok := range r.p.analyzers[f.ID].Tokens(text) {
		toks = append(toks, tok)
	}
	switch len(toks) {
	case 0:
		return nil, nil
	case 1:
		if err := r.count(pos, f); err != nil {
			return nil, err
		}
		return &Term{FieldID: f.ID, Text: toks[0].Text}, nil
	}
	if f.Positions {
		if err := r.count(pos, f); err != nil {
			return nil, err
		}
		terms := make([]string, len(toks))
		positions := make([]int, len(toks))
		for i, tok := range toks {
			terms[i] = tok.Text
			positions[i] = tok.Position - toks[0].Position
		}
		return &Phrase{FieldID: f.ID, Terms: terms, Positions: positions}, nil
	}
	if isPhrase {
		return nil, &Error{Pos: pos, Field: f.Name, Err: ErrNoPositions}
	}
	children := make([]OccurNode, len(toks))
	for i, tok := range toks {
		if err := r.count(pos, f); err != nil {
			return nil, err
		}
		children[i] = OccurNode{Occur: OccurMust, Node: &Term{FieldID: f.ID, Text: tok.Text}}
	}
	return &Boolean{Children: children}, nil
}

// count charges one emitted Term or Phrase against the clause limit.
func (r *resolver) count(pos int, f schema.Field) error {
	r.clauses++
	if r.clauses > r.p.maxClauses {
		return &Error{Pos: pos, Field: f.Name, Err: ErrTooManyClauses}
	}
	return nil
}
