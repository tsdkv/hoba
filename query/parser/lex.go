package parser

import (
	"fmt"
	"strings"
)

type tokenKind int

const (
	tokEOF    tokenKind = iota
	tokTerm             // bare word: foo
	tokPhrase           // "foo bar"
	tokField            // "title:"  -> val holds "title"
	tokLParen
	tokRParen
	tokAnd // AND
	tokOr  // OR
	tokNot // NOT
	tokPlus
	tokMinus
	tokBoost // ^1.5 -> val holds "1.5"
)

func (k tokenKind) String() string {
	switch k {
	case tokEOF:
		return "end of input"
	case tokTerm:
		return "term"
	case tokPhrase:
		return "phrase"
	case tokField:
		return "field"
	case tokLParen:
		return "'('"
	case tokRParen:
		return "')'"
	case tokAnd:
		return "AND"
	case tokOr:
		return "OR"
	case tokNot:
		return "NOT"
	case tokPlus:
		return "'+'"
	case tokMinus:
		return "'-'"
	case tokBoost:
		return "boost"
	}
	return fmt.Sprintf("tokenKind(%d)", int(k))
}

type token struct {
	kind tokenKind
	val  string
	pos  int // byte offset, for error messages
}

// String renders the token for error messages, e.g. `term "foo"` or `AND`.
func (t token) String() string {
	if t.val != "" {
		return fmt.Sprintf("%s %q", t.kind, t.val)
	}
	return t.kind.String()
}

func lex(s string) ([]token, error) {
	var toks []token
	i := 0
	for i < len(s) {
		c := s[i]
		switch c {
		case ' ', '\t', '\n', '\r', '\v', '\f':
			i++
		case '(':
			toks = append(toks, token{tokLParen, "", i})
			i++
		case ')':
			toks = append(toks, token{tokRParen, "", i})
			i++
		case '+':
			toks = append(toks, token{tokPlus, "", i})
			i++
		case '-':
			toks = append(toks, token{tokMinus, "", i})
			i++
		case '^':
			j := i + 1
			for j < len(s) && (isDigit(s[j]) || s[j] == '.') {
				j++
			}
			toks = append(toks, token{tokBoost, s[i+1 : j], i})
			i = j
		case '"':
			j := i + 1
			var b strings.Builder
			closed := false
			for j < len(s) {
				if s[j] == '\\' {
					if j+1 >= len(s) {
						break // trailing backslash: unclosed
					}
					b.WriteByte(s[j+1])
					j += 2
					continue
				}
				if s[j] == '"' {
					closed = true
					break
				}
				b.WriteByte(s[j])
				j++
			}
			if !closed {
				return nil, &Error{Pos: i, Msg: "unclosed quote"}
			}
			toks = append(toks, token{tokPhrase, b.String(), i})
			i = j + 1
		default: // a word
			j := i
			for j < len(s) && !isSpecial(s[j]) {
				j++
			}
			if j == i {
				// special char with no token of its own (e.g. stray ':');
				// swallow it into the word to guarantee progress
				j++
			}
			word := s[i:j]
			// keyword or field?
			switch word {
			case "AND":
				toks = append(toks, token{tokAnd, "", i})
			case "OR":
				toks = append(toks, token{tokOr, "", i})
			case "NOT":
				toks = append(toks, token{tokNot, "", i})
			default:
				k := j
				for k < len(s) && isSpace(s[k]) {
					k++
				} // allow "title : foo"
				if k < len(s) && s[k] == ':' {
					toks = append(toks, token{tokField, word, i})
					i = k + 1
					continue
				}
				toks = append(toks, token{tokTerm, word, i})
			}
			i = j
		}
	}
	return append(toks, token{kind: tokEOF, pos: len(s)}), nil
}

func isDigit(c byte) bool {
	return c >= '0' && c <= '9'
}

func isSpace(c byte) bool {
	switch c {
	case ' ', '\t', '\n', '\r', '\v', '\f':
		return true
	}
	return false
}

func isSpecial(c byte) bool {
	switch c {
	case '(', ')', '"', '^', '+', '-', ':':
		return true
	}
	return isSpace(c)
}
