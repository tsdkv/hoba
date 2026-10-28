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
type PostingsIterator interface {
	Doc() DocID
	Advance() DocID
	// Seek returns the first doc >= target, never moving backwards.
	Seek(target DocID) DocID
	Freq() uint32
}

type slicePostings struct {
	p   []Posting
	cur int // -1 before the first Advance
}

func (sp *slicePostings) Doc() DocID {
	if sp.cur < 0 || sp.cur >= len(sp.p) {
		return Terminated
	}
	return sp.p[sp.cur].Doc
}

func (sp *slicePostings) Advance() DocID {
	sp.cur++
	return sp.Doc()
}

func (sp *slicePostings) Seek(target DocID) DocID {
	if sp.cur < 0 {
		sp.cur = 0
	}
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
