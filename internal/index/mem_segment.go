package index

import (
	"fmt"

	"github.com/tsdkv/hoba/schema"
	"github.com/tsdkv/hoba/tokenizer"
)

var _ Segment = (*MemSegment)(nil)

type MemSegment struct {
	schema *schema.Schema

	postings *slicePostingsBuffer
	tm       *tokenizer.Manager

	lens   [][]uint32 // [fieldID][docID] — fieldnorms
	totals []uint64   // [fieldID] total tokens

	stored []schema.Document
}

func NewMemSegment(s *schema.Schema, tm *tokenizer.Manager) *MemSegment {
	return &MemSegment{
		schema:   s,
		tm:       tm,
		postings: newSlicePostingsBuffer(),
		lens:     make([][]uint32, len(s.Fields)),
		totals:   make([]uint64, len(s.Fields)),
	}
}

func (m *MemSegment) Add(doc schema.Document) (DocID, error) {
	if err := m.schema.Validate(doc); err != nil {
		return 0, err
	}
	for _, df := range doc.Fields {
		f, _ := m.schema.FieldByID(df.Field)
		if f.Type == schema.FieldTypeText {
			if _, ok := m.tm.Get(f.Tokenizer); !ok {
				return 0, fmt.Errorf("index: unknown tokenizer %q", f.Tokenizer)
			}
		}
	}

	docID := DocID(m.NumDocs())
	for fid, f := range m.schema.Fields {
		if f.Type == schema.FieldTypeText && f.Indexed {
			m.lens[fid] = append(m.lens[fid], 0)
		}
	}
	for _, df := range doc.Fields {
		f, _ := m.schema.FieldByID(df.Field) // validated above
		if !f.Indexed {
			continue
		}
		switch f.Type {
		case schema.FieldTypeInt64:
			m.postings.Record(f.ID, EncodeInt64(df.Value.Num), docID, 0, false)
		case schema.FieldTypeText:
			an, _ := m.tm.Get(f.Tokenizer) // validated above
			var n uint32
			for token := range an.Tokens(df.Value.Str) {
				m.postings.Record(f.ID, token.Text, docID, uint32(token.Position), f.Positions)
				n++
			}
			m.lens[f.ID][docID] = n
			m.totals[f.ID] += uint64(n)
		}
	}

	m.stored = append(m.stored, doc)
	return docID, nil
}

func (m *MemSegment) Postings(field int, term string) (PostingsIterator, bool) {
	return m.postings.Postings(field, term)
}

func (m *MemSegment) NumDocs() int {
	return len(m.stored)
}

func (m *MemSegment) Schema() *schema.Schema { return m.schema }

func (m *MemSegment) TokenizerManager() *tokenizer.Manager { return m.tm }

func (m *MemSegment) DocFreq(field int, term string) int {
	return m.postings.DocFreq(field, term)
}

func (m *MemSegment) DocLen(doc DocID, field int) uint32 {
	return m.lens[field][doc]
}

func (m *MemSegment) TotalTokens(field int) uint64 {
	return m.totals[field]
}

func (m *MemSegment) Stored(doc DocID) (schema.Document, bool) {
	if doc >= DocID(len(m.stored)) {
		return schema.Document{}, false
	}

	return m.stored[doc], true
}
