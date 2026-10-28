package query

import "github.com/tsdkv/hoba/index"

type TermScorer struct {
	p     index.PostingsIterator
	seg   *index.MemSegment
	field int
	w     bm25
}

func newTermScorer(seg *index.MemSegment, field int, term string) *TermScorer {
	po, ok := seg.Postings(field, term)
	if !ok {
		return nil
	}

	return &TermScorer{
		p:     po,
		seg:   seg,
		field: field,
		w:     newBM25(seg.NumDocs(), seg.DocFreq(field, term), seg.TotalTokens(field)),
	}
}

func (s *TermScorer) Score() float64 {
	return s.w.score(s.p.Freq(), s.seg.DocLen(s.p.Doc(), s.field))
}

func (s *TermScorer) Doc() index.DocID {
	return s.p.Doc()
}

func (s *TermScorer) Advance() index.DocID {
	return s.p.Advance()
}

func (s *TermScorer) Seek(target index.DocID) index.DocID {
	return s.p.Seek(target)
}
