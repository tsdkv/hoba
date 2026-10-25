package schema

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

// TODO: FromJSON(s *schema.Schema, r io.Reader) (Document, error) —
// unimplemented; streaming decoder resolving field names to IDs at the boundary.
