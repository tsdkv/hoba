package schema

import "testing"

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
