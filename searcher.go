package hoba

import (
	"fmt"

	"github.com/tsdkv/hoba/internal/collector"
	"github.com/tsdkv/hoba/internal/index"
	"github.com/tsdkv/hoba/internal/query"
	"github.com/tsdkv/hoba/schema"
)

type (
	Hit        = collector.Hit
	DocAddress = index.DocAddress

	Document = schema.Document
	Schema   = schema.Schema
)

var _ query.Stats = (*snapshot)(nil)

type snapshot struct {
	opstamp uint64
	segs    []index.Segment
}

func newSnapshot(opstamp uint64, segs []index.Segment) *snapshot {
	return &snapshot{
		opstamp: opstamp,
		segs:    segs,
	}
}

func (s *snapshot) NumDocs() int {
	sum := 0
	for _, seg := range s.segs {
		sum += seg.NumDocs()
	}
	return sum
}

func (s *snapshot) TotalTokens(field int) uint64 {
	total := uint64(0)
	for _, seg := range s.segs {
		total += seg.TotalTokens(field)
	}
	return total
}

func (s *snapshot) DocFreq(field int, term string) int {
	freq := 0
	for _, seg := range s.segs {
		freq += seg.DocFreq(field, term)
	}
	return freq
}

type Searcher struct {
	snap   *snapshot
	parser *query.Parser
}

func newSearcher(parser *query.Parser, snap *snapshot) *Searcher {
	return &Searcher{snap, parser}
}

func (s *Searcher) Search(qs string, k int) ([]Hit, error) {
	n, err := s.parser.Parse(qs)
	if err != nil {
		return nil, err
	}

	w := query.Compile(n, s.snap)
	if w == nil {
		return nil, nil
	}

	// TODO: parallel search
	top := collector.NewTopK(k)
	for ord, seg := range s.snap.segs {
		scorer := w.Scorer(seg)
		if scorer == nil {
			continue
		}

		for doc := scorer.Doc(); doc != index.Terminated; doc = scorer.Advance() {
			top.Collect(index.DocAddress{SegmentOrd: ord, DocID: doc}, scorer.Score())
		}
	}

	return top.Hits(), nil
}

func (s *Searcher) Doc(addr DocAddress) (Document, error) {
	if addr.SegmentOrd >= len(s.snap.segs) {
		return Document{}, fmt.Errorf("wrong segment ord (too big): %d (num segments: %d)", addr.SegmentOrd, len(s.snap.segs))
	}

	doc, ok := s.snap.segs[addr.SegmentOrd].Stored(addr.DocID)
	if !ok {
		return Document{}, fmt.Errorf("not found")
	}

	return doc, nil
}
