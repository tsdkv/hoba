package directory

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

type compositeTestFile struct {
	data []byte
}

func (f compositeTestFile) Len() int      { return len(f.data) }
func (f compositeTestFile) Bytes() []byte { return f.data }
func (f compositeTestFile) Close() error  { return nil }

func TestCompositeFileRoundTrip(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		numFields := rapid.IntRange(0, 20).Draw(rt, "numFields")

		want := make(map[uint32][]byte, numFields)

		var output bytes.Buffer
		writer := NewCompositeWriter(&output)
		for field := range numFields {
			size := rapid.IntRange(0, 300).Draw(rt, "payloadSize")
			payload := rapid.SliceOfN(rapid.Byte(), size, size).Draw(rt, "payload")

			want[uint32(field)] = payload

			w := writer.ForField(uint32(field))
			written := 0
			for written < size {
				toWrite := rapid.IntRange(1, size-written).Draw(rt, "written")

				_, err := w.Write(payload[written : written+toWrite])
				require.NoError(rt, err)

				written += toWrite
			}
		}
		require.NoError(rt, writer.Finalize())

		reader, err := OpenCompositeFile(compositeTestFile{data: output.Bytes()})
		require.NoError(rt, err)

		for field, payload := range want {
			got, err := reader.Field(field)
			require.NoError(rt, err)
			assert.Equal(rt, payload, got)
		}

		_, err = reader.Field(uint32(numFields))
		assert.Error(rt, err)
	})
}
