package query

import "github.com/tsdkv/hoba/internal/index"

var (
	_ Weight = (*intersectionWeight)(nil)
	_ Scorer = (*intersectionScorer)(nil)
)

type intersectionWeight struct {
	children []Weight
}

func newIntersectionWeight(s []Weight) *intersectionWeight {
	return &intersectionWeight{s}
}

func (iw *intersectionWeight) Scorer(seg index.Segment) Scorer {
	s := &intersectionScorer{}

	for _, w := range iw.children {
		sc := w.Scorer(seg)
		if sc == nil {
			return nil
		}
		s.children = append(s.children, sc)
	}

	if len(s.children) == 0 {
		return nil
	}

	if len(s.children) == 1 {
		return s.children[0]
	}

	s.doc = s.align()
	if s.doc == index.Terminated {
		return nil
	}

	return s
}

type intersectionScorer struct {
	children []Scorer
	doc      index.DocID
}

func (is *intersectionScorer) Doc() index.DocID {
	return is.doc
}

func (is *intersectionScorer) Advance() index.DocID {
	is.children[0].Advance()
	is.doc = is.align()
	return is.doc
}

func (is *intersectionScorer) Seek(target index.DocID) index.DocID {
	for _, s := range is.children {
		s.Seek(target)
	}
	is.doc = is.align()
	return is.doc
}

// Score is valid only while Doc is a real document.
func (is *intersectionScorer) Score() float64 {
	score := float64(0)
	for _, s := range is.children {
		score += s.Score()
	}

	return score
}

func (is *intersectionScorer) align() index.DocID {
	candidate := is.children[0].Doc()
	for _, s := range is.children {
		candidate = max(candidate, s.Doc())
	}

	if candidate == index.Terminated {
		return index.Terminated
	}

outer:
	for {
		for _, s := range is.children {
			cc := s.Seek(candidate)
			if cc == index.Terminated {
				return index.Terminated
			}

			if cc != candidate {
				candidate = cc
				continue outer
			}
		}

		return candidate
	}
}
