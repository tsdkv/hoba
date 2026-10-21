package parser

import (
	"errors"
	"reflect"
	"testing"
)

func term(field, text string) *Term     { return &Term{Field: field, Text: text} }
func phrase(field, text string) *Phrase { return &Phrase{Field: field, Text: text} }
func on(occur Occur, n Node) OccurNode  { return OccurNode{Occur: occur, Node: n} }
func clause(children ...OccurNode) *Clause {
	return &Clause{Children: children}
}

func TestParse(t *testing.T) {
	tests := map[string]Node{
		"foo": term("", "foo"),

		"foo bar": clause(
			on(OccurNone, term("", "foo")),
			on(OccurNone, term("", "bar")),
		),

		"a AND b": clause(
			on(OccurMust, term("", "a")),
			on(OccurMust, term("", "b")),
		),

		"a OR b AND c": clause(
			on(OccurNone, term("", "a")),
			on(OccurShould, clause(
				on(OccurMust, term("", "b")),
				on(OccurMust, term("", "c")),
			)),
		),

		"+a -b": clause(
			on(OccurMust, term("", "a")),
			on(OccurMustNot, term("", "b")),
		),

		"a NOT b": clause(
			on(OccurNone, term("", "a")),
			on(OccurMustNot, term("", "b")),
		),

		"a AND NOT b": clause(
			on(OccurMust, term("", "a")),
			on(OccurMustNot, term("", "b")),
		),

		// The negation heads the AND group: it applies to a only,
		// and the group itself is required.
		"-a AND b": clause(
			on(OccurMust, clause(
				on(OccurMustNot, term("", "a")),
				on(OccurMust, term("", "b")),
			)),
		),

		// Same, but the group followed an OR, so it stays optional.
		"x OR -a AND b": clause(
			on(OccurNone, term("", "x")),
			on(OccurShould, clause(
				on(OccurMustNot, term("", "a")),
				on(OccurMust, term("", "b")),
			)),
		),

		// AND must not merge into a user-written group.
		"(a b) AND c": clause(
			on(OccurMust, clause(
				on(OccurNone, term("", "a")),
				on(OccurNone, term("", "b")),
			)),
			on(OccurMust, term("", "c")),
		),

		"NOT foo": clause(
			on(OccurMustNot, term("", "foo")),
		),

		"title:foo": term("title", "foo"),

		"title:(a b:c)": clause(
			on(OccurNone, term("title", "a")),
			on(OccurNone, term("b", "c")),
		),

		"field:(+a b)": clause(
			on(OccurMust, term("field", "a")),
			on(OccurNone, term("field", "b")),
		),

		`"quick fox"^2`: &Boost{Node: phrase("", "quick fox"), Factor: 2},

		`"say \"hi\""`: phrase("", `say "hi"`),

		"foo\nbar": clause(
			on(OccurNone, term("", "foo")),
			on(OccurNone, term("", "bar")),
		),

		"title\t:x": term("title", "x"),

		"a^1.0": term("", "a"),

		"(a OR b) c": clause(
			on(OccurNone, clause(
				on(OccurNone, term("", "a")),
				on(OccurShould, term("", "b")),
			)),
			on(OccurNone, term("", "c")),
		),

		"(a b)^2": &Boost{
			Node: clause(
				on(OccurNone, term("", "a")),
				on(OccurNone, term("", "b")),
			),
			Factor: 2,
		},
	}

	for input, want := range tests {
		t.Run(input, func(t *testing.T) {
			got, err := Parse(input)
			if err != nil {
				t.Fatalf("Parse(%q) returned error: %v", input, err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("Parse(%q)\n got: %#v\nwant: %#v", input, got, want)
			}
		})
	}
}

func TestParseErrors(t *testing.T) {
	for _, input := range []string{"", "foo AND", "(foo", "foo)", "NOT", "NOT NOT foo", "foo^", "a b)"} {
		t.Run(input, func(t *testing.T) {
			if _, err := Parse(input); err == nil {
				t.Fatalf("Parse(%q) expected error, got nil", input)
			}
		})
	}
}

func TestParseErrorPosition(t *testing.T) {
	_, err := Parse("foo^")
	var pe *Error
	if !errors.As(err, &pe) {
		t.Fatalf(`Parse("foo^") error type = %T, want *Error`, err)
	}
	if pe.Pos != 3 {
		t.Errorf("Pos = %d, want 3 (the '^')", pe.Pos)
	}
}

func FuzzParse(f *testing.F) {
	for _, s := range []string{
		"foo", "a AND b OR c", `title:"quick fox"^2`, "-a AND (b OR NOT c)",
		`"unclosed`, "foo^", "NOT NOT x", "()", "a b)",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		_, _ = Parse(s) // must not panic
	})
}
