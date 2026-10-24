package query

import (
	"strconv"
	"strings"
)

// Occur is the decided role of a child inside a Boolean query.
// Unlike parser.Occur there is no None: resolution is complete here.
type Occur int

const (
	// OccurShould marks an optional child that only contributes to scoring.
	OccurShould Occur = iota
	// OccurMust marks a required child.
	OccurMust
	// OccurMustNot marks an excluded child.
	OccurMustNot
)

// Node is a resolved query node: *Term, *Phrase, *Boost, *Boolean or
// *MatchAll.
type Node interface {
	// String renders a debug form, used by tests.
	String() string
	node()
}

// Term is a single analyzed term against one field.
type Term struct {
	FieldID int
	Text    string
}

// Phrase is an ordered sequence of analyzed terms against one field.
// Positions[i] is relative to Terms[0]'s position, so Positions[0] == 0;
// a gap (a token the analyzer dropped) shows as a jump.
type Phrase struct {
	FieldID   int
	Terms     []string
	Positions []int
}

// Boost multiplies its child's score by Factor.
type Boost struct {
	Node   Node
	Factor float64
}

// Boolean is a boolean combination of resolved children. No children means
// matches nothing.
type Boolean struct {
	Children []OccurNode
}

// MatchAll matches every document; resolution adds it as the Must child of
// an all-negative Boolean.
type MatchAll struct{}

// OccurNode pairs a child with its role in the parent Boolean.
type OccurNode struct {
	Occur Occur
	Node  Node
}

func (*Term) node()     {}
func (*Phrase) node()   {}
func (*Boost) node()    {}
func (*Boolean) node()  {}
func (*MatchAll) node() {}

// String renders #<fieldID>:<text>.
func (t *Term) String() string {
	var b strings.Builder
	b.WriteByte('#')
	b.WriteString(strconv.Itoa(t.FieldID))
	b.WriteByte(':')
	for _, r := range t.Text {
		if r == ' ' || r == '"' || r == '(' || r == ')' || r == '\\' {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// String renders #<fieldID>:"t0 t1 ..." with a ? for each position gap.
func (p *Phrase) String() string {
	var b strings.Builder
	b.WriteByte('#')
	b.WriteString(strconv.Itoa(p.FieldID))
	b.WriteByte(':')
	b.WriteByte('"')
	for i, term := range p.Terms {
		if i > 0 {
			b.WriteByte(' ')
			gap := 1
			if i < len(p.Positions) {
				gap = p.Positions[i] - p.Positions[i-1]
			}
			for g := 1; g < gap; g++ {
				b.WriteString("? ")
			}
		}
		for _, r := range term {
			if r == '"' || r == '\\' {
				b.WriteByte('\\')
			}
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// String renders child^factor.
func (b *Boost) String() string {
	return b.Node.String() + "^" + strconv.FormatFloat(b.Factor, 'f', -1, 64)
}

// String renders (+a -b c): Must gets +, MustNot gets -, Should is bare.
func (b *Boolean) String() string {
	var sb strings.Builder
	sb.WriteByte('(')
	for i, ch := range b.Children {
		if i > 0 {
			sb.WriteByte(' ')
		}
		switch ch.Occur {
		case OccurMust:
			sb.WriteByte('+')
		case OccurMustNot:
			sb.WriteByte('-')
		}
		sb.WriteString(ch.Node.String())
	}
	sb.WriteByte(')')
	return sb.String()
}

// String renders *.
func (*MatchAll) String() string { return "*" }
