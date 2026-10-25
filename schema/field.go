package schema

import "fmt"

type FieldType int

const (
	FieldTypeText FieldType = iota
	FieldTypeInt64
)

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

type Field struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Type      FieldType `json:"type"`
	Indexed   bool      `json:"indexed,omitempty"`
	Stored    bool      `json:"stored,omitempty"`
	Fast      bool      `json:"fast,omitempty"`
	Positions bool      `json:"positions,omitempty"`
	Tokenizer string    `json:"tokenizer,omitempty"`
}

type FieldOption func(*Field)

func Indexed() FieldOption       { return func(f *Field) { f.Indexed = true } }
func Stored() FieldOption        { return func(f *Field) { f.Stored = true } }
func Fast() FieldOption          { return func(f *Field) { f.Fast = true } }
func WithPositions() FieldOption { return func(f *Field) { f.Positions = true } }

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
