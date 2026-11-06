package fieldnorm

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/tsdkv/hoba/internal/directory"
)

func TestFieldnormTable(t *testing.T) {
	for i := 0; i < 256; i++ {
		id := uint8(i)
		var want uint32
		if id < 24 {
			want = uint32(id)
		} else {
			encoded := id - 24
			bits := uint32(encoded & 7)
			shift := encoded >> 3
			if shift == 0 {
				want = 24 + bits
			} else {
				want = 24 + ((bits | 8) << (shift - 1))
			}
		}
		assert.Equal(t, want, fieldNormsTable[i])
	}
}

type fieldnormTestFile struct {
	data []byte
}

func (f fieldnormTestFile) Len() int      { return len(f.data) }
func (f fieldnormTestFile) Bytes() []byte { return f.data }
func (f fieldnormTestFile) Close() error  { return nil }

func TestFieldnormRoundTrip(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		numFields := rapid.IntRange(0, 20).Draw(rt, "numFields")
		numDocs := rapid.IntRange(0, 20).Draw(rt, "numDocs")

		want := make([][]uint32, numFields)

		var output bytes.Buffer
		writer := NewWriter(numFields)
		for field := range numFields {
			enabled := rapid.Bool().Draw(rt, "enabled")
			if !enabled {
				continue
			}

			writer.Enable(field)
			want[field] = make([]uint32, numDocs)

			for docID := range numDocs {
				present := rapid.Bool().Draw(rt, "present")
				if !present {
					continue
				}

				numTokens := rapid.SampledFrom(fieldNormsTable[:]).Draw(rt, "numTokens")
				writer.Record(uint32(docID), field, numTokens)
				want[field][docID] = numTokens
			}
		}
		require.NoError(rt, writer.Serialize(&output, numDocs))

		file, err := directory.OpenCompositeFile(fieldnormTestFile{data: output.Bytes()})
		require.NoError(rt, err)

		reader := NewReader(file)
		for field := range numFields {
			if want[field] == nil {
				_, err := reader.Fieldnorm(field, 0)
				assert.Error(rt, err)
				continue
			}

			for docID, numTokens := range want[field] {
				got, err := reader.Fieldnorm(field, uint32(docID))
				require.NoError(rt, err)
				assert.Equal(rt, numTokens, got)
			}
		}
	})
}
