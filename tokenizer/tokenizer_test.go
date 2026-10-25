package tokenizer

import (
	"reflect"
	"strings"
	"testing"
)

func collect(t Tokenizer, text string) []Token {
	var out []Token
	for tok := range t.Tokens(text) {
		out = append(out, tok)
	}
	return out
}

func TestSimple(t *testing.T) {
	tests := []struct {
		in   string
		want []Token
	}{
		{"hello world", []Token{
			{Text: "hello", Position: 0, Start: 0, End: 5},
			{Text: "world", Position: 1, Start: 6, End: 11},
		}},
		{"foo, bar!", []Token{
			{Text: "foo", Position: 0, Start: 0, End: 3},
			{Text: "bar", Position: 1, Start: 5, End: 8},
		}},
		// é and ö are two bytes each; offsets are byte offsets.
		{"héllo wörld", []Token{
			{Text: "héllo", Position: 0, Start: 0, End: 6},
			{Text: "wörld", Position: 1, Start: 7, End: 13},
		}},
		// combining marks join the base letter into one token
		{"éx", []Token{
			{Text: "éx", Position: 0, Start: 0, End: 4},
		}},
		{"abc123", []Token{
			{Text: "abc123", Position: 0, Start: 0, End: 6},
		}},
		{"12.5", []Token{
			{Text: "12", Position: 0, Start: 0, End: 2},
			{Text: "5", Position: 1, Start: 3, End: 4},
		}},
		{"こんにちは", []Token{
			{Text: "こんにちは", Position: 0, Start: 0, End: 15},
		}},
		// invalid UTF-8 bytes separate
		{"a\xffb", []Token{
			{Text: "a", Position: 0, Start: 0, End: 1},
			{Text: "b", Position: 1, Start: 2, End: 3},
		}},
		{"", nil},
		{"---", nil},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			if got := collect(Simple{}, tc.in); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Simple.Tokens(%q)\n got: %+v\nwant: %+v", tc.in, got, tc.want)
			}
		})
	}
}

func TestKeyword(t *testing.T) {
	tests := []struct {
		in   string
		want []Token
	}{
		{"New York", []Token{{Text: "New York", Position: 0, Start: 0, End: 8}}},
		{"  padded  ", []Token{{Text: "  padded  ", Position: 0, Start: 0, End: 10}}},
		{"", nil},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			if got := collect(Keyword{}, tc.in); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Keyword.Tokens(%q)\n got: %+v\nwant: %+v", tc.in, got, tc.want)
			}
		})
	}
}

func TestLowerCase(t *testing.T) {
	a := NewAnalyzer(Keyword{}, LowerCase)
	want := []Token{{Text: "abc", Position: 0, Start: 0, End: 3}}
	if got := collect(a, "AbC"); !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestRemoveLong(t *testing.T) {
	a := NewAnalyzer(Simple{}, RemoveLong(3))
	// "cccc" is dropped, leaving a gap between positions 1 and 3.
	want := []Token{
		{Text: "a", Position: 0, Start: 0, End: 1},
		{Text: "bb", Position: 1, Start: 2, End: 4},
		{Text: "dd", Position: 3, Start: 10, End: 12},
	}
	if got := collect(a, "a bb cccc dd"); !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestAnalyzerComposition(t *testing.T) {
	a := NewAnalyzer(Simple{}, RemoveLong(40), LowerCase)

	want := []Token{
		{Text: "hello", Position: 0, Start: 0, End: 5},
		{Text: "world", Position: 1, Start: 7, End: 12},
	}
	if got := collect(a, "Hello, WORLD!"); !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}

	long := strings.Repeat("x", 41)
	got := collect(a, "a "+long+" b")
	want = []Token{
		{Text: "a", Position: 0, Start: 0, End: 1},
		{Text: "b", Position: 2, Start: 2 + 41 + 1, End: 2 + 41 + 2},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestManager(t *testing.T) {
	m := NewManager()

	def, ok := m.Get("default")
	if !ok {
		t.Fatal(`Get("default") not found`)
	}
	want := []Token{
		{Text: "foo", Position: 0, Start: 0, End: 3},
		{Text: "bar", Position: 1, Start: 4, End: 7},
	}
	if got := collect(def, "Foo BAR"); !reflect.DeepEqual(got, want) {
		t.Errorf("default analyzer: got %+v, want %+v", got, want)
	}

	kw, ok := m.Get("keyword")
	if !ok {
		t.Fatal(`Get("keyword") not found`)
	}
	want = []Token{{Text: "Foo Bar", Position: 0, Start: 0, End: 7}}
	if got := collect(kw, "Foo Bar"); !reflect.DeepEqual(got, want) {
		t.Errorf("keyword analyzer: got %+v, want %+v", got, want)
	}

	if _, ok := m.Get("nope"); ok {
		t.Error(`Get("nope") found, want not found`)
	}

	custom := NewAnalyzer(Keyword{})
	m.Register("custom", custom)
	got, ok := m.Get("custom")
	if !ok || got != custom {
		t.Errorf("Get(\"custom\") = %v, %v; want the registered analyzer", got, ok)
	}
}
