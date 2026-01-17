package search

import "strings"

type FilterField struct {
	Name  string
	Op    string
	Value string
}

type queryGroup struct {
	length int
	field  string
	query  string
	op     string
	value  string
}

const notType = "NOT"

var invert = map[string]string{
	"=":   "!=",
	">=":  "<",
	">":   "<=",
	"!=":  "=",
	"<=":  ">",
	"<":   ">=",
	"in":  "nin",
	"nin": "in",
}

var groupTypes = func() []string {
	keys := make([]string, 0, len(invert))
	for k := range invert {
		if k == "in" || k == "nin" {
			continue
		}
		keys = append(keys, k)
	}
	return keys
}()

func splitInGroup(input string) []string {
	var result []string
	b := &strings.Builder{}
	var quote byte
	escape := false
	for i := 0; i < len(input); i++ {
		c := input[i]
		if escape {
			b.WriteByte(c)
			escape = false
			continue
		}
		if c == '\\' {
			escape = true
			continue
		}
		if quote != 0 {
			if c == quote {
				quote = 0
			} else {
				b.WriteByte(c)
			}
			continue
		}
		if c == '"' || c == '\'' {
			quote = c
			continue
		}
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			if b.Len() > 0 {
				result = append(result, b.String())
				b.Reset()
			}
			continue
		}
		b.WriteByte(c)
	}
	if b.Len() > 0 {
		result = append(result, b.String())
	}
	return result
}

func removeQuotation(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

func detectGroupType(group string) string {
	for _, t := range groupTypes {
		if strings.Contains(group, t) {
			return t
		}
	}
	return ""
}

func splitGroup(q string) queryGroup {
	parts := strings.Split(q, ":")
	field := removeQuotation(parts[0])
	query := removeQuotation(strings.Join(parts[1:], ":"))
	typ := detectGroupType(query)
	if typ == "" {
		// No explicit operator in query. Treat as equality but do not strip any characters.
		typ = "="
	}
	value := ""
	if typ != "" && strings.HasPrefix(query, typ) {
		value = query[len(typ):]
	} else {
		// If the detected/default operator is not actually a prefix of the query,
		// keep the query intact as the value.
		value = query
	}
	return queryGroup{length: len(parts), field: field, query: query, op: typ, value: value}
}

func parseGroup(group string) FilterField {
	if group == notType {
		return FilterField{Op: notType}
	}
	parts := splitGroup(group)
	if parts.field == "is" {
		return FilterField{Name: parts.query, Op: "", Value: "true"}
	}
	if parts.length == 1 {
		return FilterField{Name: "text", Op: "", Value: parts.field}
	}
	return FilterField{Name: parts.field, Op: parts.op, Value: parts.value}
}

func Parse(q string) []FilterField {
	if strings.TrimSpace(q) == "" {
		return []FilterField{}
	}
	groups := splitInGroup(q)
	fields := make([]FilterField, 0, len(groups))
	for _, g := range groups {
		fields = append(fields, parseGroup(g))
	}
	invertNext := false
	for i := range fields {
		if fields[i].Op == notType {
			invertNext = true
			continue
		}
		if invertNext {
			if v, ok := invert[fields[i].Op]; ok {
				fields[i].Op = v
			} else {
				fields[i].Op = ""
			}
			invertNext = false
		}
	}
	out := fields[:0]
	for _, f := range fields {
		if f.Op != notType {
			out = append(out, f)
		}
	}
	return out
}

// Optional tag mapping like QueryHelper in Kotlin: replace tag_id with ids
func ParseWithTagMapping(q string, getKeysByTagIDs func(tagIDs []string) []string) []FilterField {
	fields := Parse(q)
	tagIDsSet := map[string]struct{}{}
	for _, f := range fields {
		if f.Name == "tag_id" {
			tagIDsSet[f.Value] = struct{}{}
		}
	}
	if len(tagIDsSet) == 0 {
		return fields
	}
	// collect
	tagIDs := make([]string, 0, len(tagIDsSet))
	for id := range tagIDsSet {
		tagIDs = append(tagIDs, id)
	}
	ids := getKeysByTagIDs(tagIDs)
	// filter out tag_id fields
	out := make([]FilterField, 0, len(fields))
	for _, f := range fields {
		if f.Name != "tag_id" {
			out = append(out, f)
		}
	}
	if len(ids) == 0 {
		out = append(out, FilterField{Name: "ids", Op: ":", Value: "invalid_ids"})
		return out
	}
	out = append(out, FilterField{Name: "ids", Op: ":", Value: strings.Join(ids, ",")})
	return out
}
