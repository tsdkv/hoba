package query

import "math"

// Tantivy/Lucene defaults.
const (
	k1 = 1.2
	b  = 0.75
)

type bm25 struct {
	idf   float64
	avgdl float64
}

func newBM25(numDocs, docFreq int, totalTokens uint64) bm25 {
	var avgdl float64
	if numDocs > 0 {
		avgdl = float64(totalTokens) / float64(numDocs)
	}
	return bm25{idf: bm25IDF(numDocs, docFreq), avgdl: avgdl}
}

// Lucene-style: the classic Robertson IDF goes negative once a term appears
// in more than half the docs, which would invert rankings; +1 keeps it positive.
func bm25IDF(numDocs, docFreq int) float64 {
	return math.Log(1 + (float64(numDocs)-float64(docFreq)+0.5)/(float64(docFreq)+0.5))
}

func (w bm25) score(tf, docLen uint32) float64 {
	f := float64(tf)
	return w.idf * f * (k1 + 1) / (f + k1*(1-b+b*float64(docLen)/w.avgdl))
}

func BM25Score(numDocs, docFreq int, totalTokens uint64, tf, docLen uint32) float64 {
	return newBM25(numDocs, docFreq, totalTokens).score(tf, docLen)
}
