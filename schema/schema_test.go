package schema

import (
	"encoding/json"
	"strings"
	"testing"
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
		if got := tc.ft.String(); got != tc.want {
			t.Errorf("FieldType(%d).String() = %q, want %q", int(tc.ft), got, tc.want)
		}
	}
}

func TestNewSchemaAssignsDenseIDs(t *testing.T) {
	s, err := NewSchema(TextField("a", Indexed()), TextField("b", Indexed()))
	if err != nil {
		t.Fatalf("NewSchema: %v", err)
	}
	for i, f := range s.Fields {
		if f.ID != i {
			t.Errorf("Fields[%d].ID = %d", i, f.ID)
		}
	}
}

func TestUnmarshalJSON(t *testing.T) {
	valid, err := NewSchema(TextField("title", Indexed(), WithPositions()))
	if err != nil {
		t.Fatalf("NewSchema: %v", err)
	}
	data, err := json.Marshal(valid)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

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
				if err != nil {
					t.Fatalf("Unmarshal: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("Unmarshal(%s) expected error, got nil", tc.data)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("Unmarshal(%s) error = %v, want it to mention %q", tc.data, err, tc.wantErr)
			}
		})
	}
}
