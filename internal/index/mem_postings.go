package index

type Posting struct {
	Doc       DocID
	Freq      uint32
	Positions []uint32
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
