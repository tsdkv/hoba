package directory

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAtomicWriteCleansUpAfterRenameFailure(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenDisk(dir)
	require.NoError(t, err)
	require.NoError(t, os.Mkdir(filepath.Join(dir, "meta"), 0o755))

	assert.Error(t, d.AtomicWrite("meta", []byte("new")))

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)

	assert.Len(t, entries, 1)
}

func TestPendingFileAbort(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenDisk(dir)
	require.NoError(t, err)

	w, err := d.Create("segment")
	require.NoError(t, err)

	_, err = w.Write([]byte("partial"))
	require.NoError(t, err)

	names, err := d.List()
	require.NoError(t, err)

	assert.Empty(t, names)

	require.NoError(t, w.Abort())

	_, err = d.ReadAll("segment")
	assert.ErrorIs(t, err, fs.ErrNotExist)

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)

	assert.Empty(t, entries)
}

func TestPendingFileCloseFailureCleansUp(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenDisk(dir)
	require.NoError(t, err)

	w, err := d.Create("segment")
	require.NoError(t, err)

	_, err = w.Write([]byte("new"))
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(filepath.Join(dir, "segment"), []byte("existing"), 0o644))

	err = w.Commit()
	assert.ErrorIs(t, err, fs.ErrExist)

	data, err := os.ReadFile(filepath.Join(dir, "segment"))
	require.NoError(t, err)

	assert.Equal(t, []byte("existing"), data)

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)

	assert.Len(t, entries, 1)
	assert.NoError(t, w.Abort())
}

func TestRejectInvalidNames(t *testing.T) {
	d, err := OpenDisk(t.TempDir())
	require.NoError(t, err)

	for _, name := range []string{"", ".", "..", "../escape", "nested/file", tmpPrefix + "file"} {
		_, err := d.Create(name)
		assert.ErrorIs(t, err, fs.ErrInvalid, "Create(%q)", name)

		_, err = d.Open(name)
		assert.ErrorIs(t, err, fs.ErrInvalid, "Open(%q)", name)

		_, err = d.ReadAll(name)
		assert.ErrorIs(t, err, fs.ErrInvalid, "ReadAll(%q)", name)

		err = d.AtomicWrite(name, nil)
		assert.ErrorIs(t, err, fs.ErrInvalid, "AtomicWrite(%q)", name)
	}
}
