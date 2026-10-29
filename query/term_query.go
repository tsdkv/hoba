package query

import "github.com/tsdkv/hoba/index"

var _ Scorer = (*termScorer)(nil)

type termScorer struct {
	p     index.PostingsIterator
	seg   index.Segment
	field int
	w     bm25
}

func newTermScorer(seg index.Segment, field int, term string) *termScorer {
	po, ok := seg.Postings(field, term)
	if !ok {
		return nil
	}

	return &termScorer{
		p:     po,
		seg:   seg,
		field: field,
		w:     newBM25(seg.NumDocs(), seg.DocFreq(field, term), seg.TotalTokens(field)),
	}
}

func (s *termScorer) Score() float64 {
	return s.w.score(s.p.Freq(), s.seg.DocLen(s.p.Doc(), s.field))
}

func (s *termScorer) Doc() index.DocID {
	return s.p.Doc()
}

func (s *termScorer) Advance() index.DocID {
	return s.p.Advance()
}

func (s *termScorer) Seek(target index.DocID) index.DocID {
	return s.p.Seek(target)
}
