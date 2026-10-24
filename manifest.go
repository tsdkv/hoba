package hoba

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/tsdkv/hoba/schema"
)

const (
	manifestFile    = "meta.json"
	manifestVersion = 1
)

// SegmentMeta is the manifest's record of one live segment.
type SegmentMeta struct {
	ID     string `json:"id"`
	MaxDoc uint32 `json:"max_doc"`
}

// Manifest is the only mutable state in the engine: the list of live
// segments plus the schema every segment was written against.
type Manifest struct {
	Version  int           `json:"version"`
	Opstamp  uint64        `json:"opstamp"`
	Schema   schema.Schema `json:"schema"`
	Segments []SegmentMeta `json:"segments"`
}

// CreateIndex initializes a fresh index directory with the given schema.
// Fails if a manifest already exists: schemas are immutable for the
// lifetime of an index.
func CreateIndex(dir string, sch *schema.Schema) (*Manifest, error) {
	if _, err := os.Stat(filepath.Join(dir, manifestFile)); err == nil {
		return nil, fmt.Errorf("index already exists in %s", dir)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	m := &Manifest{Version: manifestVersion, Schema: *sch}
	if err := writeManifest(dir, m); err != nil {
		return nil, err
	}
	return m, nil
}

// OpenIndex loads the manifest of an existing index.
func OpenIndex(dir string) (*Manifest, error) {
	data, err := os.ReadFile(filepath.Join(dir, manifestFile))
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("corrupt manifest: %w", err)
	}
	if m.Version != manifestVersion {
		return nil, fmt.Errorf("unsupported manifest version %d", m.Version)
	}
	return &m, nil
}

// Commit persists a new manifest state atomically, bumping the opstamp.
// Crash at any point leaves either the old or the new manifest, never
// a half-written one.
func Commit(dir string, m *Manifest) error {
	m.Opstamp++
	return writeManifest(dir, m)
}

// writeManifest implements the commit protocol:
// write tmp -> fsync file -> atomic rename -> fsync directory.
func writeManifest(dir string, m *Manifest) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}

	tmp := filepath.Join(dir, manifestFile+".tmp")
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}

	if err := os.Rename(tmp, filepath.Join(dir, manifestFile)); err != nil {
		return err
	}

	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}
