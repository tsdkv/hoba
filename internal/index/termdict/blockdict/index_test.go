package blockdict

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func TestIndexEntryLayout(t *testing.T) {
	entry := indexEntry{
		blockOffset: 0x0102030405060708,
		termOffset:  0x11121314,
		termLen:     0x15161718,
	}
	var buf bytes.Buffer
	require.NoError(t, entry.Serialize(&buf))
	want := []byte{
		0x08, 0x07, 0x06, 0x05, 0x04, 0x03, 0x02, 0x01,
		0x14, 0x13, 0x12, 0x11,
		0x18, 0x17, 0x16, 0x15,
	}
	assert.Equal(t, want, buf.Bytes())

	got, err := deserializeIndexEntry(buf.Bytes())
	require.NoError(t, err)
	assert.Equal(t, entry, *got)
}

func TestDeserializeIndexEntryWrongSize(t *testing.T) {
	for _, size := range []int{0, indexEntrySize - 1, indexEntrySize + 1} {
		got, err := deserializeIndexEntry(make([]byte, size))
		assert.Nil(t, got)
		assert.EqualError(t, err, "wrong buffer size")
	}
}

func TestIndexEntryRoundTrip(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		want := indexEntry{
			blockOffset: rapid.Uint64().Draw(rt, "blockOffset"),
			termOffset:  rapid.Uint32().Draw(rt, "termOffset"),
			termLen:     rapid.Uint32().Draw(rt, "termLen"),
		}
		var buf bytes.Buffer
		require.NoError(rt, want.Serialize(&buf))
		require.Len(rt, buf.Bytes(), indexEntrySize)

		got, err := deserializeIndexEntry(buf.Bytes())
		require.NoError(rt, err)
		assert.Equal(rt, want, *got)
	})
}
