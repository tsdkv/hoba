package pbt

import (
	"testing"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/tsdkv/hoba/internal/index"
	"github.com/tsdkv/hoba/schema"
	"github.com/tsdkv/hoba/tokenizer"
)

func testSchema(t *testing.T) *schema.Schema {
	s, err := schema.NewSchema(
		schema.TextField("title", schema.Indexed(), schema.Stored()),
		schema.TextField("code", schema.Indexed(), schema.Keyword()),
		schema.Int64Field("year", schema.Indexed(), schema.Fast()),
		schema.TextField("note", schema.Stored()),
	)
	require.NoError(t, err)
	return s
}

// If this fails, fix the generator before trusting any property that draws from it.
func TestDocumentGeneratorValidity(t *testing.T) {
	s := testSchema(t)
	tm := tokenizer.NewManager()

	rapid.Check(t, func(rt *rapid.T) {
		doc := DocumentGenerator(s, Words).Draw(rt, "doc")

		seen := map[int]bool{}
		for _, fv := range doc.Fields {
			f, ok := s.FieldByID(fv.Field)
			require.True(rt, ok, "unknown field ID %d", fv.Field)
			require.False(rt, seen[fv.Field], "duplicate field %q", f.Name)
			seen[fv.Field] = true

			switch f.Type {
			case schema.FieldTypeText:
				require.Equal(rt, schema.Text, fv.Value.Kind, "field %q", f.Name)
			case schema.FieldTypeInt64:
				require.Equal(rt, schema.Int64, fv.Value.Kind, "field %q", f.Name)
			}
		}

		_, err := index.NewMemSegment(s, tm).Add(doc)
		require.NoError(rt, err)
	})
}

func TestDocumentGeneratorOptions(t *testing.T) {
	s := testSchema(t)
	gen := DocumentGenerator(
		s, Words,
		WithFieldProbability(100),
		WithNumRange(7, 7),
		WithMaxTokens(0),
	)

	rapid.Check(t, func(rt *rapid.T) {
		doc := gen.Draw(rt, "doc")
		require.Len(rt, doc.Fields, len(s.Fields))

		for _, fv := range doc.Fields {
			f, _ := s.FieldByID(fv.Field)
			switch f.Type {
			case schema.FieldTypeText:
				if f.Tokenizer != "keyword" {
					require.Empty(rt, fv.Value.Str, "field %q", f.Name)
				}
			case schema.FieldTypeInt64:
				require.Equal(rt, int64(7), fv.Value.Num, "field %q", f.Name)
			}
		}
	})
}
