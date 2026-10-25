package schema

type Kind uint8

const (
	Text Kind = iota
	Int64
)

type Value struct {
	Kind Kind
	Str  string // set when Kind == Text
	Num  int64  // set when Kind == Int64
}

type FieldValue struct {
	Field int
	Value Value
}

type Document struct {
	Fields []FieldValue
}

func (d *Document) Text(field int, s string) {
	d.Fields = append(d.Fields, FieldValue{Field: field, Value: Value{Kind: Text, Str: s}})
}

func (d *Document) Int64(field int, n int64) {
	d.Fields = append(d.Fields, FieldValue{Field: field, Value: Value{Kind: Int64, Num: n}})
}

// TODO: FromJSON(s *Schema, r io.Reader) (Document, error) —
// unimplemented; streaming decoder resolving field names to IDs at the boundary.
