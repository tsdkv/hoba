package query

import "github.com/tsdkv/hoba/internal/index"

var (
	_ Weight = (*intersectionWeight)(nil)
	_ Scorer = (*intersectionScorer)(nil)
)

type intersectionWeight struct {
	children []Weight
}

func (iw *intersectionWeight) Scorer(seg index.Segment) Scorer {
	scorers := make([]Scorer, 0, len(iw.children))
	for _, w := range iw.children {
		sc := w.Scorer(seg)
		if sc == nil {
			return nil
		}
		scorers = append(scorers, sc)
	}

	if len(scorers) == 0 {
		return nil
	}

	if len(scorers) == 1 {
		return scorers[0]
	}

	ds := newIntersectionDocSet(toDocSets(scorers))
	if ds.Doc() == index.Terminated {
		return nil
	}

	return &intersectionScorer{intersectionDocSet: ds, scorers: scorers}
}

type intersectionDocSet struct {
	children []DocSet
	doc      index.DocID
}

func newIntersectionDocSet(children []DocSet) *intersectionDocSet {
	ds := &intersectionDocSet{children: children}
	ds.doc = ds.align()
	return ds
}

func (ds *intersectionDocSet) Doc() index.DocID {
	return ds.doc
}

func (ds *intersectionDocSet) Advance() index.DocID {
	ds.children[0].Advance()
	ds.doc = ds.align()
	return ds.doc
}

func (ds *intersectionDocSet) Seek(target index.DocID) index.DocID {
	for _, s := range ds.children {
		s.Seek(target)
	}
	ds.doc = ds.align()
	return ds.doc
}

func (ds *intersectionDocSet) align() index.DocID {
	candidate := ds.children[0].Doc()
	for _, s := range ds.children {
		candidate = max(candidate, s.Doc())
	}

	if candidate == index.Terminated {
		return index.Terminated
	}

outer:
	for {
		for _, s := range ds.children {
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

type intersectionScorer struct {
	*intersectionDocSet
	scorers []Scorer
}

func (s *intersectionScorer) Score() float64 {
	var score float64
	for _, sc := range s.scorers {
		score += sc.Score()
	}

	return score
}
