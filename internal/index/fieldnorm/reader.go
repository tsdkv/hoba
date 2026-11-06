package fieldnorm

import "github.com/tsdkv/hoba/internal/directory"

type Reader struct {
	file *directory.CompositeReader
}

func NewReader(file *directory.CompositeReader) *Reader {
	return &Reader{file}
}

func (r *Reader) Fieldnorm(field int, docID uint32) (uint32, error) {
	data, err := r.file.Field(uint32(field))
	if err != nil {
		return 0, err
	}

	return FieldnormToID(data[docID]), nil
}
