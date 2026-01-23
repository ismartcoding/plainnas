package search

import (
	"reflect"
	"testing"
)

func Test_splitInGroup(t *testing.T) {
	cases := []struct {
		input  string
		expect []string
	}{
		{"a b c", []string{"a", "b", "c"}},
		{"foo:'bar baz' x", []string{"foo:'bar baz'", "x"}},
		{"foo:\"bar\" baz", []string{"foo:\"bar\"", "baz"}},
		{"a\\ b c", []string{"a b", "c"}},
		{"'a b' c", []string{"'a b'", "c"}},
	}
	for _, c := range cases {
		got := splitInGroup(c.input)
		if !reflect.DeepEqual(got, c.expect) {
			t.Errorf("splitInGroup(%q) = %#v, want %#v", c.input, got, c.expect)
		}
	}
}

func Test_removeQuotation(t *testing.T) {
	cases := []struct {
		input  string
		expect string
	}{
		{"'abc'", "abc"},
		{"\"abc\"", "abc"},
		{"abc", "abc"},
		{"'a b'", "a b"},
	}
	for _, c := range cases {
		got := removeQuotation(c.input)
		if got != c.expect {
			t.Errorf("removeQuotation(%q) = %q, want %q", c.input, got, c.expect)
		}
	}
}

func Test_detectGroupType(t *testing.T) {
	cases := []struct {
		input  string
		expect string
	}{
		{"<=100MB", "<="},
		{">=1GB", ">="},
		{"!=0", "!="},
		{"=42", "="},
		{">10", ">"},
		{"<5", "<"},
		{"100", "="},
	}
	for _, c := range cases {
		got := detectGroupType(c.input)
		if got != c.expect {
			t.Errorf("detectGroupType(%q) = %q, want %q", c.input, got, c.expect)
		}
	}
}

func Test_splitGroup(t *testing.T) {
	cases := []struct {
		input  string
		expect queryGroup
	}{
		{"foo:bar", queryGroup{length: 2, field: "foo", query: "bar", op: "=", value: "bar"}},
		{"foo:>10", queryGroup{length: 2, field: "foo", query: ">10", op: ">", value: "10"}},
		{"foo:<=100", queryGroup{length: 2, field: "foo", query: "<=100", op: "<=", value: "100"}},
		{"foo:!=0", queryGroup{length: 2, field: "foo", query: "!=0", op: "!=", value: "0"}},
		{"foo:bar:baz", queryGroup{length: 2, field: "foo", query: "bar:baz", op: "=", value: "bar:baz"}},
		{"foo", queryGroup{length: 1, field: "foo", query: "", op: "=", value: ""}},
	}
	for _, c := range cases {
		got := splitGroup(c.input)
		if !reflect.DeepEqual(got, c.expect) {
			t.Errorf("splitGroup(%q) = %#v, want %#v", c.input, got, c.expect)
		}
	}
}

func Test_parseGroup(t *testing.T) {
	cases := []struct {
		input  string
		expect FilterField
	}{
		{"foo:bar", FilterField{Name: "foo", Op: "=", Value: "bar"}},
		{"foo:>10", FilterField{Name: "foo", Op: ">", Value: "10"}},
		{"foo:<=100", FilterField{Name: "foo", Op: "<=", Value: "100"}},
		{"foo:!=0", FilterField{Name: "foo", Op: "!=", Value: "0"}},
		{"foo:bar:baz", FilterField{Name: "foo", Op: "=", Value: "bar:baz"}},
		{"foo", FilterField{Name: "text", Op: "", Value: "foo"}},
		{"is:dir", FilterField{Name: "dir", Op: "", Value: "true"}},
	}
	for _, c := range cases {
		got := parseGroup(c.input)
		if !reflect.DeepEqual(got, c.expect) {
			t.Errorf("parseGroup(%q) = %#v, want %#v", c.input, got, c.expect)
		}
	}
}
