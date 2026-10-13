package parser

import "fmt"

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

type token struct {
	kind tokenKind
	val  string
	pos  int // byte offset, for error messages
}

func lex(s string) ([]token, error) {
	var toks []token
	i := 0
	for i < len(s) {
		c := s[i]
		switch c {
		case ' ', '\t':
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
			for j < len(s) && s[j] != '"' {
				if s[j] == '\\' {
					j++
				} // skip escaped char
				j++
			}
			if j >= len(s) {
				return nil, fmt.Errorf("unclosed quote at %d", i)
			}
			toks = append(toks, token{tokPhrase, s[i+1 : j], i})
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
				for k < len(s) && s[k] == ' ' {
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

func isSpecial(c byte) bool {
	switch c {
	case ' ', '\t', '(', ')', '"', '^', '+', '-', ':':
		return true
	}
	return false
}
