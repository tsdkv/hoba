package query

import "github.com/tsdkv/hoba/internal/index"

var (
	_ Weight = matchAllWeight{}
	_ Scorer = (*matchAllScorer)(nil)
)

type matchAllWeight struct{}

func (matchAllWeight) Scorer(seg index.Segment) Scorer {
	if seg.NumDocs() == 0 {
		return nil
	}
	return newMatchAllScorer(seg)
}

type matchAllScorer struct {
	numDocs int
	cur     int
}

func newMatchAllScorer(seg index.Segment) *matchAllScorer {
	return &matchAllScorer{numDocs: seg.NumDocs()}
}

func (s *matchAllScorer) Score() float64 {
	return 1.0
}

func (s *matchAllScorer) Doc() index.DocID {
	if s.cur >= s.numDocs {
		return index.Terminated
	}

	return index.DocID(s.cur)
}

func (s *matchAllScorer) Advance() index.DocID {
	s.cur++
	return s.Doc()
}

func (s *matchAllScorer) Seek(target index.DocID) index.DocID {
	if int64(target) > int64(s.cur) {
		s.cur = int(target)
	}
	return s.Doc()
}
