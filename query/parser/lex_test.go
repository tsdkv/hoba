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
			{tokPhrase, `say \"hi\"`, 0},
			{kind: tokEOF, pos: 12},
		},
		"foo^2.5": {
			{tokTerm, "foo", 0},
			{tokBoost, "2.5", 3},
			{kind: tokEOF, pos: 7},
		},
		"bo*": {
			{tokTerm, "bo*", 0},
			{kind: tokEOF, pos: 3},
		},
		"NOTa": {
			{tokTerm, "NOTa", 0},
			{kind: tokEOF, pos: 4},
		},
		":": {
			{tokTerm, ":", 0},
			{kind: tokEOF, pos: 1},
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

func TestLexUnclosedQuote(t *testing.T) {
	if _, err := lex(`"unclosed`); err == nil {
		t.Fatal("lex() expected error for unclosed quote, got nil")
	}
}
