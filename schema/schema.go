package schema

import (
	"encoding/json"
	"fmt"
)

type Schema struct {
	Fields []Field `json:"fields"`

	byName map[string]int
}

func NewSchema(fields ...Field) (*Schema, error) {
	s := &Schema{Fields: fields, byName: make(map[string]int, len(fields))}
	for i := range s.Fields {
		s.Fields[i].ID = i
	}
	if err := s.index(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Schema) index() error {
	s.byName = make(map[string]int, len(s.Fields))
	for i, f := range s.Fields {
		// A hand-edited or corrupt meta.json with non-dense IDs must not load silently.
		if f.ID != i {
			return fmt.Errorf("schema: field %q has ID %d, want %d", f.Name, f.ID, i)
		}
		if f.Name == "" {
			return fmt.Errorf("schema: field %d has empty name", i)
		}
		if _, dup := s.byName[f.Name]; dup {
			return fmt.Errorf("schema: duplicate field %q", f.Name)
		}
		if f.Type == FieldTypeText && f.Indexed && f.Tokenizer == "" {
			return fmt.Errorf("schema: text field %q is indexed but has no tokenizer", f.Name)
		}
		if f.Type == FieldTypeInt64 && f.Positions {
			return fmt.Errorf("schema: numeric field %q cannot have positions", f.Name)
		}
		s.byName[f.Name] = i
	}
	return nil
}

func (s *Schema) Field(name string) (Field, bool) {
	i, ok := s.byName[name]
	if !ok {
		return Field{}, false
	}
	return s.Fields[i], true
}

func (s *Schema) FieldByID(id int) (Field, bool) {
	if id < 0 || id >= len(s.Fields) {
		return Field{}, false
	}
	return s.Fields[id], true
}

func (s *Schema) UnmarshalJSON(data []byte) error {
	var raw struct {
		Fields []Field `json:"fields"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	s.Fields = raw.Fields
	return s.index()
}
