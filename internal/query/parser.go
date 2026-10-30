package query

import (
	"errors"
	"fmt"

	"github.com/tsdkv/hoba/internal/query/parser"
	"github.com/tsdkv/hoba/schema"
	"github.com/tsdkv/hoba/tokenizer"
)

var (
	ErrUnknownField     = errors.New("unknown field")
	ErrNotText          = errors.New("cannot search non-text field")
	ErrNotIndexed       = errors.New("cannot search unindexed field")
	ErrNoPositions      = errors.New("phrase query needs positions on field")
	ErrNoField          = errors.New("no field given and no default fields configured")
	ErrTooManyClauses   = errors.New("query expands to too many clauses")
	ErrUnknownTokenizer = errors.New("unknown tokenizer")
)

type Error struct {
	Pos   int
	Field string
	Err   error
}

func (e *Error) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("query: %v %q at byte %d", e.Err, e.Field, e.Pos)
	}
	return fmt.Sprintf("query: %v at byte %d", e.Err, e.Pos)
}

func (e *Error) Unwrap() error { return e.Err }

type Parser struct {
	s            *schema.Schema
	analyzers    map[int]*tokenizer.Analyzer
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

type Option func(*config)

func DefaultFields(names ...string) Option {
	return func(c *config) { c.defaultFields = names }
}

func DefaultOR() Option {
	return func(c *config) { c.defaultOccur = OccurShould }
}

func Tokenizers(m *tokenizer.Manager) Option {
	return func(c *config) { c.manager = m }
}

func MaxClauses(n int) Option {
	return func(c *config) { c.maxClauses = n }
}

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

func (p *Parser) Parse(q string) (Node, error) {
	ast, err := parser.Parse(q)
	if err != nil {
		return nil, err
	}
	return p.Resolve(ast)
}

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
