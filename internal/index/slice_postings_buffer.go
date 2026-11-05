package index

import "strings"

type termKey struct {
	field int
	text  string
}

type slicePostingsBuffer struct {
	postings map[termKey][]Posting
}

func newSlicePostingsBuffer() *slicePostingsBuffer {
	return &slicePostingsBuffer{
		postings: map[termKey][]Posting{},
	}
}

func (pb *slicePostingsBuffer) Record(field int, term string, doc DocID, pos uint32, withPositions bool) {
	key := termKey{field, term}
	p, ok := pb.postings[key]
	if !ok {
		// Token text aliases the document; cloning avoids pinning it in memory.
		key.text = strings.Clone(key.text)
	}

	if n := len(p); n > 0 && p[n-1].Doc == doc {
		p[n-1].Freq++
		if withPositions {
			p[n-1].Positions = append(p[n-1].Positions, pos)
		}
	} else {
		posting := Posting{Doc: doc, Freq: 1}
		if withPositions {
			posting.Positions = []uint32{pos}
		}
		p = append(p, posting)
	}
	pb.postings[key] = p
}

func (pb *slicePostingsBuffer) Postings(field int, term string) (PostingsIterator, bool) {
	p, ok := pb.postings[termKey{field, term}]
	if !ok {
		return nil, false
	}
	return &slicePostings{p: p}, true
}

func (pb *slicePostingsBuffer) DocFreq(field int, term string) int {
	return len(pb.postings[termKey{field, term}])
}
