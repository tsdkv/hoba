package parser

import (
	"errors"
	"strings"
	"testing"
)

// TestParseString runs each input through Parse and compares the canonical
// printed form; the canonical form must re-parse to the same tree.
func TestParseString(t *testing.T) {
	tests := []struct{ in, want string }{
		{"", ""},
		{"  ", ""},
		{"()", ""},
		{"foo", "foo"},

		// precedence: AND > OR > whitespace
		{"a b", "a b"},
		{"a AND b", "a AND b"},
		{"a OR b", "a OR b"},
		{"a OR b AND c", "a OR (b AND c)"},
		{"a AND b OR c", "(a AND b) OR c"},
		{"a b OR c", "a (b OR c)"},

		{"+a -b", "a AND -b"},
		{"a NOT b", "a -b"},
		{"a AND NOT b", "a AND -b"},
		{"NOT foo", "-foo"},
		{"+foo", "+foo"},
		{"-a AND b", "-a AND b"},
		{"x OR -a AND b", "x OR (-a AND b)"},
		{"a OR +b", "a OR +b"},

		{"(a b) AND c", "(a b) AND c"},
		{"(a OR b) c", "(a OR b) c"},
		{"-(a b)", "-(a b)"},

		{"title:foo", "title:foo"},
		{"title : hello", "title:hello"},
		{"title\t:x", "title:x"},
		{"title:(a b:c)", "title:a b:c"},
		{"field:(+a b)", "+field:a field:b"},

		{`"quick fox"`, `"quick fox"`},
		{`"say \"hi\""`, `"say \"hi\""`},
		{`""`, `""`},

		{`"quick fox"^2`, `"quick fox"^2`},
		{"(a b)^2", "(a b)^2"},
		{"(a OR b)^2 AND c", "(a OR b)^2 AND c"},
		{"a^1.0", "a"},
		{"a^2.5", "a^2.5"},
		{"a^.5", "a^0.5"},
		{"(a^2)^3", "(a^2)^3"},

		// escapes and literal + / - inside words
		{`foo\:bar`, `foo\:bar`},
		{`\-1`, `\-1`},
		{`\AND`, `\AND`},
		{`\OR`, `\OR`},
		{`a\ b`, `a\ b`},
		{`a\*b`, `a\*b`},
		{"c++", "c++"},
		{"e-mail", "e-mail"},
		{"2024-01-01", "2024-01-01"},

		// unicode whitespace separates
		{"a　b", "a b"},
		{"a\nb", "a b"},
	}

	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			n, err := Parse(tc.in)
			if err != nil {
				t.Fatalf("Parse(%q) returned error: %v", tc.in, err)
			}
			if got := n.String(); got != tc.want {
				t.Errorf("Parse(%q).String() = %q, want %q", tc.in, got, tc.want)
			}
			n2, err := Parse(tc.want)
			if err != nil {
				t.Fatalf("canonical form %q fails to parse: %v", tc.want, err)
			}
			if got := n2.String(); got != tc.want {
				t.Errorf("re-parsed %q prints %q", tc.want, got)
			}
		})
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		in  string
		pos int
	}{
		{`"unclosed`, 0},
		{`"trailing\`, 0},
		{"foo^", 3},
		{"foo^abc", 3},
		{"foo^1.2.3", 3},
		{"foo^2bar", 3},
		{"bo*", 2},
		{"a~2", 1},
		{"[a]", 0},
		{":foo", 0},
		{`a\`, 1},
		{"a\xffb", 1},
		{"foo)", 3},
		{"a b)", 3},
		{"(foo", 4},
		{"a AND", 5},
		{"a OR", 4},
		{"AND a", 0},
		{"NOT NOT foo", 4},
		{"title:", 6},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			_, err := Parse(tc.in)
			if err == nil {
				t.Fatalf("Parse(%q) expected error, got nil", tc.in)
			}
			var pe *Error
			if !errors.As(err, &pe) {
				t.Fatalf("Parse(%q) error type = %T, want *Error", tc.in, err)
			}
			if pe.Pos != tc.pos {
				t.Errorf("Parse(%q) error Pos = %d, want %d", tc.in, pe.Pos, tc.pos)
			}
		})
	}
}

func TestParseNestingDepth(t *testing.T) {
	deep := strings.Repeat("(", maxDepth) + "x" + strings.Repeat(")", maxDepth)
	if _, err := Parse(deep); err != nil {
		t.Fatalf("Parse at maxDepth failed: %v", err)
	}
	tooDeep := strings.Repeat("(", maxDepth+1) + "x" + strings.Repeat(")", maxDepth+1)
	_, err := Parse(tooDeep)
	var pe *Error
	if !errors.As(err, &pe) {
		t.Fatalf("Parse beyond maxDepth error = %v, want *Error", err)
	}
	if pe.Pos != maxDepth {
		t.Errorf("depth error Pos = %d, want %d", pe.Pos, maxDepth)
	}
}

func TestNodePositions(t *testing.T) {
	n, err := Parse("+title:(a OR b)^2")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	// "+title:(a OR b)^2": + 0, title 1, : 6, ( 7, a 8, OR 10, b 13, ) 14, ^2 15
	root, ok := n.(*Clause)
	if !ok {
		t.Fatalf("root is %T, want *Clause", n)
	}
	if root.Start != 0 {
		t.Errorf("root.Start = %d, want 0 (the '+')", root.Start)
	}
	if len(root.Children) != 1 || root.Children[0].Occur != OccurMust {
		t.Fatalf("root.Children = %+v, want one Must child", root.Children)
	}
	boost, ok := root.Children[0].Node.(*Boost)
	if !ok {
		t.Fatalf("child is %T, want *Boost", root.Children[0].Node)
	}
	if boost.Pos() != 8 {
		t.Errorf("boost.Pos() = %d, want 8 (the child clause)", boost.Pos())
	}
	inner, ok := boost.Node.(*Clause)
	if !ok {
		t.Fatalf("boost child is %T, want *Clause", boost.Node)
	}
	if inner.Start != 8 {
		t.Errorf("inner.Start = %d, want 8", inner.Start)
	}
	for i, want := range []int{8, 13} {
		term, ok := inner.Children[i].Node.(*Term)
		if !ok {
			t.Fatalf("inner child %d is %T, want *Term", i, inner.Children[i].Node)
		}
		if term.Start != want {
			t.Errorf("term %d Start = %d, want %d", i, term.Start, want)
		}
		if term.Field != "title" {
			t.Errorf("term %d Field = %q, want title (group field)", i, term.Field)
		}
	}
}

func TestTermStartIncludesFieldPrefix(t *testing.T) {
	n, err := Parse("  title:foo")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	term, ok := n.(*Term)
	if !ok {
		t.Fatalf("root is %T, want *Term", n)
	}
	if term.Start != 2 {
		t.Errorf("Start = %d, want 2 (the field prefix)", term.Start)
	}
}

func FuzzParse(f *testing.F) {
	for _, s := range []string{
		"foo", "a AND b OR c", `title:"quick fox"^2`, "-a AND (b OR NOT c)",
		`"unclosed`, "foo^", "NOT NOT x", "()", "a b)", `foo\:bar`, `(a b)^2`,
		"c++", "", "  ", `"a \"b\""`, "~", `\`, "a　b", "title:(a b:c)",
		"+a -b", `(a^2)^3`, "foo^2bar", "e-mail",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		n, err := Parse(s)
		if err != nil {
			return // a syntax error is fine; a panic is not
		}
		s2 := n.String()
		n2, err := Parse(s2)
		if err != nil {
			t.Fatalf("Parse(%q).String() = %q, which fails to parse: %v", s, s2, err)
		}
		if s3 := n2.String(); s3 != s2 {
			t.Fatalf("round trip: Parse(%q).String() = %q, re-parse prints %q", s, s2, s3)
		}
	})
}
