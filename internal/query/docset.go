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

func toDocSets(scorers []Scorer) []DocSet {
	docsets := make([]DocSet, len(scorers))
	for i, s := range scorers {
		docsets[i] = s
	}
	return docsets
}
