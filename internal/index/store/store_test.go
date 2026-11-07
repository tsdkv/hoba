package store

import (
	"bytes"
	"encoding/binary"
	"io"
	"math"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/tsdkv/hoba/schema"
)

type storeTestFile struct {
	data []byte
}

func (f storeTestFile) Len() int      { return len(f.data) }
func (f storeTestFile) Bytes() []byte { return f.data }
func (f storeTestFile) Close() error  { return nil }

type failingStoreWriter struct {
	remaining int
	failed    bool
}

func (w *failingStoreWriter) Write(p []byte) (int, error) {
	if w.failed {
		return len(p), nil
	}
	if len(p) <= w.remaining {
		w.remaining -= len(p)
		return len(p), nil
	}
	n := w.remaining
	w.remaining = 0
	w.failed = true
	return n, io.ErrClosedPipe
}

func storeDocumentGenerator() *rapid.Generator[schema.Document] {
	return rapid.Custom(func(rt *rapid.T) schema.Document {
		textLen := rapid.SampledFrom([]int{0, 1, 7, 64, 1024, maxBlockSize + 1}).Draw(rt, "textLength")
		suffix := rapid.String().Draw(rt, "textSuffix")
		number := rapid.Int64().Draw(rt, "number")

		var doc schema.Document
		doc.Text(0, strings.Repeat("x", textLen)+suffix)
		doc.Int64(1, number)
		return doc
	})
}

func smallStoreDocumentGenerator() *rapid.Generator[schema.Document] {
	return rapid.Custom(func(rt *rapid.T) schema.Document {
		var doc schema.Document
		doc.Text(0, rapid.String().Draw(rt, "text"))
		doc.Int64(1, rapid.Int64().Draw(rt, "number"))
		return doc
	})
}

func storeTestSchema(t require.TestingT) *schema.Schema {
	s, err := schema.NewSchema(
		schema.TextField("text", schema.Stored()),
		schema.Int64Field("number", schema.Stored()),
	)
	require.NoError(t, err)
	return s
}

func writeTestStore(t require.TestingT, s *schema.Schema, docs []schema.Document) []byte {
	var output bytes.Buffer
	writer := NewWriter(s, &output)
	for _, doc := range docs {
		require.NoError(t, writer.Store(doc))
	}
	require.NoError(t, writer.Finalize())
	return output.Bytes()
}

func openTestStoreWithSchema(t require.TestingT, s *schema.Schema, docs []schema.Document) *Reader {
	reader, err := NewReader(storeTestFile{data: writeTestStore(t, s, docs)})
	require.NoError(t, err)
	return reader
}

func openTestStore(t require.TestingT, docs []schema.Document) *Reader {
	return openTestStoreWithSchema(t, storeTestSchema(t), docs)
}

func assertStoredDocument(t require.TestingT, reader *Reader, docs []schema.Document, docID int) {
	got, err := reader.Get(uint32(docID))
	require.NoError(t, err)
	assert.Equal(t, docs[docID], *got)
}

func TestStoreRoundTrip(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		numDocs := rapid.SampledFrom([]int{0, 1, 2, 10, 100, 300}).Draw(rt, "numDocs")
		docs := rapid.SliceOfN(storeDocumentGenerator(), numDocs, numDocs).Draw(rt, "docs")
		reader := openTestStore(rt, docs)

		for i := range docs {
			assertStoredDocument(rt, reader, docs, i)
		}
		for i := len(docs) - 1; i >= 0; i-- {
			assertStoredDocument(rt, reader, docs, i)
		}

		_, err := reader.Get(uint32(len(docs)))
		assert.Error(rt, err)
	})
}

func TestStoreRandomAccess(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		numDocs := rapid.SampledFrom([]int{1, 2, 10, 100, 300}).Draw(rt, "numDocs")
		docs := rapid.SliceOfN(storeDocumentGenerator(), numDocs, numDocs).Draw(rt, "docs")
		reader := openTestStore(rt, docs)

		docIDs := make([]int, len(docs))
		for i := range docIDs {
			docIDs[i] = i
		}
		readOrder := rapid.Permutation(docIDs).Draw(rt, "readOrder")
		for _, docID := range readOrder {
			assertStoredDocument(rt, reader, docs, docID)
		}

		numRepeats := rapid.SampledFrom([]int{1, 2, 5, 20}).Draw(rt, "numRepeats")
		for range numRepeats {
			docID := rapid.SampledFrom(docIDs).Draw(rt, "repeatDocID")
			assertStoredDocument(rt, reader, docs, docID)
		}
	})
}

func TestStoreCacheEviction(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		numDocs := rapid.IntRange(blockCacheCapacity+1, blockCacheCapacity+20).Draw(rt, "numDocs")
		docGen := rapid.Custom(func(rt *rapid.T) schema.Document {
			var doc schema.Document
			doc.Text(0, strings.Repeat("x", maxBlockSize+1))
			doc.Int64(1, rapid.Int64().Draw(rt, "number"))
			return doc
		})
		docs := rapid.SliceOfN(docGen, numDocs, numDocs).Draw(rt, "docs")
		reader := openTestStore(rt, docs)

		docIDs := make([]int, len(docs))
		for i := range docIDs {
			docIDs[i] = i
		}
		readOrder := rapid.Permutation(docIDs).Draw(rt, "readOrder")
		for _, docID := range readOrder {
			assertStoredDocument(rt, reader, docs, docID)
		}

		assert.Equal(rt, blockCacheCapacity, reader.cache.Len())
	})
}

func TestStoreStoredFieldsOnly(t *testing.T) {
	s, err := schema.NewSchema(
		schema.TextField("text", schema.Stored()),
		schema.Int64Field("ignored"),
		schema.Int64Field("number", schema.Stored()),
	)
	require.NoError(t, err)

	rapid.Check(t, func(rt *rapid.T) {
		numDocs := rapid.IntRange(0, 30).Draw(rt, "numDocs")
		docs := make([]schema.Document, numDocs)
		want := make([]schema.Document, numDocs)
		for i := range numDocs {
			want[i].Fields = []schema.FieldValue{}
			if rapid.Bool().Draw(rt, "textPresent") {
				text := rapid.String().Draw(rt, "text")
				docs[i].Text(0, text)
				want[i].Text(0, text)
			}
			docs[i].Int64(1, rapid.Int64().Draw(rt, "ignored"))
			if rapid.Bool().Draw(rt, "numberPresent") {
				number := rapid.Int64().Draw(rt, "number")
				docs[i].Int64(2, number)
				want[i].Int64(2, number)
			}
		}

		reader := openTestStoreWithSchema(rt, s, docs)
		for i := range want {
			assertStoredDocument(rt, reader, want, i)
		}
	})
}

func TestStoreWriterFailure(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		numDocs := rapid.SampledFrom([]int{1, 2, 10}).Draw(rt, "numDocs")
		docs := rapid.SliceOfN(smallStoreDocumentGenerator(), numDocs, numDocs).Draw(rt, "docs")
		s := storeTestSchema(rt)
		valid := writeTestStore(rt, s, docs)
		footerOffset := binary.BigEndian.Uint64(valid[len(valid)-storeFooterSize : len(valid)-1])
		failurePoints := []int{0, len(valid) / 2, int(footerOffset) + 1, len(valid) - 1}

		for _, failAfter := range failurePoints {
			dst := &failingStoreWriter{remaining: failAfter}
			writer := NewWriter(s, dst)
			var writeErr error
			for _, doc := range docs {
				if err := writer.Store(doc); err != nil {
					writeErr = err
					break
				}
			}
			if writeErr == nil {
				writeErr = writer.Finalize()
			}
			assert.ErrorIs(rt, writeErr, io.ErrClosedPipe)
		}
	})
}

func TestStoreRejectsCorruption(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		docs := rapid.SliceOfN(smallStoreDocumentGenerator(), 3, 3).Draw(rt, "docs")
		data := writeTestStore(rt, storeTestSchema(rt), docs)
		footerOffset := binary.BigEndian.Uint64(data[len(data)-storeFooterSize : len(data)-1])
		checkpointOffset := int(footerOffset)
		blockLen := binary.LittleEndian.Uint64(data[checkpointOffset+16:])

		truncateAt := rapid.IntRange(0, len(data)-1).Draw(rt, "truncateAt")
		truncated := bytes.Clone(data[:truncateAt])

		badCompressor := bytes.Clone(data)
		badCompressor[len(badCompressor)-1] = math.MaxUint8

		badFooterOffset := bytes.Clone(data)
		binary.BigEndian.PutUint64(badFooterOffset[len(badFooterOffset)-storeFooterSize:len(badFooterOffset)-1], math.MaxUint64)

		badCheckpointIndex := make([]byte, 0, len(data)+1)
		badCheckpointIndex = append(badCheckpointIndex, data[:len(data)-storeFooterSize]...)
		badCheckpointIndex = append(badCheckpointIndex, 0)
		badCheckpointIndex = append(badCheckpointIndex, data[len(data)-storeFooterSize:]...)

		badCheckpointDocs := bytes.Clone(data)
		binary.LittleEndian.PutUint32(badCheckpointDocs[checkpointOffset+4:], 0)

		badCheckpointBlock := bytes.Clone(data)
		binary.LittleEndian.PutUint64(badCheckpointBlock[checkpointOffset+16:], 0)

		badUncompressedSize := bytes.Clone(data)
		size := binary.LittleEndian.Uint32(badUncompressedSize[:4])
		binary.LittleEndian.PutUint32(badUncompressedSize[:4], size+1)

		mutations := []struct {
			name string
			data []byte
		}{
			{name: "truncate", data: truncated},
			{name: "compressor", data: badCompressor},
			{name: "footer offset", data: badFooterOffset},
			{name: "checkpoint index", data: badCheckpointIndex},
			{name: "checkpoint documents", data: badCheckpointDocs},
			{name: "checkpoint block", data: badCheckpointBlock},
			{name: "uncompressed size", data: badUncompressedSize},
		}
		for _, mutation := range mutations {
			reader, err := NewReader(storeTestFile{data: mutation.data})
			if err == nil {
				for range docs {
					_, err = reader.Get(0)
					if err != nil {
						break
					}
				}
			}
			assert.Error(rt, err, mutation.name)
		}

		_, err := decompressBlock(badUncompressedSize[:int(blockLen)])
		assert.Error(rt, err)
	})
}

func TestReadDocumentRejectsInvalidOffset(t *testing.T) {
	block := make([]byte, 8)
	binary.LittleEndian.PutUint32(block[:4], 1)
	binary.LittleEndian.PutUint32(block[4:], 1)

	_, err := readDocument(block, 0, 1)
	assert.Error(t, err)
}

func TestStoreConcurrentReads(t *testing.T) {
	docs := make([]schema.Document, 200)
	for i := range docs {
		docs[i].Text(0, strings.Repeat("x", 1024)+strconv.Itoa(i))
		docs[i].Int64(1, int64(i))
	}
	reader := openTestStore(t, docs)

	var wg sync.WaitGroup
	for worker := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				docID := (worker*17 + i) % len(docs)
				got, err := reader.Get(uint32(docID))
				if !assert.NoError(t, err) {
					return
				}
				assert.Equal(t, docs[docID], *got)
			}
		}()
	}
	wg.Wait()
}
