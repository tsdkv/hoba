package index

import "math"

type DocID uint32

// Terminated is the sentinel DocID returned by an exhausted Postings iterator.
const Terminated = DocID(math.MaxUint32)

type Posting struct {
	Doc       DocID
	Freq      uint32
	Positions []uint32
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

type slicePostings struct {
	p   []Posting
	cur int
}

func (sp *slicePostings) Doc() DocID {
	if sp.cur >= len(sp.p) {
		return Terminated
	}
	return sp.p[sp.cur].Doc
}

func (sp *slicePostings) Advance() DocID {
	sp.cur++
	return sp.Doc()
}

func (sp *slicePostings) Seek(target DocID) DocID {
	for sp.cur < len(sp.p) && sp.p[sp.cur].Doc < target {
		sp.cur++
	}
	return sp.Doc()
}

func (sp *slicePostings) Freq() uint32 {
	if sp.cur < 0 || sp.cur >= len(sp.p) {
		return 0
	}
	return sp.p[sp.cur].Freq
}

func (sp *slicePostings) Positions() []uint32 {
	return sp.p[sp.cur].Positions
}
