package manifest

import (
	"encoding/json"
	"fmt"

	"github.com/tsdkv/hoba/internal/directory"
	"github.com/tsdkv/hoba/schema"
)

const (
	manifestFile    = "meta.json"
	manifestVersion = 1
)

type SegmentMeta struct {
	ID     string `json:"id"`
	MaxDoc uint32 `json:"max_doc"`
}

type Manifest struct {
	Version  int           `json:"version"`
	Opstamp  uint64        `json:"opstamp"`
	Schema   schema.Schema `json:"schema"`
	Segments []SegmentMeta `json:"segments"`
}

func Load(dir directory.Directory) (*Manifest, error) {
	data, err := dir.ReadAll(manifestFile)
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

func Commit(dir directory.Directory, m *Manifest) error {
	m.Opstamp++
	m.Version = manifestVersion
	return writeManifest(dir, m)
}

// A crash mid-write must never leave a half-written manifest.
func writeManifest(dir directory.Directory, m *Manifest) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}

	return dir.AtomicWrite(manifestFile, data)
}
