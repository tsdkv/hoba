package parser

import (
	"strconv"
	"strings"
)

// The fuzz test relies on Parse(Parse(s).String()).String() == Parse(s).String().

func (t *Term) String() string {
	var b strings.Builder
	if t.Field != "" {
		escapeWord(&b, t.Field)
		b.WriteByte(':')
	}
	escapeWord(&b, t.Text)
	return b.String()
}

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

// A Clause or Boost child is parenthesized so the boost binds to the whole child.
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

// OccurNone and OccurShould are implied by the joiner, so they get no prefix.
func occurPrefix(o Occur) string {
	switch o {
	case OccurMust:
		return "+"
	case OccurMustNot:
		return "-"
	}
	return ""
}

func childString(n Node) string {
	if c, ok := n.(*Clause); ok {
		return "(" + c.String() + ")"
	}
	return n.String()
}

// A leading + or - and exact keywords get a backslash so the word never re-lexes as syntax.
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
