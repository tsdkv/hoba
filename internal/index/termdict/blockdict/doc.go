// Package blockdict implements a temporary immutable term-to-ordinal dictionary
// using front-coded blocks and a sparse block index. It will be replaced by an
// FST-based implementation.
//
// Writers must provide unique terms in lexicographic order. Lookup binary-searches
// block first terms, then scans at most 31 terms in the selected block.
package blockdict
