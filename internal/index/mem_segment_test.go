package index

import (
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/tsdkv/hoba/schema"
	"github.com/tsdkv/hoba/tokenizer"
)

// Regression lock: lens must stay dense ([field][doc]) even when docs omit
// the field — the sparse-append version panicked here.
func TestMemSegmentDocLen(t *testing.T) {
	tm := tokenizer.NewManager()
	s, err := schema.NewSchema(schema.TextField("body", schema.Indexed()))
	require.NoError(t, err)

	m := NewMemSegment(s, tm)

	var empty schema.Document
	_, err = m.Add(empty)
	require.NoError(t, err)

	var d schema.Document
	d.Text(0, "a b c")
	_, err = m.Add(d)
	require.NoError(t, err)

	_, err = m.Add(empty)
	require.NoError(t, err)

	assert.Equal(t, uint32(0), m.DocLen(0, 0))
	assert.Equal(t, uint32(3), m.DocLen(1, 0))
	assert.Equal(t, uint32(0), m.DocLen(2, 0))
	assert.Equal(t, uint64(3), m.TotalTokens(0))
}

// Model-based oracle: index random docs into a MemSegment and into plain Go
// maps side by side, then require agreement on every observable — postings,
// freqs, seek, field lengths, token totals.
func TestMemSegmentMatchesModel(t *testing.T) {
	tm := tokenizer.NewManager()
	s, err := schema.NewSchema(
		schema.TextField("title", schema.Indexed()),
		schema.TextField("body", schema.Indexed(), schema.WithPositions()),
		schema.Int64Field("num", schema.Indexed()),
		schema.TextField("note", schema.Stored()),
	)
	require.NoError(t, err)

	// Small vocab on purpose: random strings would never collide, and
	// collisions are what exercise freq>1 and cross-doc postings. Words are
	// joined with single spaces so the model can re-tokenize independently
	// with strings.Fields instead of reusing the analyzer under test.
	vocab := []string{"a", "an", "bb", "cat", "dog", "eel", "fox"}

	rapid.Check(t, func(rt *rapid.T) {
		n := rapid.IntRange(0, 30).Draw(rt, "numDocs")

		m := NewMemSegment(s, tm)

		freq := map[int]map[string]map[DocID]uint32{}
		lens := map[int]map[DocID]uint32{}
		totals := map[int]uint64{}
		var noteWords []string

		bump := func(field int, term string, doc DocID) {
			if freq[field] == nil {
				freq[field] = map[string]map[DocID]uint32{}
			}
			if freq[field][term] == nil {
				freq[field][term] = map[DocID]uint32{}
			}
			freq[field][term][doc]++
		}

		for i := 0; i < n; i++ {
			var doc schema.Document
			for fid, f := range s.Fields {
				if !rapid.Bool().Draw(rt, "has_"+f.Name) {
					continue
				}
				switch f.Type {
				case schema.FieldTypeText:
					nw := rapid.IntRange(0, 5).Draw(rt, "len_"+f.Name)
					words := rapid.SliceOfN(rapid.SampledFrom(vocab), nw, nw).Draw(rt, "text_"+f.Name)
					doc.Text(fid, strings.Join(words, " "))
				case schema.FieldTypeInt64:
					doc.Int64(fid, int64(rapid.IntRange(-3, 3).Draw(rt, "val_"+f.Name)))
				}
			}

			docID, err := m.Add(doc)
			require.NoError(rt, err)
			require.Equal(rt, DocID(i), docID)

			for _, df := range doc.Fields {
				f := s.Fields[df.Field]
				if !f.Indexed {
					noteWords = append(noteWords, strings.Fields(df.Value.Str)...)
					continue
				}
				switch f.Type {
				case schema.FieldTypeInt64:
					bump(f.ID, EncodeInt64(df.Value.Num), docID)
				case schema.FieldTypeText:
					words := strings.Fields(df.Value.Str)
					for _, w := range words {
						bump(f.ID, w, docID)
					}
					if lens[f.ID] == nil {
						lens[f.ID] = map[DocID]uint32{}
					}
					lens[f.ID][docID] = uint32(len(words))
					totals[f.ID] += uint64(len(words))
				}
			}
		}

		require.Equal(rt, n, m.NumDocs())

		for _, fid := range slices.Sorted(maps.Keys(freq)) {
			for _, term := range slices.Sorted(maps.Keys(freq[fid])) {
				want := freq[fid][term]
				p, ok := m.Postings(fid, term)
				require.True(rt, ok, "field %d term %q", fid, term)

				got := map[DocID]uint32{}
				prev, started := DocID(0), false
				for d := p.Advance(); d != Terminated; d = p.Advance() {
					if started {
						require.Greater(rt, d, prev, "postings not sorted")
					}
					started, prev = true, d
					got[d] = p.Freq()
				}
				require.Equal(rt, want, got, "field %d term %q", fid, term)

				// Seek target goes past n so the Terminated outcome stays
				// reachable; a fresh iterator because Seek never moves back.
				target := rapid.IntRange(0, n+2).Draw(rt, "seekTarget")
				wantSeek := Terminated
				for _, d := range slices.Sorted(maps.Keys(want)) {
					if d >= DocID(target) {
						wantSeek = d
						break
					}
				}
				fresh, _ := m.Postings(fid, term)
				require.Equal(rt, wantSeek, fresh.Seek(DocID(target)),
					"field %d term %q seek %d", fid, term, target)
			}
		}

		for _, fid := range []int{0, 1} {
			for d := 0; d < n; d++ {
				require.Equal(rt, lens[fid][DocID(d)], m.DocLen(DocID(d), fid),
					"field %d doc %d", fid, d)
			}
			require.Equal(rt, totals[fid], m.TotalTokens(fid), "field %d", fid)
		}

		_, ok := m.Postings(0, "zz-absent")
		require.False(rt, ok)
		for _, w := range noteWords {
			_, ok := m.Postings(3, w)
			require.False(rt, ok, "unindexed field leaked term %q", w)
		}
	})
}
