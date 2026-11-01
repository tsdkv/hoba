package query

import "github.com/tsdkv/hoba/internal/index"

var (
	_ Weight = (*unionWeight)(nil)
	_ Scorer = (*unionScorer)(nil)
)

type unionWeight struct {
	children []Weight
}

func (uw *unionWeight) Scorer(seg index.Segment) Scorer {
	scorers := make([]Scorer, 0, len(uw.children))
	for _, c := range uw.children {
		sc := c.Scorer(seg)
		if sc == nil {
			continue
		}
		scorers = append(scorers, sc)
	}

	if len(scorers) == 0 {
		return nil
	}

	if len(scorers) == 1 {
		return scorers[0]
	}

	return &unionScorer{unionDocSet: newUnionDocSet(toDocSets(scorers)), scorers: scorers}
}

type unionDocSet struct {
	children []DocSet
	doc      index.DocID
}

func newUnionDocSet(children []DocSet) *unionDocSet {
	ds := &unionDocSet{children: children, doc: index.Terminated}
	for _, s := range children {
		ds.doc = min(ds.doc, s.Doc())
	}
	return ds
}

func (ds *unionDocSet) Doc() index.DocID {
	return ds.doc
}

func (ds *unionDocSet) Advance() index.DocID {
	candidate := index.Terminated
	for _, s := range ds.children {
		if s.Doc() == ds.doc {
			s.Advance()
		}
		candidate = min(candidate, s.Doc())
	}

	ds.doc = candidate
	return ds.doc
}

func (ds *unionDocSet) Seek(target index.DocID) index.DocID {
	ds.doc = index.Terminated
	for _, s := range ds.children {
		ds.doc = min(ds.doc, s.Seek(target))
	}
	return ds.doc
}

type unionScorer struct {
	*unionDocSet
	scorers []Scorer
}

func (s *unionScorer) Score() float64 {
	var score float64
	for _, sc := range s.scorers {
		if sc.Doc() == s.Doc() {
			score += sc.Score()
		}
	}

	return score
}
