package query

import "github.com/tsdkv/hoba/internal/index"

var (
	_ Weight = (*phraseWeight)(nil)
	_ Scorer = (*phraseScorer)(nil)
)

type postingsWithOffset struct {
	index.PostingsIterator
	offset uint32
}

func (p *postingsWithOffset) Positions() []uint32 {
	positions := p.PostingsIterator.Positions()
	shifted := make([]uint32, len(positions))
	for i, position := range positions {
		shifted[i] = position + p.offset
	}

	return shifted
}

type phraseWeight struct {
	fieldID int
	terms   []*termWeight
	offsets []uint32
}

func newPhraseWeight(st Stats, p *Phrase) Weight {
	if len(p.Terms) == 0 {
		return nil
	}
	if len(p.Terms) == 1 {
		return newTermWeight(st, p.FieldID, p.Terms[0])
	}

	maxPos := 0
	positions := make([]int, len(p.Terms))
	for i := range p.Terms {
		positions[i] = i
		if i < len(p.Positions) {
			positions[i] = p.Positions[i]
		}
		maxPos = max(maxPos, positions[i])
	}

	pw := &phraseWeight{fieldID: p.FieldID}
	for i, term := range p.Terms {
		pw.terms = append(pw.terms, newTermWeight(st, p.FieldID, term))
		pw.offsets = append(pw.offsets, uint32(maxPos-positions[i]))
	}

	return pw
}

func (pw *phraseWeight) Scorer(seg index.Segment) Scorer {
	ps := &phraseScorer{
		seg:     seg,
		fieldID: pw.fieldID,
	}

	docsets := make([]DocSet, 0, len(pw.terms))
	for i, term := range pw.terms {
		sc := term.Scorer(seg)
		if sc == nil {
			return nil
		}
		ts := sc.(*termScorer)

		docsets = append(docsets, sc)
		ps.postings = append(ps.postings, &postingsWithOffset{
			PostingsIterator: ts.PostingsIterator,
			offset:           pw.offsets[i],
		})
		ps.bm25 = append(ps.bm25, term.w)
	}

	ps.docs = newIntersectionDocSet(docsets)
	if ps.Seek(0) == index.Terminated {
		return nil
	}
	return ps
}

type phraseScorer struct {
	docs     *intersectionDocSet
	postings []*postingsWithOffset

	seg     index.Segment
	fieldID int
	bm25    []bm25

	phraseCount uint32
	doc         index.DocID
}

func (ps *phraseScorer) Doc() index.DocID {
	return ps.doc
}

func (ps *phraseScorer) Advance() index.DocID {
	for {
		ps.doc = ps.docs.Advance()
		if ps.doc == index.Terminated {
			return ps.doc
		}

		if ps.phraseMatch() {
			return ps.doc
		}
	}
}

func (ps *phraseScorer) Seek(target index.DocID) index.DocID {
	ps.doc = ps.docs.Seek(target)
	if ps.doc == index.Terminated {
		return ps.doc
	}

	if ps.phraseMatch() {
		return ps.doc
	}

	return ps.Advance()
}

func (ps *phraseScorer) Score() float64 {
	var score float64
	docLen := ps.seg.DocLen(ps.doc, ps.fieldID)
	for _, bm25 := range ps.bm25 {
		score += bm25.score(ps.phraseCount, docLen)
	}

	return score
}

func (ps *phraseScorer) phraseMatch() bool {
	lPos := ps.postings[0].Positions()

	for _, term := range ps.postings[1:] {
		if len(lPos) == 0 {
			ps.phraseCount = 0
			return false
		}

		rPos := term.Positions()
		var newlPos []uint32

		li, ri := 0, 0
		for li < len(lPos) && ri < len(rPos) {
			switch {
			case lPos[li] == rPos[ri]:
				newlPos = append(newlPos, lPos[li])
				li++
				ri++
			case lPos[li] < rPos[ri]:
				li++
			case lPos[li] > rPos[ri]:
				ri++
			}
		}
		lPos = newlPos
	}

	ps.phraseCount = uint32(len(lPos))
	return ps.phraseCount > 0
}
