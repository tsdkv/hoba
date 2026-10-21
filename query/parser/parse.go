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

// Parse parses a Lucene-like query string into an AST.
//
// Grammar sketch:
//
//	clause := leaf ( (AND | OR | whitespace) leaf )*
//	leaf   := [+ | - | NOT] [field ":"] (term | phrase | "(" clause ")") ["^" boost]
//
// The AND, OR and NOT keywords are uppercase-only; their lowercase forms
// are ordinary terms. AND binds tighter than OR and whitespace.
// Whitespace-separated leaves keep OccurNone: deciding whether that means
// AND or OR is left to the schema stage. A field prefix on a parenthesised
// clause applies to every leaf inside that has no field of its own.
// A "^1.0" boost is a no-op and is dropped.
func Parse(s string) (Node, error) {
	toks, err := lex(s)
	if err != nil {
		return nil, err
	}

	p := &parser{toks: toks}
	n, err := p.parseClause()
	if err != nil {
		return nil, err
	}

	if t := p.peek(); t.kind != tokEOF {
		return nil, &Error{Pos: t.pos, Msg: "unexpected " + t.String()}
	}

	return n, nil
}

type parser struct {
	toks []token
	pos  int
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

// binop is how an operand is joined to its left neighbour.
type binop int

const (
	binopImplicit binop = iota // whitespace
	binopAnd
	binopOr
)

type operand struct {
	binop binop
	occur Occur // from a +, - or NOT prefix
	node  Node
}

// parseClause parses a flat list of operands separated by whitespace,
// AND, or OR, then lets aggregate impose boolean structure. Precedence
// never appears in the recursive shape — that is the aggregate pass's job.
func (p *parser) parseClause() (Node, error) {
	first, err := p.parseOccurLeaf()
	if err != nil {
		return nil, err
	}
	ops := []operand{first}

loop:
	for {
		var b binop
		switch p.peek().kind {
		case tokAnd:
			p.next()
			b = binopAnd
		case tokOr:
			p.next()
			b = binopOr
		default:
			if !startsLeaf(p.peek()) {
				break loop
			}
			b = binopImplicit
		}
		op, err := p.parseOccurLeaf()
		if err != nil {
			return nil, err
		}
		op.binop = b
		ops = append(ops, op)
	}

	if len(ops) == 1 && ops[0].binop == binopImplicit && ops[0].occur == OccurNone {
		return ops[0].node, nil
	}
	return aggregate(ops), nil
}

func startsLeaf(t token) bool {
	switch t.kind {
	case tokTerm, tokPhrase, tokField, tokLParen, tokNot, tokPlus, tokMinus:
		return true
	}
	return false
}

func (p *parser) parseOccurLeaf() (operand, error) {
	o := operand{occur: OccurNone}
	switch p.peek().kind {
	case tokPlus:
		p.next()
		o.occur = OccurMust
	case tokMinus, tokNot:
		p.next()
		o.occur = OccurMustNot
	}
	n, err := p.parseLeaf()
	o.node = n
	return o, err
}

func (p *parser) parseLeaf() (Node, error) {
	field := ""
	if p.peek().kind == tokField {
		field = p.next().val
	}

	var n Node
	switch t := p.next(); t.kind {
	case tokTerm:
		n = &Term{Field: field, Text: t.val}
	case tokPhrase:
		n = &Phrase{Field: field, Text: t.val}
	case tokLParen:
		sub, err := p.parseClause()
		if err != nil {
			return nil, err
		}
		if p.peek().kind != tokRParen {
			return nil, &Error{Pos: p.peek().pos, Msg: "expected ')'"}
		}
		p.next()
		if field != "" {
			applyField(sub, field)
		}
		n = sub
	default:
		return nil, &Error{Pos: t.pos, Msg: "unexpected " + t.String()}
	}

	if p.peek().kind == tokBoost {
		t := p.next()
		f, err := strconv.ParseFloat(t.val, 64)
		if err != nil {
			return nil, &Error{Pos: t.pos, Msg: fmt.Sprintf("invalid boost %q", t.val), Err: err}
		}
		if f != 1.0 {
			n = &Boost{Node: n, Factor: f}
		}
	}
	return n, nil
}

// aggregate folds a flat operand list into boolean structure:
// AND merges with its left neighbour into a Must group, OR starts a
// new Should child, implicit whitespace keeps the operand's occur
// (OccurNone stays undecided for the schema stage).
func aggregate(ops []operand) Node {
	clauses := make([]OccurNode, 0, len(ops))
	// lastIsGroup reports whether clauses[last] is an AND group built here,
	// as opposed to a group the user wrote with parentheses. Only the
	// former may absorb further AND operands.
	lastIsGroup := false
	for i, op := range ops {
		switch op.binop {
		case binopAnd:
			occur := op.occur
			if occur == OccurNone {
				occur = OccurMust
			}
			last := &clauses[len(clauses)-1]
			sub, ok := last.Node.(*Clause)
			if !ok || !lastIsGroup {
				// The left neighbour's outer role (e.g. Should from a
				// preceding OR) becomes Must inside the AND group, unless
				// it was an explicit negation.
				firstOccur := last.Occur
				if firstOccur == OccurNone || firstOccur == OccurShould {
					firstOccur = OccurMust
				}
				if last.Occur == OccurMustNot {
					// An explicit negation heads the group ("-a AND b"):
					// it negates the first operand only. The group itself
					// is required, or optional when it followed an OR.
					if i > 0 && ops[i-1].binop == binopOr {
						last.Occur = OccurShould
					} else {
						last.Occur = OccurMust
					}
				}
				sub = &Clause{Children: []OccurNode{{firstOccur, last.Node}}}
				last.Node = sub
			}
			sub.Children = append(sub.Children, OccurNode{occur, op.node})
			lastIsGroup = true
		case binopOr:
			occur := op.occur
			if occur == OccurNone {
				occur = OccurShould
			}
			clauses = append(clauses, OccurNode{occur, op.node})
			lastIsGroup = false
		default:
			clauses = append(clauses, OccurNode{op.occur, op.node})
			lastIsGroup = false
		}
	}
	if len(clauses) == 1 && clauses[0].Occur == OccurNone {
		return clauses[0].Node
	}
	return &Clause{Children: clauses}
}

// applyField stamps a group field (title:(...)) onto every leaf that
// does not already carry one, like tantivy's set_default_field.
func applyField(n Node, field string) {
	switch t := n.(type) {
	case *Term:
		if t.Field == "" {
			t.Field = field
		}
	case *Phrase:
		if t.Field == "" {
			t.Field = field
		}
	case *Boost:
		applyField(t.Node, field)
	case *Clause:
		for _, c := range t.Children {
			applyField(c.Node, field)
		}
	}
}
