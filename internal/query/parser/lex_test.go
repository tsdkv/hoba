package parser

import (
	"reflect"
	"testing"
)

func TestLex(t *testing.T) {
	tests := map[string][]token{
		"a AND b OR NOT c": {
			{tokTerm, "a", 0},
			{tokAnd, "", 2},
			{tokTerm, "b", 6},
			{tokOr, "", 8},
			{tokNot, "", 11},
			{tokTerm, "c", 15},
			{kind: tokEOF, pos: 16},
		},
		"-foo bar": {
			{tokMinus, "", 0},
			{tokTerm, "foo", 1},
			{tokTerm, "bar", 5},
			{kind: tokEOF, pos: 8},
		},
		"title:hello": {
			{tokField, "title", 0},
			{tokTerm, "hello", 6},
			{kind: tokEOF, pos: 11},
		},
		"title : hello": {
			{tokField, "title", 0},
			{tokTerm, "hello", 8},
			{kind: tokEOF, pos: 13},
		},
		"title:(foo bar)": {
			{tokField, "title", 0},
			{tokLParen, "", 6},
			{tokTerm, "foo", 7},
			{tokTerm, "bar", 11},
			{tokRParen, "", 14},
			{kind: tokEOF, pos: 15},
		},
		`"quick fox"`: {
			{tokPhrase, "quick fox", 0},
			{kind: tokEOF, pos: 11},
		},
		`"say \"hi\""`: {
			{tokPhrase, `say "hi"`, 0},
			{kind: tokEOF, pos: 12},
		},
		"foo\nbar": {
			{tokTerm, "foo", 0},
			{tokTerm, "bar", 4},
			{kind: tokEOF, pos: 7},
		},
		"a　b": { // U+3000 ideographic space separates
			{tokTerm, "a", 0},
			{tokTerm, "b", 4},
			{kind: tokEOF, pos: 5},
		},
		"title\t:x": {
			{tokField, "title", 0},
			{tokTerm, "x", 7},
			{kind: tokEOF, pos: 8},
		},
		"foo^2.5": {
			{tokTerm, "foo", 0},
			{tokBoost, "2.5", 3},
			{kind: tokEOF, pos: 7},
		},
		"NOTa": {
			{tokTerm, "NOTa", 0},
			{kind: tokEOF, pos: 4},
		},
		// + and - are literal inside a word
		"c++": {
			{tokTerm, "c++", 0},
			{kind: tokEOF, pos: 3},
		},
		"2024-01-01": {
			{tokTerm, "2024-01-01", 0},
			{kind: tokEOF, pos: 10},
		},
		// escapes produce the unescaped word; an escaped word is never a keyword
		`foo\:bar`: {
			{tokTerm, "foo:bar", 0},
			{kind: tokEOF, pos: 8},
		},
		`\AND`: {
			{tokTerm, "AND", 0},
			{kind: tokEOF, pos: 4},
		},
		`a\ b`: {
			{tokTerm, "a b", 0},
			{kind: tokEOF, pos: 4},
		},
		"": {
			{kind: tokEOF, pos: 0},
		},
	}

	for input, want := range tests {
		t.Run(input, func(t *testing.T) {
			got, err := lex(input)
			if err != nil {
				t.Fatalf("lex(%q) returned error: %v", input, err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("lex(%q)\n got: %+v\nwant: %+v", input, got, want)
			}
		})
	}
}

func TestLexErrors(t *testing.T) {
	tests := map[string]int{ // input -> error position
		`"unclosed`:  0,
		`"trailing\`: 0,
		"a\xffb":     1,
		"bo*":        2,
		"a~2":        1,
		"{x}":        0,
		":":          0,
		`a\`:         1,
		"foo^":       3,
		"foo^1.2.3":  3,
		"foo^2bar":   3,
	}
	for input, pos := range tests {
		t.Run(input, func(t *testing.T) {
			_, err := lex(input)
			if err == nil {
				t.Fatalf("lex(%q) expected error, got nil", input)
			}
			pe, ok := err.(*Error)
			if !ok {
				t.Fatalf("lex(%q) error type = %T, want *Error", input, err)
			}
			if pe.Pos != pos {
				t.Errorf("lex(%q) error Pos = %d, want %d", input, pe.Pos, pos)
			}
		})
	}
}
