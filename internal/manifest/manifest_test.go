package manifest

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tsdkv/hoba/internal/directory"
	"github.com/tsdkv/hoba/schema"
)

func TestLoad(t *testing.T) {
	dir, err := directory.OpenDisk(t.TempDir())
	require.NoError(t, err)
	sch, err := schema.NewSchema(schema.TextField("body", schema.Indexed()))
	require.NoError(t, err)
	m := &Manifest{
		Schema:   *sch,
		Segments: []SegmentMeta{{ID: "segment-1", MaxDoc: 42}},
	}
	require.NoError(t, Commit(dir, m))

	got, err := Load(dir)
	require.NoError(t, err)
	want := &Manifest{
		Version:  manifestVersion,
		Opstamp:  1,
		Schema:   *sch,
		Segments: []SegmentMeta{{ID: "segment-1", MaxDoc: 42}},
	}
	assert.Equal(t, want, got)
}

func TestLoadRejectsInvalidManifest(t *testing.T) {
	tests := []struct {
		name string
		data string
		want string
	}{
		{name: "invalid JSON", data: "{", want: "corrupt manifest"},
		{name: "unsupported version", data: `{"version":2,"schema":{"fields":[]}}`, want: "unsupported manifest version 2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir, err := directory.OpenDisk(t.TempDir())
			require.NoError(t, err)
			require.NoError(t, dir.AtomicWrite(manifestFile, []byte(tt.data)))

			_, err = Load(dir)
			assert.ErrorContains(t, err, tt.want)
		})
	}
}
