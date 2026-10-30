package query

import (
	"strconv"
	"strings"
)

type Occur int

const (
	OccurShould Occur = iota
	OccurMust
	OccurMustNot
)

type Node interface {
	String() string
	node()
}

type Term struct {
	FieldID int
	Text    string
}

// Positions are relative to the first term, so dropped tokens show as jumps.
type Phrase struct {
	FieldID   int
	Terms     []string
	Positions []int
}

type Boost struct {
	Node   Node
	Factor float64
}

type Boolean struct {
	Children []OccurNode
}

type MatchAll struct{}

type OccurNode struct {
	Occur Occur
	Node  Node
}

func (*Term) node()     {}
func (*Phrase) node()   {}
func (*Boost) node()    {}
func (*Boolean) node()  {}
func (*MatchAll) node() {}

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

func (b *Boost) String() string {
	return b.Node.String() + "^" + strconv.FormatFloat(b.Factor, 'f', -1, 64)
}

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

func (*MatchAll) String() string { return "*" }
