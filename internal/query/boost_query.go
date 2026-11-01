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
	return &boostScorer{Scorer: child, factor: w.factor}
}

type boostScorer struct {
	Scorer
	factor float64
}

func (s *boostScorer) Score() float64 {
	return s.Scorer.Score() * s.factor
}
