package query

import "github.com/tsdkv/hoba/internal/index"

var (
	_ Weight = (*booleanWeight)(nil)
	_ Scorer = (*booleanScorer)(nil)
)

type booleanWeight struct {
	musts    *intersectionWeight
	shoulds  *unionWeight
	mustNots []Weight
}

func newBooleanWeight(st Stats, children []OccurNode) Weight {
	w := &booleanWeight{}

	var musts []Weight
	var shoulds []Weight
	for _, ch := range children {
		cw := Compile(ch.Node, st)
		if cw == nil {
			continue
		}
		switch ch.Occur {
		case OccurMust:
			musts = append(musts, cw)
		case OccurShould:
			shoulds = append(shoulds, cw)
		case OccurMustNot:
			w.mustNots = append(w.mustNots, cw)
		}
	}

	if len(musts)+len(shoulds) == 0 {
		return nil
	}

	w.musts = &intersectionWeight{musts}
	w.shoulds = &unionWeight{shoulds}

	return w
}

func (w *booleanWeight) Scorer(seg index.Segment) Scorer {
	bs := &booleanScorer{}

	if len(w.musts.children) > 0 {
		bs.musts = w.musts.Scorer(seg)
		if bs.musts == nil {
			return nil
		}
	}

	bs.shoulds = w.shoulds.Scorer(seg)

	for _, mn := range w.mustNots {
		sc := mn.Scorer(seg)
		if sc == nil {
			continue
		}
		bs.mustNots = append(bs.mustNots, sc)
	}

	if bs.musts == nil && bs.shoulds == nil {
		return nil
	}

	bs.Seek(0)
	return bs
}

type booleanScorer struct {
	musts    Scorer
	shoulds  Scorer
	mustNots []Scorer
	doc      index.DocID
}

func (s *booleanScorer) Doc() index.DocID {
	return s.doc
}

func (s *booleanScorer) Advance() index.DocID {
outer:
	for {
		candidate := index.Terminated
		if s.musts != nil {
			candidate = s.musts.Advance()
		} else {
			candidate = s.shoulds.Advance()
		}

		if candidate == index.Terminated {
			s.doc = index.Terminated
			return s.doc
		}

		for _, mn := range s.mustNots {
			if mn.Seek(candidate) == candidate {
				continue outer
			}
		}

		s.doc = candidate
		return s.doc
	}
}

func (s *booleanScorer) Seek(target index.DocID) index.DocID {
	candidate := index.Terminated
	if s.musts != nil {
		candidate = s.musts.Seek(target)
	} else {
		candidate = s.shoulds.Seek(target)
	}

	if candidate == index.Terminated {
		s.doc = index.Terminated
		return s.doc
	}

	excluded := false
	for _, mn := range s.mustNots {
		if mn.Seek(candidate) == candidate {
			excluded = true
		}
	}

	if excluded {
		s.doc = s.Advance()
	} else {
		s.doc = candidate
	}

	return s.doc
}

func (s *booleanScorer) Score() float64 {
	var score float64
	if s.musts != nil {
		score = s.musts.Score()
	}

	if s.shoulds != nil {
		if s.doc == s.shoulds.Seek(s.doc) {
			score += s.shoulds.Score()
		}
	}

	return score
}
