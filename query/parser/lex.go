package parser

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

type tokenKind int

const (
	tokEOF tokenKind = iota
	tokTerm
	tokPhrase
	tokField
	tokLParen
	tokRParen
	tokAnd
	tokOr
	tokNot
	tokPlus
	tokMinus
	tokBoost
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
	val  string // unescaped text for term/phrase/field, digits for boost
	pos  int
}

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
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			return nil, &Error{Pos: i, Msg: "invalid UTF-8"}
		}
		switch {
		case unicode.IsSpace(r):
			i += size
		case r == '(':
			toks = append(toks, token{tokLParen, "", i})
			i++
		case r == ')':
			toks = append(toks, token{tokRParen, "", i})
			i++
		case r == '+':
			toks = append(toks, token{tokPlus, "", i})
			i++
		case r == '-':
			toks = append(toks, token{tokMinus, "", i})
			i++
		case r == '"':
			t, next, err := lexPhrase(s, i)
			if err != nil {
				return nil, err
			}
			toks = append(toks, t)
			i = next
		case r == '^':
			t, next, err := lexBoost(s, i)
			if err != nil {
				return nil, err
			}
			toks = append(toks, t)
			i = next
		case r == ':':
			return nil, &Error{Pos: i, Msg: "stray ':'"}
		case isReserved(r):
			return nil, &Error{Pos: i, Msg: fmt.Sprintf("reserved character %q: escape it with a backslash", r)}
		default:
			t, next, err := lexWord(s, i)
			if err != nil {
				return nil, err
			}
			toks = append(toks, t)
			i = next
		}
	}
	return append(toks, token{kind: tokEOF, pos: len(s)}), nil
}

// Runes set aside for future query features (fuzzy, wildcard, ranges).
func isReserved(r rune) bool {
	switch r {
	case '~', '*', '[', ']', '{', '}':
		return true
	}
	return false
}

// + and - are literal inside a word (c++, e-mail, 2024-01-01).
func isWordEnd(r rune) bool {
	return unicode.IsSpace(r) || r == '(' || r == ')' || r == '"' || r == '^' || r == ':' || isReserved(r)
}

// A word built with escapes never lexes as an AND/OR/NOT keyword.
func lexWord(s string, i int) (token, int, error) {
	var b strings.Builder
	esc := false
	j := i
	for j < len(s) {
		r, size := utf8.DecodeRuneInString(s[j:])
		if r == utf8.RuneError && size == 1 {
			return token{}, 0, &Error{Pos: j, Msg: "invalid UTF-8"}
		}
		if isWordEnd(r) {
			break
		}
		if r == '\\' {
			r2, size2, err := escapedRune(s, j, size)
			if err != nil {
				return token{}, 0, err
			}
			b.WriteString(r2)
			esc = true
			j += size + size2
			continue
		}
		b.WriteString(s[j : j+size])
		j += size
	}
	word := b.String()
	if !esc {
		switch word {
		case "AND":
			return token{tokAnd, "", i}, j, nil
		case "OR":
			return token{tokOr, "", i}, j, nil
		case "NOT":
			return token{tokNot, "", i}, j, nil
		}
	}
	k := j
	for k < len(s) {
		r, size := utf8.DecodeRuneInString(s[k:])
		if !unicode.IsSpace(r) {
			break
		}
		k += size
	}
	if k < len(s) && s[k] == ':' {
		return token{tokField, word, i}, k + 1, nil
	}
	return token{tokTerm, word, i}, j, nil
}

func lexPhrase(s string, i int) (token, int, error) {
	var b strings.Builder
	j := i + 1
	for j < len(s) {
		r, size := utf8.DecodeRuneInString(s[j:])
		if r == utf8.RuneError && size == 1 {
			return token{}, 0, &Error{Pos: j, Msg: "invalid UTF-8"}
		}
		switch r {
		case '\\':
			r2, size2, err := escapedRune(s, j, size)
			if err != nil {
				return token{}, 0, &Error{Pos: i, Msg: "unclosed quote"}
			}
			b.WriteString(r2)
			j += size + size2
		case '"':
			return token{tokPhrase, b.String(), i}, j + 1, nil
		default:
			b.WriteString(s[j : j+size])
			j += size
		}
	}
	return token{}, 0, &Error{Pos: i, Msg: "unclosed quote"}
}

// foo^2bar is an error, never foo^2 followed by bar.
func lexBoost(s string, i int) (token, int, error) {
	j := i + 1
	digits, dots := 0, 0
	for j < len(s) {
		c := s[j]
		if c != '.' && (c < '0' || c > '9') {
			break
		}
		if c == '.' {
			dots++
		} else {
			digits++
		}
		j++
	}
	num := s[i+1 : j]
	if digits == 0 || dots > 1 {
		return token{}, 0, &Error{Pos: i, Msg: fmt.Sprintf("invalid boost %q", s[i:j])}
	}
	if j < len(s) {
		r, size := utf8.DecodeRuneInString(s[j:])
		if !(r == utf8.RuneError && size == 1) && !isWordEnd(r) && r != '+' && r != '-' {
			return token{}, 0, &Error{Pos: i, Msg: fmt.Sprintf("boost %q runs into a word character", num)}
		}
	}
	return token{tokBoost, num, i}, j, nil
}

func escapedRune(s string, i, size int) (string, int, error) {
	if i+size >= len(s) {
		return "", 0, &Error{Pos: i, Msg: "trailing backslash"}
	}
	r, size2 := utf8.DecodeRuneInString(s[i+size:])
	if r == utf8.RuneError && size2 == 1 {
		return "", 0, &Error{Pos: i + size, Msg: "invalid UTF-8"}
	}
	return s[i+size : i+size+size2], size2, nil
}
