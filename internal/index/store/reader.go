package store

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/pierrec/lz4/v4"

	"github.com/tsdkv/hoba/internal/directory"
	"github.com/tsdkv/hoba/schema"
)

const (
	storeFooterSize    = 9
	blockCacheCapacity = 100
)

type Reader struct {
	data        []byte
	checkpoints []checkpoint

	// Uncompressed blocks cache
	cache *lru.Cache[uint64, []byte]
}

func NewReader(file directory.ReadFile) (*Reader, error) {
	data := file.Bytes()
	if len(data) < storeFooterSize {
		return nil, io.ErrUnexpectedEOF
	}

	footerOffset := binary.BigEndian.Uint64(data[len(data)-storeFooterSize : len(data)-1])
	if footerOffset > uint64(len(data)-storeFooterSize) {
		return nil, errors.New("invalid store footer offset")
	}

	if CompressorKind(data[len(data)-1]) != LZ4 {
		return nil, fmt.Errorf("unknown store compressor %d", data[len(data)-1])
	}

	index := data[int(footerOffset) : len(data)-storeFooterSize]
	if len(index)%24 != 0 {
		return nil, errors.New("invalid store checkpoint index length")
	}

	cache, err := lru.New[uint64, []byte](blockCacheCapacity)
	if err != nil {
		return nil, err
	}
	r := &Reader{
		data:        data[:int(footerOffset)],
		checkpoints: make([]checkpoint, 0, len(index)/24),
		cache:       cache,
	}

	var nextDoc uint32
	var nextBlock uint64
	// Checkpoints must cover document IDs and compressed data contiguously.
	for len(index) > 0 {
		c, err := deserializeCheckpoint(index[:24])
		if err != nil {
			return nil, err
		}

		if c.FirstDoc != nextDoc || c.NumDocs == 0 || c.NumDocs > math.MaxUint32-nextDoc {
			return nil, errors.New("invalid store checkpoint document range")
		}

		if c.BlockStart != nextBlock || c.BlockLen == 0 || c.BlockLen > footerOffset-nextBlock {
			return nil, errors.New("invalid store checkpoint block range")
		}

		r.checkpoints = append(r.checkpoints, *c)
		nextDoc += c.NumDocs
		nextBlock += c.BlockLen
		index = index[24:]
	}

	if nextBlock != footerOffset {
		return nil, errors.New("store blocks do not cover the data section")
	}

	return r, nil
}

func (r *Reader) Get(docID uint32) (*schema.Document, error) {
	i := sort.Search(len(r.checkpoints), func(i int) bool {
		c := r.checkpoints[i]
		return docID < c.FirstDoc+c.NumDocs
	})

	if i == len(r.checkpoints) || docID < r.checkpoints[i].FirstDoc {
		return nil, fmt.Errorf("document %d is out of range", docID)
	}

	c := r.checkpoints[i]
	block, err := r.readBlock(c)
	if err != nil {
		return nil, fmt.Errorf("read document %d block: %w", docID, err)
	}

	doc, err := readDocument(block, docID-c.FirstDoc, c.NumDocs)
	if err != nil {
		return nil, fmt.Errorf("read document %d: %w", docID, err)
	}

	return doc, nil
}

func (r *Reader) readBlock(c checkpoint) ([]byte, error) {
	if block, ok := r.cache.Get(c.BlockStart); ok {
		return block, nil
	}

	compressed := r.data[int(c.BlockStart):int(c.BlockStart+c.BlockLen)]
	block, err := decompressBlock(compressed)
	if err != nil {
		return nil, err
	}

	r.cache.Add(c.BlockStart, block)
	return block, nil
}

func decompressBlock(compressed []byte) ([]byte, error) {
	if len(compressed) < 4 {
		return nil, io.ErrUnexpectedEOF
	}

	size := binary.LittleEndian.Uint32(compressed[:4])
	if uint64(size) > uint64(math.MaxInt) {
		return nil, errors.New("decompressed block is too large")
	}

	block := make([]byte, int(size))
	n, err := lz4.UncompressBlock(compressed[4:], block)
	if err != nil {
		return nil, fmt.Errorf("decompress LZ4 block: %w", err)
	}
	if n != len(block) {
		return nil, fmt.Errorf("decompressed block size %d does not match expected size %d", n, len(block))
	}

	return block, nil
}

func readDocument(block []byte, docPos uint32, numDocs uint32) (*schema.Document, error) {
	if docPos >= numDocs || len(block) < 4 {
		return nil, errors.New("invalid document block")
	}

	indexLen := binary.LittleEndian.Uint32(block[len(block)-4:])
	if indexLen != numDocs || uint64(indexLen) > uint64((len(block)-4)/4) {
		return nil, errors.New("invalid document block footer")
	}

	indexStart := len(block) - 4 - int(indexLen)*4

	start := uint64(binary.LittleEndian.Uint32(block[indexStart+int(docPos)*4:]))
	// The last document ends where the offset table starts.
	end := uint64(indexStart)
	if docPos+1 < numDocs {
		end = uint64(binary.LittleEndian.Uint32(block[indexStart+int(docPos+1)*4:]))
	}

	if start > end || end > uint64(indexStart) {
		return nil, errors.New("invalid document offset")
	}

	return schema.DeserializeDocument(block[int(start):int(end)])
}
