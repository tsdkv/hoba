package query

import "github.com/tsdkv/hoba/index"

type boostScorer struct {
	child  Scorer
	factor float64
}

func newBoostScorer(child Scorer, factor float64) *boostScorer {
	return &boostScorer{child: child, factor: factor}
}

func (s *boostScorer) Doc() index.DocID {
	return s.child.Doc()
}

func (s *boostScorer) Advance() index.DocID {
	return s.child.Advance()
}

func (s *boostScorer) Seek(target index.DocID) index.DocID {
	return s.child.Seek(target)
}

func (s *boostScorer) Score() float64 {
	return s.child.Score() * s.factor
}
