package blockdict

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sort"
)

const footerSize = 12

var errInvalidDictionary = errors.New("invalid block dictionary")

type Reader struct {
	data        []byte
	entries     []indexEntry
	firsts      []byte
	indexOffset uint64
}

func NewReader(data []byte) (*Reader, error) {
	if len(data) < footerSize {
		return nil, io.ErrUnexpectedEOF
	}

	footerStart := uint64(len(data) - footerSize)

	numBlocks := binary.LittleEndian.Uint32(data[len(data)-12:])
	indexOffset := binary.LittleEndian.Uint64(data[len(data)-8:])

	indexBytes := uint64(numBlocks) * indexEntrySize
	if indexOffset > footerStart || indexBytes > footerStart-indexOffset {
		return nil, fmt.Errorf("%w: index bounds", errInvalidDictionary)
	}

	firstsOffset := indexOffset + indexBytes
	firsts := data[firstsOffset:footerStart]
	entries := make([]indexEntry, int(numBlocks))

	for i := range entries {
		start := indexOffset + uint64(i)*indexEntrySize
		entry, err := deserializeIndexEntry(data[start : start+indexEntrySize])
		if err != nil {
			return nil, err
		}

		termStart := uint64(entry.termOffset)
		if termStart > uint64(len(firsts)) || uint64(entry.termLen) > uint64(len(firsts))-termStart {
			return nil, fmt.Errorf("%w: first term bounds", errInvalidDictionary)
		}

		if entry.blockOffset > indexOffset || (i == 0 && entry.blockOffset != 0) || (i > 0 && entry.blockOffset <= entries[i-1].blockOffset) {
			return nil, fmt.Errorf("%w: block bounds", errInvalidDictionary)
		}

		entries[i] = *entry
	}

	return &Reader{
		data:        data,
		entries:     entries,
		firsts:      firsts,
		indexOffset: indexOffset,
	}, nil
}

func (r *Reader) firstTerm(i int) []byte {
	entry := r.entries[i]
	start := int(entry.termOffset)
	return r.firsts[start : start+int(entry.termLen)]
}

func (r *Reader) Get(term []byte) (uint32, bool, error) {
	blockIndex := sort.Search(len(r.entries), func(i int) bool {
		return bytes.Compare(r.firstTerm(i), term) > 0
	}) - 1

	if blockIndex < 0 {
		return 0, false, nil
	}

	baseOrd := uint32(blockIndex) * (maxBlockTerms + 1)
	if bytes.Equal(r.firstTerm(blockIndex), term) {
		return baseOrd, true, nil
	}

	entry := r.entries[blockIndex]
	blockEnd := r.indexOffset
	if blockIndex+1 < len(r.entries) {
		blockEnd = r.entries[blockIndex+1].blockOffset
	}

	if entry.blockOffset == blockEnd {
		return 0, false, nil
	}

	block, err := newBlockReader(r.data[entry.blockOffset:blockEnd])
	if err != nil {
		return 0, false, err
	}

	innerOrd, ok, err := block.Get(term)
	if err != nil || !ok {
		return 0, false, err
	}

	return baseOrd + 1 + innerOrd, true, nil
}
