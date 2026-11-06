// Package fieldnorm stores per-document field lengths for BM25 scoring.
// A code is the one-byte encoding of a field length, using Lucene/Tantivy's table.
// BM25 uses the decoded length to normalize term frequency across short and long fields.
package fieldnorm
