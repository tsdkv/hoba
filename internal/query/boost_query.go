package query

import "github.com/tsdkv/hoba/internal/index"

var (
	_ Weight = (*boostWeight)(nil)
	_ Scorer = (*boostScorer)(nil)
)

type boostWeight struct {
	child  Weight
	factor float64
}

func (w *boostWeight) Scorer(seg index.Segment) Scorer {
	child := w.child.Scorer(seg)
	if child == nil {
		return nil
	}
	return &boostScorer{child: child, factor: w.factor}
}

type boostScorer struct {
	child  Scorer
	factor float64
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
