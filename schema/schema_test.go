package schema

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFieldTypeString(t *testing.T) {
	tests := []struct {
		ft   FieldType
		want string
	}{
		{FieldTypeText, "text"},
		{FieldTypeInt64, "int64"},
		{FieldType(42), "FieldType(42)"},
		{FieldType(-1), "FieldType(-1)"},
	}
	for _, tc := range tests {
		assert.Equal(t, tc.want, tc.ft.String(), "FieldType(%d)", int(tc.ft))
	}
}

func TestNewSchemaAssignsDenseIDs(t *testing.T) {
	s, err := NewSchema(TextField("a", Indexed()), TextField("b", Indexed()))
	require.NoError(t, err)
	for i, f := range s.Fields {
		assert.Equal(t, i, f.ID, "Fields[%d]", i)
	}
}

func TestUnmarshalJSON(t *testing.T) {
	valid, err := NewSchema(TextField("title", Indexed(), WithPositions()))
	require.NoError(t, err)
	data, err := json.Marshal(valid)
	require.NoError(t, err)

	tests := []struct {
		name    string
		data    string
		wantErr string // "" means success
	}{
		{"round trip", string(data), ""},
		{"dense IDs in order", `{"fields":[{"id":0,"name":"a","type":0},{"id":1,"name":"b","type":1}]}`, ""},
		{"non-dense ID", `{"fields":[{"id":1,"name":"a","type":0}]}`, "has ID 1, want 0"},
		{"IDs out of order", `{"fields":[{"id":1,"name":"a","type":0},{"id":0,"name":"b","type":1}]}`, "has ID 1, want 0"},
		{"empty name", `{"fields":[{"id":0,"name":"","type":0}]}`, "empty name"},
		{"duplicate name", `{"fields":[{"id":0,"name":"a","type":0},{"id":1,"name":"a","type":1}]}`, "duplicate field"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var s Schema
			err := json.Unmarshal([]byte(tc.data), &s)
			if tc.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.ErrorContains(t, err, tc.wantErr)
		})
	}
}
