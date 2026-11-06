package schema

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
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
	assert.Equal(t, want, d.Fields)
}

func TestDocumentSerializationRoundTrip(t *testing.T) {
	s, err := NewSchema(TextField("title", Stored()), Int64Field("count", Stored()))
	require.NoError(t, err)

	fieldValueGen := rapid.Custom(func(rt *rapid.T) FieldValue {
		field := rapid.SampledFrom([]int{0, 1}).Draw(rt, "field")
		kind := rapid.SampledFrom([]Kind{Text, Int64}).Draw(rt, "kind")
		value := Value{Kind: kind}
		if kind == Text {
			value.Str = rapid.String().Draw(rt, "text")
		} else {
			value.Num = rapid.Int64().Draw(rt, "number")
		}
		return FieldValue{Field: field, Value: value}
	})

	rapid.Check(t, func(rt *rapid.T) {
		doc := Document{Fields: rapid.SliceOf(fieldValueGen).Draw(rt, "fields")}
		var buf bytes.Buffer

		n, err := doc.Serialize(&buf, s)
		require.NoError(rt, err)
		require.Equal(rt, buf.Len(), n)

		got, err := DeserializeDocument(buf.Bytes())
		require.NoError(rt, err)
		assert.Equal(rt, doc.Fields, got.Fields)
	})
}

func TestDocumentDeserializationRejectsTrailingData(t *testing.T) {
	doc, err := DeserializeDocument([]byte{0, 0})
	assert.Nil(t, doc)
	assert.EqualError(t, err, "unexpected trailing data of length 1")
}

func BenchmarkDeserializeDocumentText(b *testing.B) {
	s, err := NewSchema(
		TextField("title", Stored()),
		TextField("body", Stored()),
		TextField("footer", Stored()),
	)
	require.NoError(b, err)
	doc := Document{}
	doc.Text(0, "hello")
	doc.Text(1, "bodybody")
	doc.Text(2, "bodybody")
	var buf bytes.Buffer
	_, err = doc.Serialize(&buf, s)
	require.NoError(b, err)
	data := buf.Bytes()

	b.ReportAllocs()
	for b.Loop() {
		_, err := DeserializeDocument(data)
		require.NoError(b, err)
	}
}
