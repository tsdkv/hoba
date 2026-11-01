package query

import "github.com/tsdkv/hoba/internal/index"

type Stats interface {
	NumDocs() int
	TotalTokens(field int) uint64
	DocFreq(field int, term string) int
}

// Scorer returns nil when nothing in seg can match.
type Weight interface {
	Scorer(seg index.Segment) Scorer
}

func Compile(q Node, st Stats) Weight {
	switch n := q.(type) {
	case *Term:
		return newTermWeight(st, n.FieldID, n.Text)
	case *Boost:
		if child := Compile(n.Node, st); child != nil {
			return &boostWeight{child: child, factor: n.Factor}
		}
		return nil
	case *MatchAll:
		return matchAllWeight{}
	case *Boolean:
		return newBooleanWeight(st, n.Children)
	case *Phrase:
		panic("unimplemented")
	}

	return nil
}
