package query

import "github.com/tsdkv/hoba/internal/index"

// A DocSet starts positioned on its first document; Doc returns
// index.Terminated once iteration is exhausted.
type DocSet interface {
	Doc() index.DocID
	Advance() index.DocID
	Seek(target index.DocID) index.DocID
}

type Scorer interface {
	DocSet
	// Score is valid only while Doc is a real document.
	Score() float64
}

type Stats interface {
	NumDocs() int
	TotalTokens(field int) uint64
	DocFreq(field int, term string) int
}

// Scorer returns nil when nothing in seg can match.
type Weight interface {
	Scorer(seg index.Segment) Scorer
}
