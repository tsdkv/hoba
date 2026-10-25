package tokenizer

import (
	"iter"
	"strings"
	"unicode"
	"unicode/utf8"
)

type Token struct {
	Text string
	// Dropped tokens leave gaps in the position sequence.
	Position int
	Start    int
	End      int
}

type Tokenizer interface {
	Tokens(text string) iter.Seq[Token]
}

type Simple struct{}

func (Simple) Tokens(text string) iter.Seq[Token] {
	return func(yield func(Token) bool) {
		pos, start := 0, -1
		flush := func(end int) bool {
			if start < 0 {
				return true
			}
			t := Token{Text: text[start:end], Position: pos, Start: start, End: end}
			pos++
			start = -1
			return yield(t)
		}
		for i := 0; i < len(text); {
			r, size := utf8.DecodeRuneInString(text[i:])
			// Invalid bytes decode as RuneError, which separates like punctuation.
			if unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.IsMark(r) {
				if start < 0 {
					start = i
				}
			} else if !flush(i) {
				return
			}
			i += size
		}
		flush(len(text))
	}
}

type Keyword struct{}

func (Keyword) Tokens(text string) iter.Seq[Token] {
	return func(yield func(Token) bool) {
		if text != "" {
			yield(Token{Text: text, Position: 0, Start: 0, End: len(text)})
		}
	}
}

type Filter func(Token) (tok Token, keep bool)

func LowerCase(tok Token) (Token, bool) {
	tok.Text = strings.ToLower(tok.Text)
	return tok, true
}

func RemoveLong(limit int) Filter {
	return func(tok Token) (Token, bool) {
		if len(tok.Text) > limit {
			return Token{}, false
		}
		return tok, true
	}
}

type Analyzer struct {
	tokenizer Tokenizer
	filters   []Filter
}

func NewAnalyzer(t Tokenizer, filters ...Filter) *Analyzer {
	return &Analyzer{tokenizer: t, filters: filters}
}

func (a *Analyzer) Tokens(text string) iter.Seq[Token] {
	return func(yield func(Token) bool) {
		for tok := range a.tokenizer.Tokens(text) {
			keep := true
			for _, f := range a.filters {
				tok, keep = f(tok)
				if !keep {
					break
				}
			}
			if keep && !yield(tok) {
				return
			}
		}
	}
}
