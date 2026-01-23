package search

import (
	"reflect"
	"testing"
)

func TestParse_FileSize(t *testing.T) {
	cases := []struct {
		q      string
		expect FilterField
	}{
		{
			q:      "file_size:>1GB",
			expect: FilterField{Name: "file_size", Op: ">", Value: "1GB"},
		},
		{
			q:      "file_size:<=100MB",
			expect: FilterField{Name: "file_size", Op: "<=", Value: "100MB"},
		},
		{
			q:      "file_size:=1234",
			expect: FilterField{Name: "file_size", Op: "=", Value: "1234"},
		},
		{
			q:      "file_size:!=0",
			expect: FilterField{Name: "file_size", Op: "!=", Value: "0"},
		},
		{
			q:      "file_size:1KB",
			expect: FilterField{Name: "file_size", Op: "=", Value: "1KB"},
		},
	}
	for _, c := range cases {
		fields := Parse(c.q)
		if len(fields) != 1 {
			t.Fatalf("Parse(%q) got %d fields, want 1", c.q, len(fields))
		}
		if !reflect.DeepEqual(fields[0], c.expect) {
			t.Errorf("Parse(%q) got %+v, want %+v", c.q, fields[0], c.expect)
		}
	}
}
