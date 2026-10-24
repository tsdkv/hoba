// Package tokenizer turns text into tokens for indexing and querying.
// Both sides share one analyzer per field (via Manager and
// schema.Field.Tokenizer) so indexed and queried terms never diverge.
package tokenizer

import (
	"iter"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Token is one unit of analyzed text.
type Token struct {
	Text     string // term text, as indexed and as looked up
	Position int    // ordinal assigned by the Tokenizer; dropped tokens leave gaps
	Start    int    // byte offset of the token in the input
	End      int    // byte offset just past the token
}

// Tokenizer splits text into tokens.
type Tokenizer interface {
	Tokens(text string) iter.Seq[Token]
}

// Simple splits text into maximal runs of letter, number and mark runes;
// everything else (including invalid UTF-8 bytes) is a separator.
type Simple struct{}

// Tokens implements Tokenizer.
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
			// utf8.RuneError is neither letter, number nor mark, so invalid
			// bytes (size 1) separate just like punctuation.
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

// Keyword emits the whole input as a single token, case kept.
type Keyword struct{}

// Tokens implements Tokenizer.
func (Keyword) Tokens(text string) iter.Seq[Token] {
	return func(yield func(Token) bool) {
		if text != "" {
			yield(Token{Text: text, Position: 0, Start: 0, End: len(text)})
		}
	}
}

// Filter rewrites one token, or drops it by returning keep=false.
type Filter func(Token) (tok Token, keep bool)

// LowerCase lowercases the token text.
func LowerCase(tok Token) (Token, bool) {
	tok.Text = strings.ToLower(tok.Text)
	return tok, true
}

// RemoveLong returns a Filter that drops tokens longer than limit bytes,
// leaving a gap in the position sequence.
func RemoveLong(limit int) Filter {
	return func(tok Token) (Token, bool) {
		if len(tok.Text) > limit {
			return Token{}, false
		}
		return tok, true
	}
}

// Analyzer is a Tokenizer followed by Filters. It is itself a Tokenizer.
type Analyzer struct {
	tokenizer Tokenizer
	filters   []Filter
}

// NewAnalyzer chains t with the given filters, applied in order.
func NewAnalyzer(t Tokenizer, filters ...Filter) *Analyzer {
	return &Analyzer{tokenizer: t, filters: filters}
}

// Tokens implements Tokenizer.
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
