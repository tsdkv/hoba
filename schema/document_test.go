package schema

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidate(t *testing.T) {
	s, err := NewSchema(TextField("title"), Int64Field("count"))
	require.NoError(t, err)

	tests := []struct {
		name    string
		doc     Document
		wantErr string
	}{
		{
			name: "valid",
			doc: Document{Fields: []FieldValue{
				{Field: 0, Value: Value{Kind: Text, Str: "hello"}},
				{Field: 1, Value: Value{Kind: Int64, Num: 2}},
			}},
		},
		{
			name:    "unknown field",
			doc:     Document{Fields: []FieldValue{{Field: 2, Value: Value{Kind: Text}}}},
			wantErr: "schema: unknown field ID: 2",
		},
		{
			name: "duplicate field",
			doc: Document{Fields: []FieldValue{
				{Field: 0, Value: Value{Kind: Text}},
				{Field: 0, Value: Value{Kind: Text}},
			}},
			wantErr: `schema: duplicate field "title"`,
		},
		{
			name:    "text field with int64 value",
			doc:     Document{Fields: []FieldValue{{Field: 0, Value: Value{Kind: Int64}}}},
			wantErr: `schema: field "title": not a text value`,
		},
		{
			name:    "int64 field with text value",
			doc:     Document{Fields: []FieldValue{{Field: 1, Value: Value{Kind: Text}}}},
			wantErr: `schema: field "count": not an int64 value`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := s.Validate(tc.doc)
			if tc.wantErr == "" {
				require.NoError(t, err)
			} else {
				assert.EqualError(t, err, tc.wantErr)
			}
		})
	}

	s.Fields[0].Type = FieldType(42)
	err = s.Validate(Document{Fields: []FieldValue{{Field: 0, Value: Value{Kind: Text}}}})
	assert.EqualError(t, err, "schema: unexpected field type: FieldType(42)")
}

func TestBuildersAppendInOrder(t *testing.T) {
	var d Document
	d.Text(0, "quick fox")
	d.Int64(2, 2024)
	d.Text(1, "body text")

	want := []FieldValue{
		{Field: 0, Value: Value{Kind: Text, Str: "quick fox"}},
		{Field: 2, Value: Value{Kind: Int64, Num: 2024}},
		{Field: 1, Value: Value{Kind: Text, Str: "body text"}},
	}
	if len(d.Fields) != len(want) {
		t.Fatalf("got %d fields, want %d", len(d.Fields), len(want))
	}
	for i, w := range want {
		if d.Fields[i] != w {
			t.Errorf("Fields[%d] = %+v, want %+v", i, d.Fields[i], w)
		}
	}
}
