package query

import "github.com/tsdkv/hoba/internal/index"

var (
	_ Weight = (*disjunctionWeight)(nil)
	_ Scorer = (*disjunctionScorer)(nil)
)

type disjunctionWeight struct {
	children []Weight
}

func newDisjunctionWeight(c []Weight) *disjunctionWeight {
	return &disjunctionWeight{c}
}

func (dw *disjunctionWeight) Scorer(seg index.Segment) Scorer {
	ds := &disjunctionScorer{}

	for _, c := range dw.children {
		sc := c.Scorer(seg)
		if sc == nil {
			continue
		}
		ds.children = append(ds.children, sc)
	}

	if len(ds.children) == 0 {
		return nil
	}

	if len(ds.children) == 1 {
		return ds.children[0]
	}

	ds.doc = index.Terminated
	for _, s := range ds.children {
		ds.doc = min(ds.doc, s.Doc())
	}

	return ds
}

type disjunctionScorer struct {
	children []Scorer
	doc      index.DocID
}

func (sd *disjunctionScorer) Doc() index.DocID {
	return sd.doc
}

func (sd *disjunctionScorer) Advance() index.DocID {
	candidate := index.Terminated
	for _, s := range sd.children {
		if s.Doc() == sd.doc {
			s.Advance()
		}
		candidate = min(candidate, s.Doc())
	}

	sd.doc = candidate
	return sd.doc
}

func (sd *disjunctionScorer) Seek(target index.DocID) index.DocID {
	sd.doc = index.Terminated
	for _, s := range sd.children {
		sd.doc = min(sd.doc, s.Seek(target))
	}
	return sd.doc
}

func (sd *disjunctionScorer) Score() float64 {
	score := float64(0)
	for _, s := range sd.children {
		if s.Doc() == sd.doc {
			score += s.Score()
		}
	}

	return score
}
