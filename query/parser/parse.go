package parser

import (
	"fmt"
	"strconv"
)

// Error describes a query syntax problem. Pos is the byte offset into the
// query string where the problem was found.
type Error struct {
	Pos int
	Msg string
	Err error
}

func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s at byte %d: %v", e.Msg, e.Pos, e.Err)
	}
	return fmt.Sprintf("%s at byte %d", e.Msg, e.Pos)
}

func (e *Error) Unwrap() error { return e.Err }

// maxDepth bounds nested parentheses so hostile input cannot overflow the
// stack through deep recursion.
const maxDepth = 64

// Parse parses a Lucene-like query string into an AST.
//
// Grammar:
//
//	query   := orExpr+                    juxtaposition → OccurNone
//	orExpr  := andExpr ( OR andExpr )*    → OccurShould
//	andExpr := unary ( AND unary )*       → OccurMust
//	unary   := [ + | - | NOT ] boosted    an explicit occur always wins
//	boosted := primary [ ^NUMBER ]
//	primary := [ FIELD: ] ( TERM | PHRASE | "(" query ")" )
//
// AND binds tighter than OR, OR tighter than whitespace. The AND, OR and
// NOT keywords are uppercase-only; their lowercase forms are ordinary
// terms. Whitespace-separated leaves keep OccurNone: deciding whether that
// means AND or OR is left to the schema stage. A field prefix on a
// parenthesised clause applies to every leaf inside that has no field of
// its own. A "^1.0" boost is a no-op and is dropped. An empty or
// whitespace-only query is valid and returns an empty *Clause.
func Parse(s string) (Node, error) {
	toks, err := lex(s)
	if err != nil {
		return nil, err
	}

	p := &parser{toks: toks}
	n, err := p.parseQuery("")
	if err != nil {
		return nil, err
	}

	if t := p.peek(); t.kind != tokEOF {
		return nil, &Error{Pos: t.pos, Msg: "unexpected " + t.String()}
	}

	return n, nil
}

type parser struct {
	toks  []token
	pos   int
	depth int
}

// peek returns the current token. Past the end it keeps returning the
// final EOF token, so a miscounted caller can never index out of range.
func (p *parser) peek() token {
	if p.pos >= len(p.toks) {
		return p.toks[len(p.toks)-1]
	}
	return p.toks[p.pos]
}

func (p *parser) next() token {
	t := p.peek()
	if p.pos < len(p.toks) {
		p.pos++
	}
	return t
}

// operand is one parsed unit at a precedence level: the node plus the
// explicit occur a +, - or NOT prefix gave it (OccurNone when undecided).
type operand struct {
	pos   int // start of the operand, including any + / - / NOT prefix
	occur Occur
	node  Node
}

// parseQuery parses juxtaposed operands, the loosest level, whose undecided
// occur is OccurNone. It is where a query must stand alone — the top level
// and inside parentheses — so a single operand with an explicit occur is
// kept in a one-child Clause instead of losing its prefix.
func (p *parser) parseQuery(field string) (Node, error) {
	if !startsOperand(p.peek()) {
		// an empty query or group is valid and matches nothing
		return &Clause{Start: p.peek().pos}, nil
	}
	op, err := p.level(p.parseOr, tokEOF, OccurNone, field)
	if err != nil {
		return nil, err
	}
	if op.occur == OccurNone {
		return op.node, nil
	}
	return &Clause{Start: op.pos, Children: []OccurNode{{Occur: op.occur, Node: op.node}}}, nil
}

// parseOr parses andExpr operands joined by OR.
func (p *parser) parseOr(field string) (operand, error) {
	return p.level(p.parseAnd, tokOr, OccurShould, field)
}

// parseAnd parses unary operands joined by AND.
func (p *parser) parseAnd(field string) (operand, error) {
	return p.level(p.parseUnary, tokAnd, OccurMust, field)
}

// level parses one or more sub-operands joined by sep (tokEOF means
// juxtaposition). A single operand passes up unchanged, so its occur
// survives until it reaches a level with siblings. Several operands build a
// Clause in which each child without an explicit occur gets the level's
// occur.
func (p *parser) level(sub func(string) (operand, error), sep tokenKind, occur Occur, field string) (operand, error) {
	first, err := sub(field)
	if err != nil {
		return operand{}, err
	}
	ops := []operand{first}
	for p.separated(sep) {
		op, err := sub(field)
		if err != nil {
			return operand{}, err
		}
		ops = append(ops, op)
	}
	if len(ops) == 1 {
		return ops[0], nil
	}
	children := make([]OccurNode, len(ops))
	for i, op := range ops {
		o := op.occur
		if o == OccurNone {
			o = occur
		}
		children[i] = OccurNode{Occur: o, Node: op.node}
	}
	return operand{pos: ops[0].pos, node: &Clause{Start: ops[0].pos, Children: children}}, nil
}

// separated reports (consuming sep if explicit) whether another operand
// follows at this level.
func (p *parser) separated(sep tokenKind) bool {
	if sep == tokEOF {
		return startsOperand(p.peek())
	}
	if p.peek().kind == sep {
		p.next()
		return true
	}
	return false
}

func startsOperand(t token) bool {
	switch t.kind {
	case tokTerm, tokPhrase, tokField, tokLParen, tokNot, tokPlus, tokMinus:
		return true
	}
	return false
}

// parseUnary parses an optional + / - / NOT prefix; an explicit occur
// always wins over any level default.
func (p *parser) parseUnary(field string) (operand, error) {
	pos := p.peek().pos
	occur := OccurNone
	switch p.peek().kind {
	case tokPlus:
		p.next()
		occur = OccurMust
	case tokMinus, tokNot:
		p.next()
		occur = OccurMustNot
	}
	n, err := p.parseBoosted(field)
	if err != nil {
		return operand{}, err
	}
	return operand{pos: pos, occur: occur, node: n}, nil
}

// parseBoosted parses a primary with an optional ^NUMBER boost.
func (p *parser) parseBoosted(field string) (Node, error) {
	n, err := p.parsePrimary(field)
	if err != nil {
		return nil, err
	}
	if p.peek().kind == tokBoost {
		t := p.next()
		f, err := strconv.ParseFloat(t.val, 64)
		if err != nil {
			return nil, &Error{Pos: t.pos, Msg: fmt.Sprintf("invalid boost %q", t.val), Err: err}
		}
		if f != 1 { // a ^1 boost is a no-op
			n = &Boost{Node: n, Factor: f}
		}
	}
	return n, nil
}

// parsePrimary parses [FIELD:] (TERM | PHRASE | group). An explicit field
// prefix overrides the group field passed down from an enclosing
// parenthesised clause.
func (p *parser) parsePrimary(field string) (Node, error) {
	start := p.peek().pos
	if p.peek().kind == tokField {
		field = p.next().val
	}
	t := p.next()
	switch t.kind {
	case tokTerm:
		return &Term{Start: start, Field: field, Text: t.val}, nil
	case tokPhrase:
		return &Phrase{Start: start, Field: field, Text: t.val}, nil
	case tokLParen:
		if p.depth >= maxDepth {
			return nil, &Error{Pos: t.pos, Msg: "parentheses nested too deeply"}
		}
		p.depth++
		defer func() { p.depth-- }()
		sub, err := p.parseQuery(field)
		if err != nil {
			return nil, err
		}
		if p.peek().kind != tokRParen {
			return nil, &Error{Pos: p.peek().pos, Msg: "expected ')'"}
		}
		p.next()
		return sub, nil
	default:
		return nil, &Error{Pos: t.pos, Msg: "unexpected " + t.String()}
	}
}
