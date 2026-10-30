package query

import (
	"errors"
	"strings"
	"testing"

	"github.com/tsdkv/hoba/internal/query/parser"
	"github.com/tsdkv/hoba/schema"
)

// Field IDs: title=0, body=1, id=2, year=3, meta=4, desc=5.
func testSchema(t *testing.T) *schema.Schema {
	t.Helper()
	s, err := schema.NewSchema(
		schema.TextField("title", schema.Indexed(), schema.Stored(), schema.WithPositions()),
		schema.TextField("body", schema.Indexed(), schema.WithPositions()),
		schema.TextField("id", schema.Keyword(), schema.Indexed()),
		schema.Int64Field("year", schema.Stored()),
		schema.TextField("meta", schema.Stored()),
		schema.TextField("desc", schema.Indexed()), // no positions
	)
	if err != nil {
		t.Fatalf("NewSchema: %v", err)
	}
	return s
}

func newParser(t *testing.T, opts ...Option) *Parser {
	t.Helper()
	p, err := NewParser(testSchema(t), opts...)
	if err != nil {
		t.Fatalf("NewParser: %v", err)
	}
	return p
}

func TestParse(t *testing.T) {
	longToken := strings.Repeat("x", 41) // dropped by RemoveLong(40)
	tests := []struct {
		name  string
		query string
		opts  []Option
		want  string
	}{
		{
			name:  "term on default field is analyzed",
			query: "Foo",
			opts:  []Option{DefaultFields("title")},
			want:  "#0:foo",
		},
		{
			name:  "explicit field skips defaults",
			query: "title:Foo",
			want:  "#0:foo",
		},
		{
			name:  "keyword field keeps case",
			query: "id:ABC",
			want:  "#2:ABC",
		},
		{
			name:  "keyword phrase analyzes to one exact term",
			query: `id:"a b"`,
			want:  `#2:a\ b`,
		},
		{
			name:  "multiple default fields expand to Should",
			query: "Foo",
			opts:  []Option{DefaultFields("title", "body")},
			want:  "(#0:foo #1:foo)",
		},
		{
			name:  "boost wraps the default-field expansion",
			query: "foo^2",
			opts:  []Option{DefaultFields("title", "body")},
			want:  "(#0:foo #1:foo)^2",
		},
		{
			name:  "phrase is tokenized",
			query: `body:"Quick Fox"`,
			want:  `#1:"quick fox"`,
		},
		{
			name:  "single-token phrase collapses to term",
			query: `body:"Word"`,
			want:  "#1:word",
		},
		{
			name:  "dropped token leaves a position gap",
			query: `body:"a ` + longToken + ` b"`,
			want:  `#1:"a ? b"`,
		},
		{
			name:  "whitespace defaults to AND",
			query: "a b",
			opts:  []Option{DefaultFields("title")},
			want:  "(+#0:a +#0:b)",
		},
		{
			name:  "whitespace with DefaultOR",
			query: "a b",
			opts:  []Option{DefaultFields("title"), DefaultOR()},
			want:  "(#0:a #0:b)",
		},
		{
			name:  "explicit OR stays Should regardless of policy",
			query: "a OR b",
			opts:  []Option{DefaultFields("title")},
			want:  "(#0:a #0:b)",
		},
		{
			name:  "explicit occurs survive resolution",
			query: "a OR -b",
			opts:  []Option{DefaultFields("title")},
			want:  "(#0:a -#0:b)",
		},
		{
			name:  "all-negative boolean matches everything except",
			query: "NOT foo",
			opts:  []Option{DefaultFields("title")},
			want:  "(+* -#0:foo)",
		},
		{
			name:  "single Must child is unwrapped",
			query: "+foo",
			opts:  []Option{DefaultFields("title")},
			want:  "#0:foo",
		},
		{
			name:  "nested groups and boost",
			query: "(a OR b)^2 AND c",
			opts:  []Option{DefaultFields("title")},
			want:  "(+(#0:a #0:b)^2 +#0:c)",
		},
		{
			name:  "negated group",
			query: "-(a b)",
			opts:  []Option{DefaultFields("title")},
			want:  "(+* -(+#0:a +#0:b))",
		},
		{
			name:  "group field applies to fieldless leaves",
			query: "body:(a b)",
			want:  "(+#1:a +#1:b)",
		},
		{
			name:  "multi-token bare term on positions field becomes phrase",
			query: "e-mail",
			opts:  []Option{DefaultFields("body")},
			want:  `#1:"e mail"`,
		},
		{
			name:  "multi-token bare term without positions becomes AND",
			query: "desc:e-mail",
			want:  "(+#5:e +#5:mail)",
		},
		{
			name:  "expansion skips fields that cannot take the leaf",
			query: `"a b"`,
			opts:  []Option{DefaultFields("title", "desc")},
			want:  `#0:"a b"`,
		},
		{
			name:  "empty query matches nothing",
			query: "",
			opts:  []Option{DefaultFields("title")},
			want:  "()",
		},
		{
			name:  "leaf analyzing to zero tokens is dropped",
			query: `title:"---"`,
			want:  "()",
		},
		{
			name:  "clause whose children are all dropped is dropped",
			query: `-title:"---"`,
			want:  "()",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := newParser(t, tc.opts...)
			n, err := p.Parse(tc.query)
			if err != nil {
				t.Fatalf("Parse(%q) returned error: %v", tc.query, err)
			}
			if got := n.String(); got != tc.want {
				t.Errorf("Parse(%q).String() = %q, want %q", tc.query, got, tc.want)
			}
		})
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		name    string
		query   string
		opts    []Option
		want    error
		wantPos int
		field   string
	}{
		{"unknown field", "nope:x", nil, ErrUnknownField, 0, "nope"},
		{"non-text field", "year:2024", nil, ErrNotText, 0, "year"},
		{"unindexed field", "meta:x", nil, ErrNotIndexed, 0, "meta"},
		{"phrase on field without positions", `desc:"a b"`, nil, ErrNoPositions, 0, "desc"},
		{"bare term without default fields", "foo", nil, ErrNoField, 0, ""},
		{"error position is the leaf's", `id:"a b" desc:"c d"`, nil, ErrNoPositions, 9, "desc"},
		{"every default field skipped", `"a b"`, []Option{DefaultFields("desc")}, ErrNoPositions, 0, "desc"},
		{"clause limit", "a b", []Option{DefaultFields("title"), MaxClauses(1)}, ErrTooManyClauses, 2, "title"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := newParser(t, tc.opts...)
			_, err := p.Parse(tc.query)
			if err == nil {
				t.Fatalf("Parse(%q) expected error, got nil", tc.query)
			}
			if !errors.Is(err, tc.want) {
				t.Errorf("Parse(%q) error = %v, want errors.Is %v", tc.query, err, tc.want)
			}
			var qe *Error
			if !errors.As(err, &qe) {
				t.Fatalf("Parse(%q) error type = %T, want *Error", tc.query, err)
			}
			if qe.Pos != tc.wantPos {
				t.Errorf("Parse(%q) error Pos = %d, want %d", tc.query, qe.Pos, tc.wantPos)
			}
			if qe.Field != tc.field {
				t.Errorf("Parse(%q) error Field = %q, want %q", tc.query, qe.Field, tc.field)
			}
		})
	}
}

func TestSyntaxErrorPassesThrough(t *testing.T) {
	p := newParser(t, DefaultFields("title"))
	_, err := p.Parse("foo^")
	var pe *parser.Error
	if !errors.As(err, &pe) {
		t.Fatalf(`Parse("foo^") error type = %T, want *parser.Error`, err)
	}
	if pe.Pos != 3 {
		t.Errorf("Pos = %d, want 3", pe.Pos)
	}
}

func TestNewParserValidation(t *testing.T) {
	s := testSchema(t)
	tests := []struct {
		name string
		opts []Option
		want error
	}{
		{"unknown default field", []Option{DefaultFields("nope")}, ErrUnknownField},
		{"non-text default field", []Option{DefaultFields("year")}, ErrNotText},
		{"unindexed default field", []Option{DefaultFields("meta")}, ErrNotIndexed},
		{"non-positive clause limit", []Option{MaxClauses(0)}, ErrTooManyClauses},
		{"negative clause limit", []Option{MaxClauses(-1)}, ErrTooManyClauses},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewParser(s, tc.opts...)
			if !errors.Is(err, tc.want) {
				t.Errorf("NewParser error = %v, want errors.Is %v", err, tc.want)
			}
		})
	}

	t.Run("unregistered tokenizer", func(t *testing.T) {
		bad, err := schema.NewSchema(schema.Field{
			Name: "x", Type: schema.FieldTypeText, Indexed: true, Tokenizer: "nope",
		})
		if err != nil {
			t.Fatalf("NewSchema: %v", err)
		}
		if _, err := NewParser(bad); !errors.Is(err, ErrUnknownTokenizer) {
			t.Errorf("NewParser error = %v, want errors.Is %v", err, ErrUnknownTokenizer)
		}
	})
}

func TestResolveHandBuiltAST(t *testing.T) {
	p := newParser(t, DefaultFields("title"))
	n, err := p.Resolve(&parser.Boost{
		Node:   &parser.Term{Start: 0, Field: "body", Text: "Foo"},
		Factor: 2,
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got := n.String(); got != "#1:foo^2" {
		t.Errorf("got %q, want %q", got, "#1:foo^2")
	}
}
