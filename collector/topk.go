package collector

import (
	"container/heap"
	"sort"

	"github.com/tsdkv/hoba/index"
)

type Hit struct {
	Doc   index.DocID
	Score float64
}

func (a Hit) better(b Hit) bool {
	return a.Score > b.Score || (a.Score == b.Score && a.Doc < b.Doc)
}

type hitHeap []Hit

func (h hitHeap) Len() int           { return len(h) }
func (h hitHeap) Less(i, j int) bool { return h[j].better(h[i]) }
func (h hitHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *hitHeap) Push(x any) {
	*h = append(*h, x.(Hit))
}

func (h *hitHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

// TopK collects the k best hits, skipping the first offset. Collect assumes
// each doc is collected at most once; duplicates count as independent hits.
//
// TODO: swap the heap for a 2*k buffer with quickselect truncation and
// score-threshold rejection (Tantivy TopNComputer) — amortized O(1) per push.
type TopK struct {
	offset int
	k      int
	hh     hitHeap
}

// NewTopK returns a collector keeping the k best hits. k <= 0 collects nothing.
func NewTopK(k int) *TopK {
	return NewTopKRange(0, k)
}

// NewTopKRange returns a collector keeping hits ranked [offset, offset+k).
func NewTopKRange(offset, k int) *TopK {
	offset = max(offset, 0)
	k = max(k, 0)
	return &TopK{offset: offset, k: k, hh: make(hitHeap, 0, offset+k)}
}

func (c *TopK) Collect(doc index.DocID, score float64) {
	if c.k == 0 {
		return
	}
	h := Hit{Doc: doc, Score: score}
	if len(c.hh) < c.offset+c.k {
		heap.Push(&c.hh, h)
		return
	}
	if h.better(c.hh[0]) {
		c.hh[0] = h
		heap.Fix(&c.hh, 0)
	}
}

func (c *TopK) Hits() []Hit {
	out := make([]Hit, len(c.hh))
	copy(out, c.hh)
	sort.Slice(out, func(i, j int) bool { return out[i].better(out[j]) })
	return out[min(c.offset, len(out)):]
}
