package query

import "github.com/tsdkv/hoba/index"

type DocSet interface {
	Doc() index.DocID
	Advance() index.DocID
	Seek(target index.DocID) index.DocID
}

type Scorer interface {
	DocSet
	Score() float64
}
