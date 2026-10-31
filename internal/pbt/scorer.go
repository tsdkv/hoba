package pbt

import (
	"math"
	"slices"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/tsdkv/hoba/internal/index"
	"github.com/tsdkv/hoba/internal/query"
)

type Hit struct {
	Doc   index.DocID
	Score float64
}

// CheckScorer walks a fresh scorer linearly and verifies the DocSet
// contract: strictly increasing docs, repeatable scores, sticky Terminated.
// Fresh scorers seeking to every hit and to random targets must reproduce
// the walk's docs and scores. Returns the walk's hits; nil scorer means nil.
func CheckScorer(t *rapid.T, numDocs int, newScorer func() query.Scorer) []Hit {
	sc := newScorer()
	if sc == nil {
		return nil
	}

	var hits []Hit
	prev := -1
	for d := sc.Doc(); d != index.Terminated; d = sc.Advance() {
		require.Less(t, int(d), numDocs)
		require.Greater(t, int(d), prev, "docIDs must increase")
		require.False(t, math.IsNaN(sc.Score()), "NaN score at doc %d", d)
		require.Equal(t, sc.Score(), sc.Score(), "Score not repeatable at doc %d", d)
		hits = append(hits, Hit{Doc: d, Score: sc.Score()})
		prev = int(d)
	}

	require.Equal(t, index.Terminated, sc.Doc(), "Terminated not sticky")
	require.Equal(t, index.Terminated, sc.Advance(), "Advance past Terminated")
	require.Equal(t, index.Terminated, sc.Seek(index.DocID(numDocs)), "Seek past end")

	fresh := newScorer()
	for _, h := range hits {
		require.Equal(t, h.Doc, fresh.Seek(h.Doc), "Seek to hit %d", h.Doc)
		require.Equal(t, h.Score, fresh.Score(), "score after Seek to %d", h.Doc)
	}

	targets := []int{0, numDocs}
	for range 3 {
		targets = append(targets, dieRoll(numDocs+1).Draw(t, "seekTarget"))
	}
	slices.Sort(targets) // Seek never moves backwards

	fresh = newScorer()
	for _, tgt := range targets {
		want := index.Terminated
		var wantScore float64
		for _, h := range hits {
			if h.Doc >= index.DocID(tgt) {
				want, wantScore = h.Doc, h.Score
				break
			}
		}

		require.Equal(t, want, fresh.Seek(index.DocID(tgt)), "Seek(%d)", tgt)
		if want != index.Terminated {
			require.Equal(t, wantScore, fresh.Score(), "score after Seek(%d)", tgt)
		}
	}

	return hits
}
