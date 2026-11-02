package directory

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/edsrzf/mmap-go"
)

const tmpPrefix = ".hoba-tmp-"

type Disk struct {
	dirPath string
}

func OpenDisk(dir string) (*Disk, error) {
	st, err := os.Stat(dir)
	if err != nil {
		return nil, err
	}
	if !st.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", dir)
	}
	return &Disk{dirPath: dir}, nil
}

func (d *Disk) Create(name string) (WriteFile, error) {
	if err := validateName(name); err != nil {
		return nil, err
	}
	path := filepath.Join(d.dirPath, name)

	if _, err := os.Lstat(path); err == nil {
		return nil, &os.PathError{Op: "create", Path: path, Err: fs.ErrExist}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}

	f, err := os.CreateTemp(d.dirPath, tmpPrefix+"*")
	if err != nil {
		return nil, err
	}

	return &pendingFile{
		f:       f,
		tmpPath: f.Name(),
		path:    path,
	}, nil
}

func (d *Disk) Open(name string) (_ ReadFile, rerr error) {
	if err := validateName(name); err != nil {
		return nil, err
	}

	f, err := os.Open(filepath.Join(d.dirPath, name))
	if err != nil {
		return nil, err
	}

	defer func() {
		if rerr != nil {
			rerr = errors.Join(rerr, f.Close())
		}
	}()

	st, err := f.Stat()
	if err != nil {
		return nil, err
	}

	// can't mmap empty file
	if st.Size() == 0 {
		return &mappedFile{
			bytes: []byte{},
			f:     f,
		}, nil
	}

	mm, err := mmap.Map(f, mmap.RDONLY, 0)
	if err != nil {
		return nil, err
	}

	return &mappedFile{
		bytes: mm,
		f:     f,
	}, nil
}

func (d *Disk) ReadAll(name string) ([]byte, error) {
	if err := validateName(name); err != nil {
		return nil, err
	}

	return os.ReadFile(filepath.Join(d.dirPath, name))
}

func (d *Disk) AtomicWrite(name string, data []byte) (rerr error) {
	if err := validateName(name); err != nil {
		return err
	}
	filePath := filepath.Join(d.dirPath, name)

	f, err := os.CreateTemp(d.dirPath, tmpPrefix+"*")
	if err != nil {
		return err
	}
	tmpPath := f.Name()
	defer func() {
		closeErr := f.Close()
		if errors.Is(closeErr, os.ErrClosed) {
			closeErr = nil
		}
		rerr = errors.Join(rerr, closeErr, removeTemp(tmpPath))
	}()

	if _, err := f.Write(data); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, filePath); err != nil {
		return err
	}
	return d.SyncDir()
}

func (d *Disk) List() ([]string, error) {
	entries, err := os.ReadDir(d.dirPath)
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || strings.HasPrefix(entry.Name(), tmpPrefix) {
			continue
		}
		names = append(names, entry.Name())
	}

	return names, nil
}

func (d *Disk) SyncDir() error {
	dir, err := os.Open(d.dirPath)
	if err != nil {
		return err
	}

	syncErr := dir.Sync()
	closeErr := dir.Close()
	return errors.Join(syncErr, closeErr)
}

func validateName(name string) error {
	if name == "" || name == "." || name == ".." || filepath.Base(name) != name || strings.HasPrefix(name, tmpPrefix) {
		return fmt.Errorf("invalid file name %q: %w", name, fs.ErrInvalid)
	}

	return nil
}

func removeTemp(path string) error {
	err := os.Remove(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}

	return err
}
