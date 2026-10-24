// Package query turns a parsed query AST into a schema-resolved query
// tree: field names become field IDs, leaf text goes through the field's
// analyzer, and the parser's undecided OccurNone is decided by policy.
package query

import (
	"errors"
	"fmt"

	"github.com/tsdkv/hoba/query/parser"
	"github.com/tsdkv/hoba/schema"
	"github.com/tsdkv/hoba/tokenizer"
)

// Sentinel errors wrapped by Error (and by NewParser's validation errors).
var (
	ErrUnknownField     = errors.New("unknown field")
	ErrNotText          = errors.New("cannot search non-text field")
	ErrNotIndexed       = errors.New("cannot search unindexed field")
	ErrNoPositions      = errors.New("phrase query needs positions on field")
	ErrNoField          = errors.New("no field given and no default fields configured")
	ErrTooManyClauses   = errors.New("query expands to too many clauses")
	ErrUnknownTokenizer = errors.New("unknown tokenizer")
)

// Error describes a schema-resolution problem at one query leaf.
type Error struct {
	Pos   int    // byte offset of the offending leaf
	Field string // field involved, if any
	Err   error  // a sentinel; Unwrap returns it
}

func (e *Error) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("query: %v %q at byte %d", e.Err, e.Field, e.Pos)
	}
	return fmt.Sprintf("query: %v at byte %d", e.Err, e.Pos)
}

func (e *Error) Unwrap() error { return e.Err }

// Parser resolves query strings against a schema. It is safe for
// concurrent use.
type Parser struct {
	s            *schema.Schema
	analyzers    map[int]*tokenizer.Analyzer // indexed text field ID -> analyzer
	defaultIDs   []int
	defaultOccur Occur
	maxClauses   int
}

type config struct {
	defaultFields []string
	defaultOccur  Occur
	manager       *tokenizer.Manager
	maxClauses    int
}

// Option configures NewParser.
type Option func(*config)

// DefaultFields sets the fields searched by leaves without an explicit
// field prefix. A leaf expands into a Should disjunction when more than
// one default field is set.
func DefaultFields(names ...string) Option {
	return func(c *config) { c.defaultFields = names }
}

// DefaultOR resolves whitespace-separated leaves as Should (Lucene-style
// OR semantics) instead of the default Must (AND semantics).
func DefaultOR() Option {
	return func(c *config) { c.defaultOccur = OccurShould }
}

// Tokenizers sets the analyzer registry for leaf text. The default is
// tokenizer.NewManager().
func Tokenizers(m *tokenizer.Manager) Option {
	return func(c *config) { c.manager = m }
}

// MaxClauses bounds the number of leaf clauses after default-field
// expansion; n must be > 0. The default is 1024.
func MaxClauses(n int) Option {
	return func(c *config) { c.maxClauses = n }
}

// NewParser builds a Parser against the schema. It checks that the clause
// limit is positive, that each default field exists and is an indexed text
// field, and that every indexed text field's tokenizer is registered in
// the Manager. These errors wrap the package's sentinel errors.
func NewParser(s *schema.Schema, opts ...Option) (*Parser, error) {
	c := config{
		defaultOccur: OccurMust,
		manager:      tokenizer.NewManager(),
		maxClauses:   1024,
	}
	for _, opt := range opts {
		opt(&c)
	}
	if c.maxClauses <= 0 {
		return nil, fmt.Errorf("query: max clauses %d: %w", c.maxClauses, ErrTooManyClauses)
	}
	analyzers := make(map[int]*tokenizer.Analyzer)
	for _, f := range s.Fields {
		if f.Type != schema.FieldTypeText || !f.Indexed {
			continue
		}
		a, ok := c.manager.Get(f.Tokenizer)
		if !ok {
			return nil, fmt.Errorf("query: field %q: %w %q", f.Name, ErrUnknownTokenizer, f.Tokenizer)
		}
		analyzers[f.ID] = a
	}
	defaultIDs := make([]int, 0, len(c.defaultFields))
	for _, name := range c.defaultFields {
		f, ok := s.Field(name)
		if !ok {
			return nil, fmt.Errorf("query: default field %q: %w", name, ErrUnknownField)
		}
		if f.Type != schema.FieldTypeText {
			return nil, fmt.Errorf("query: default field %q: %w", name, ErrNotText)
		}
		if !f.Indexed {
			return nil, fmt.Errorf("query: default field %q: %w", name, ErrNotIndexed)
		}
		defaultIDs = append(defaultIDs, f.ID)
	}
	return &Parser{
		s:            s,
		analyzers:    analyzers,
		defaultIDs:   defaultIDs,
		defaultOccur: c.defaultOccur,
		maxClauses:   c.maxClauses,
	}, nil
}

// Parse parses q and resolves it against the schema.
func (p *Parser) Parse(q string) (Node, error) {
	ast, err := parser.Parse(q)
	if err != nil {
		return nil, err
	}
	return p.Resolve(ast)
}

// Resolve resolves a parsed (or hand-built) AST: it verifies that every
// field exists, is indexed and supports the query shape, analyzes leaf
// text with the field's analyzer, decides OccurNone by policy (Must unless
// DefaultOR is set), and rewrites the tree bottom-up — dropped leaves
// disappear, an all-negative Boolean gains a MatchAll Must child, and a
// single-child Boolean is unwrapped.
func (p *Parser) Resolve(ast parser.Node) (Node, error) {
	r := resolver{p: p}
	n, err := r.node(ast)
	if err != nil {
		return nil, err
	}
	if n == nil {
		return &Boolean{}, nil // every leaf was dropped: matches nothing
	}
	return n, nil
}
