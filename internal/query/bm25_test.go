package query

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// 3 docs, term in 2 of them, field lengths 1/2/5 → avgdl 8/3.
func TestBM25HandComputed(t *testing.T) {
	w := newBM25(3, 2, 8)

	assert.InDelta(t, 0.47000362924573563, w.idf, 1e-12)
	assert.InDelta(t, 2.6666666666666665, w.avgdl, 1e-12)

	assert.InDelta(t, 0.6314552576125915, w.score(1, 1), 1e-12)
	assert.InDelta(t, 0.6951314180441133, w.score(2, 2), 1e-12)
	assert.InDelta(t, 0.3461114591935125, w.score(1, 5), 1e-12)
}

func TestBM25IDFStaysPositive(t *testing.T) {
	assert.Greater(t, bm25IDF(3, 3), 0.0)
}

func TestBM25RankingIntuitions(t *testing.T) {
	w := newBM25(3, 2, 8)

	assert.Greater(t, w.score(1, 1), w.score(1, 5), "same tf: shorter doc wins")
	assert.Greater(t, w.score(2, 2), w.score(1, 2), "same length: higher tf wins")

	common := newBM25(3, 3, 8)
	rare := newBM25(3, 1, 8)
	assert.Greater(t, rare.idf, common.idf, "rarer term has higher idf")
}
