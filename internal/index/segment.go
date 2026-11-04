package index

import (
	"math"

	"github.com/tsdkv/hoba/schema"
)

type DocID uint32

// Terminated is the sentinel DocID returned by an exhausted Postings iterator.
const Terminated = DocID(math.MaxUint32)

type DocAddress struct {
	SegmentOrd int
	DocID      DocID
}

type Segment interface {
	NumDocs() int
	DocFreq(field int, term string) int
	DocLen(doc DocID, field int) uint32
	Postings(field int, term string) (PostingsIterator, bool)
	TotalTokens(field int) uint64
	Stored(doc DocID) (schema.Document, bool) // hits → docs
}

// PostingsIterator iterates one term's posting list, sorted by DocID.
// A fresh iterator is positioned on the first posting; Doc returns
// Terminated once the list is exhausted.
type PostingsIterator interface {
	Doc() DocID
	Advance() DocID
	// Seek returns the first doc >= target, never moving backwards.
	Seek(target DocID) DocID
	Freq() uint32
	Positions() []uint32
}
