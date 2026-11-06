package fieldnorm

import (
	"io"

	"github.com/tsdkv/hoba/internal/directory"
)

type Writer struct {
	// Indexed by field ID, then document ID.
	// Nil means this field does not have fieldnorms.
	fields [][]byte
}

func NewWriter(numFeld int) *Writer {
	return &Writer{
		fields: make([][]byte, numFeld),
	}
}

func (w *Writer) Enable(fieldID int) {
	w.fields[fieldID] = make([]byte, 0)
}

// Document IDs must increase between calls for the same field.
func (w *Writer) Record(docID uint32, fieldID int, numTokens uint32) {
	f := w.fields[fieldID]
	if f == nil {
		return
	}
	if int(docID) < len(f) {
		panic("fieldnorm recorded twice")
	}

	// Documents that omitted this field have length zero.
	for len(f) < int(docID) {
		f = append(f, 0)
	}
	w.fields[fieldID] = append(f, FieldnormFromID(numTokens))
}

func (w *Writer) Serialize(dst io.Writer, numDocs int) error {
	cFile := directory.NewCompositeWriter(dst)

	for field, values := range w.fields {
		if values == nil {
			continue
		}

		w := cFile.ForField(uint32(field))

		if _, err := w.Write(values); err != nil {
			return err
		}

		zeroes := make([]byte, numDocs-len(values))
		if _, err := w.Write(zeroes); err != nil {
			return err
		}
	}

	return cFile.Finalize()
}
