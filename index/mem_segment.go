package index

import (
	"fmt"
	"strings"

	"github.com/tsdkv/hoba/schema"
	"github.com/tsdkv/hoba/tokenizer"
)

type termKey struct {
	field int
	text  string
}

type MemSegment struct {
	schema *schema.Schema

	postings map[termKey][]Posting
	tm       *tokenizer.Manager

	lens   [][]uint32 // [fieldID][docID] — fieldnorms
	totals []uint64   // [fieldID] total tokens

	stored []schema.Document
}

func NewMemSegment(s *schema.Schema, tm *tokenizer.Manager) *MemSegment {
	return &MemSegment{
		schema:   s,
		tm:       tm,
		postings: map[termKey][]Posting{},
		totals:   make([]uint64, len(s.Fields)),
	}
}

func (m *MemSegment) Add(doc schema.Document) (DocID, error) {
	seen := make([]bool, len(m.schema.Fields))
	for _, df := range doc.Fields {
		f, ok := m.schema.FieldByID(df.Field)
		if !ok {
			return 0, fmt.Errorf("index: unknown field ID: %d", df.Field)
		}

		if seen[df.Field] {
			return 0, fmt.Errorf("index: duplicate field %q", f.Name)
		}
		seen[df.Field] = true

		switch f.Type {
		case schema.FieldTypeInt64:
			if df.Value.Kind != schema.Int64 {
				return 0, fmt.Errorf("index: field %q: not an int64 value", f.Name)
			}
		case schema.FieldTypeText:
			if df.Value.Kind != schema.Text {
				return 0, fmt.Errorf("index: field %q: not a text value", f.Name)
			}
			if _, ok := m.tm.Get(f.Tokenizer); !ok {
				return 0, fmt.Errorf("index: unknown tokenizer %q", f.Tokenizer)
			}
		default:
			return 0, fmt.Errorf("index: unexpected field type: %s", f.Type)
		}
	}

	docID := DocID(m.NumDocs())
	for _, df := range doc.Fields {
		f, _ := m.schema.FieldByID(df.Field) // validated above
		if !f.Indexed {
			continue
		}
		switch f.Type {
		case schema.FieldTypeInt64:
			m.appendPosting(termKey{f.ID, EncodeInt64(df.Value.Num)}, docID, 0, false)
		case schema.FieldTypeText:
			an, _ := m.tm.Get(f.Tokenizer) // validated above
			var n uint32
			for token := range an.Tokens(df.Value.Str) {
				m.appendPosting(termKey{f.ID, token.Text}, docID, uint32(token.Position), f.Positions)
				n++
			}
			m.lens[f.ID] = append(m.lens[f.ID], n)
			m.totals[f.ID] += uint64(n)
		}
	}

	m.stored = append(m.stored, doc)
	return docID, nil
}

func (m *MemSegment) Postings(field int, term string) (Postings, bool) {
	p, ok := m.postings[termKey{field, term}]
	if !ok {
		return nil, false
	}
	return &slicePostings{p: p, cur: -1}, true
}

func (m *MemSegment) NumDocs() int {
	return len(m.stored)
}

func (m *MemSegment) DocLen(doc DocID, field int) uint32 {
	return m.lens[field][doc]
}

func (m *MemSegment) TotalTokens(field int) uint64 {
	return m.totals[field]
}

func (m *MemSegment) appendPosting(k termKey, doc DocID, pos uint32, recordPos bool) {
	p, ok := m.postings[k]
	if !ok {
		// Token text is a substring of the document; using it as a map key
		// would pin the whole document in memory. Clone once, at key creation.
		k.text = strings.Clone(k.text)
	}
	if n := len(p); n > 0 && p[n-1].Doc == doc {
		p[n-1].Freq++
		if recordPos {
			p[n-1].Positions = append(p[n-1].Positions, pos)
		}
	} else {
		posting := Posting{Doc: doc, Freq: 1}
		if recordPos {
			posting.Positions = []uint32{pos}
		}
		p = append(p, posting)
	}
	m.postings[k] = p
}
