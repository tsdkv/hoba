package query

import "github.com/tsdkv/hoba/internal/index"

func Compile(q Node, seg index.Segment) Scorer {
	switch n := q.(type) {
	case *Term:
		if s := newTermScorer(seg, n.FieldID, n.Text); s != nil {
			return s
		}
		return nil
	case *Boost:
		if child := Compile(n.Node, seg); child != nil {
			return newBoostScorer(child, n.Factor)
		}
		return nil
	case *MatchAll:
		return newMatchAllScorer(seg)
	case *Boolean, *Phrase:
		panic("unimplemented")
	}

	return nil
}
