package query

import "github.com/tsdkv/hoba/internal/index"

var (
	_ Weight = (*termWeight)(nil)
	_ Scorer = (*termScorer)(nil)
)

// The bm25 weight is corpus-wide: idf and avgdl come from Stats at Compile
// time, so every segment's scorer ranks against the same scale.
type termWeight struct {
	field int
	term  string
	w     bm25
}

func newTermWeight(st Stats, field int, term string) *termWeight {
	return &termWeight{
		field: field,
		term:  term,
		w:     newBM25(st.NumDocs(), st.DocFreq(field, term), st.TotalTokens(field)),
	}
}

func (w *termWeight) Scorer(seg index.Segment) Scorer {
	po, ok := seg.Postings(w.field, w.term)
	if !ok {
		return nil
	}
	return &termScorer{PostingsIterator: po, seg: seg, field: w.field, w: w.w}
}

type termScorer struct {
	index.PostingsIterator
	seg   index.Segment
	field int
	w     bm25
}

func (s *termScorer) Score() float64 {
	return s.w.score(s.Freq(), s.seg.DocLen(s.Doc(), s.field))
}
