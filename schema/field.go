package schema

import "fmt"

// FieldType identifies the kind of value a field holds.
type FieldType int

const (
	// FieldTypeText is string data for full-text or keyword search.
	FieldTypeText FieldType = iota
	FieldTypeInt64
)

// String returns the field type's name; unknown values print as FieldType(N).
func (ft FieldType) String() string {
	switch ft {
	case FieldTypeText:
		return "text"
	case FieldTypeInt64:
		return "int64"
	default:
		return fmt.Sprintf("FieldType(%d)", int(ft))
	}
}

// Field describes one document field: its identity plus which data
// structures the indexer must build from its value.
//
// The ID (not the Name) is what gets written into segment files;
// the schema persisted in meta.json is the only name<->ID decoder key.
type Field struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Type      FieldType `json:"type"`
	Indexed   bool      `json:"indexed,omitempty"`   // build inverted index entries
	Stored    bool      `json:"stored,omitempty"`    // keep raw value in the docstore
	Fast      bool      `json:"fast,omitempty"`      // build columnar fastfield (numbers)
	Positions bool      `json:"positions,omitempty"` // store term positions (phrase queries)
	Tokenizer string    `json:"tokenizer,omitempty"` // "default", "keyword"
}

type FieldOption func(*Field)

func Indexed() FieldOption       { return func(f *Field) { f.Indexed = true } }
func Stored() FieldOption        { return func(f *Field) { f.Stored = true } }
func Fast() FieldOption          { return func(f *Field) { f.Fast = true } }
func WithPositions() FieldOption { return func(f *Field) { f.Positions = true } }

// Keyword makes a text field index as one whole token (no splitting)
// and drops positions.
func Keyword() FieldOption {
	return func(f *Field) { f.Tokenizer = "keyword"; f.Positions = false }
}

func TextField(name string, opts ...FieldOption) Field {
	f := Field{Name: name, Type: FieldTypeText, Tokenizer: "default"}
	for _, o := range opts {
		o(&f)
	}
	return f
}

func Int64Field(name string, opts ...FieldOption) Field {
	f := Field{Name: name, Type: FieldTypeInt64}
	for _, o := range opts {
		o(&f)
	}
	return f
}
