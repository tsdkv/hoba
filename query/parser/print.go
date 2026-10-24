package parser

import (
	"strconv"
	"strings"
)

// The String methods render one canonical form per tree; the fuzz test
// relies on Parse(Parse(s).String()).String() == Parse(s).String().

// String renders the term as [field:]text with all special runes escaped.
func (t *Term) String() string {
	var b strings.Builder
	if t.Field != "" {
		escapeWord(&b, t.Field)
		b.WriteByte(':')
	}
	escapeWord(&b, t.Text)
	return b.String()
}

// String renders the phrase as [field:]"text", escaping \ and ".
func (p *Phrase) String() string {
	var b strings.Builder
	if p.Field != "" {
		escapeWord(&b, p.Field)
		b.WriteByte(':')
	}
	b.WriteByte('"')
	for _, r := range p.Text {
		if r == '\\' || r == '"' {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	b.WriteByte('"')
	return b.String()
}

// String renders the boost as child^factor; a Clause or Boost child is
// parenthesized so the boost keeps binding to the whole child.
func (b *Boost) String() string {
	var sb strings.Builder
	switch b.Node.(type) {
	case *Clause, *Boost:
		sb.WriteByte('(')
		sb.WriteString(b.Node.String())
		sb.WriteByte(')')
	default:
		sb.WriteString(b.Node.String())
	}
	sb.WriteByte('^')
	sb.WriteString(strconv.FormatFloat(b.Factor, 'f', -1, 64))
	return sb.String()
}

// String renders the clause. The joiner encodes the occur the children
// share: " " when any child is undecided, else " OR " when any is Should,
// else " AND ". A child whose occur differs from the joiner's gets an
// explicit + or - prefix.
func (c *Clause) String() string {
	switch len(c.Children) {
	case 0:
		return ""
	case 1:
		ch := c.Children[0]
		return occurPrefix(ch.Occur) + childString(ch.Node)
	}
	joiner, implied := " AND ", OccurMust
	for _, ch := range c.Children {
		switch ch.Occur {
		case OccurNone:
			joiner, implied = " ", OccurNone
		case OccurShould:
			if joiner != " " {
				joiner, implied = " OR ", OccurShould
			}
		}
	}
	parts := make([]string, len(c.Children))
	for i, ch := range c.Children {
		prefix := ""
		if ch.Occur != implied {
			prefix = occurPrefix(ch.Occur)
		}
		parts[i] = prefix + childString(ch.Node)
	}
	return strings.Join(parts, joiner)
}

// occurPrefix is the explicit marker for occurs the joiner cannot express.
// OccurNone and OccurShould have none: they are the joiner's default.
func occurPrefix(o Occur) string {
	switch o {
	case OccurMust:
		return "+"
	case OccurMustNot:
		return "-"
	}
	return ""
}

// childString parenthesizes nested clauses; the top level never gets them.
func childString(n Node) string {
	if c, ok := n.(*Clause); ok {
		return "(" + c.String() + ")"
	}
	return n.String()
}

// escapeWord renders term text or a field name so it lexes back as one
// word: every rune the lexer treats specially gets a backslash, plus a
// leading + or - (an operator at token start). A word equal to a keyword
// gets a leading backslash so it never re-lexes as AND, OR or NOT.
func escapeWord(b *strings.Builder, s string) {
	switch s {
	case "AND", "OR", "NOT":
		b.WriteByte('\\')
	}
	for i, r := range s {
		if r == '\\' || i == 0 && (r == '+' || r == '-') || isWordEnd(r) {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
}
