package index

import (
	"github.com/tsdkv/hoba/schema"
)

type Segment interface {
	NumDocs() int
	DocFreq(field int, term string) int
	DocLen(doc DocID, field int) uint32
	Postings(field int, term string) (PostingsIterator, bool)
	TotalTokens(field int) uint64
	Stored(doc DocID) (schema.Document, bool) // hits → docs
}
