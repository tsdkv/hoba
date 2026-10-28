package query

import "github.com/tsdkv/hoba/index"

type BoostScorer struct {
	child  Scorer
	factor float64
}

func newBoostScorer(child Scorer, factor float64) *BoostScorer {
	return &BoostScorer{child: child, factor: factor}
}

func (s *BoostScorer) Doc() index.DocID {
	return s.child.Doc()
}

func (s *BoostScorer) Advance() index.DocID {
	return s.child.Advance()
}

func (s *BoostScorer) Seek(target index.DocID) index.DocID {
	return s.child.Seek(target)
}

func (s *BoostScorer) Score() float64 {
	return s.child.Score() * s.factor
}
