// Package kv implements the small yaml subset devclean uses in both
// .devclean/config.yml (§8.1) and the task contract frontmatter (§6.1):
// flat `clave: valor` scalars and inline lists `clave: ["a", "b"]`.
//
// It lives apart so config and task share one parser. Two copies that
// drift is a bug waiting to happen (adenda C.1).
package kv

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// StripComment removes a trailing ` #` comment outside double quotes.
func StripComment(s string) string {
	inQuote := false
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '"':
			inQuote = !inQuote
		case '#':
			if !inQuote && (i == 0 || s[i-1] == ' ' || s[i-1] == '\t') {
				return s[:i]
			}
		}
	}
	return s
}

// Unquote removes surrounding double quotes if present.
func Unquote(v string) string {
	if len(v) >= 2 && v[0] == '"' && v[len(v)-1] == '"' {
		if s, err := strconv.Unquote(v); err == nil {
			return s
		}
	}
	return v
}

// Quote renders a value as a double-quoted string.
func Quote(v string) string { return strconv.Quote(v) }

// ParseList parses an inline list of double-quoted strings: ["a", "b"].
// A missing value yields a nil list; an empty one, an empty list.
func ParseList(v string) ([]string, error) {
	if v == "" {
		return nil, nil
	}
	if !strings.HasPrefix(v, "[") || !strings.HasSuffix(v, "]") {
		return nil, fmt.Errorf("lista mal formada: %s", v)
	}
	inner := strings.TrimSpace(v[1 : len(v)-1])
	if inner == "" {
		return []string{}, nil
	}
	parts := strings.Split(inner, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if len(p) < 2 || p[0] != '"' || p[len(p)-1] != '"' {
			return nil, fmt.Errorf("elemento de lista sin comillas: %s", p)
		}
		s, err := strconv.Unquote(p)
		if err != nil {
			return nil, fmt.Errorf("elemento de lista mal formado: %s", p)
		}
		out = append(out, s)
	}
	return out, nil
}

// MarshalList renders a list the way ParseList reads it.
func MarshalList(values []string) string {
	quoted := make([]string, len(values))
	for i, v := range values {
		quoted[i] = strconv.Quote(v)
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

// ParseInt reads an integer scalar.
func ParseInt(v string) (int, error) {
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("entero inválido: %s", v)
	}
	return n, nil
}

// ParseInlineMap parses an inline map literal `{ a: b, c: d }` into a
// map of unquoted strings. Empty braces yield an empty map.
func ParseInlineMap(v string) (map[string]string, error) {
	v = strings.TrimSpace(v)
	if !strings.HasPrefix(v, "{") || !strings.HasSuffix(v, "}") {
		return nil, fmt.Errorf("mapa mal formado: %s", v)
	}
	inner := strings.TrimSpace(v[1 : len(v)-1])
	if inner == "" {
		return map[string]string{}, nil
	}
	out := map[string]string{}
	for _, part := range strings.Split(inner, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		k, val, ok := strings.Cut(part, ":")
		if !ok {
			return nil, fmt.Errorf("elemento de mapa sin dos puntos: %s", part)
		}
		out[strings.TrimSpace(k)] = Unquote(strings.TrimSpace(val))
	}
	return out, nil
}

// MarshalInlineMap renders a map the way ParseInlineMap reads it, keys
// sorted for a stable output.
func MarshalInlineMap(m map[string]string) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+": "+Quote(m[k]))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

// Pair is one `clave: valor` line, with the 1-based line number it came
// from so callers can point at it in an error.
type Pair struct {
	Key   string
	Value string
	Line  int
}

// Nested returns the indented child pairs under a top-level `clave:`
// heading with an empty value. offset is added to every child line
// number, like Pairs. If the heading is absent, it returns nil.
func Nested(lines []string, key string, offset int) ([]Pair, error) {
	var out []Pair
	inBlock := false
	headingIndent := -1
	for i, raw := range lines {
		trimmed := strings.TrimSpace(StripComment(raw))
		if trimmed == "" {
			continue
		}
		indent := len(raw) - len(strings.TrimLeft(raw, " \t"))

		if !inBlock {
			k, v, ok := strings.Cut(trimmed, ":")
			if ok && strings.TrimSpace(k) == key && strings.TrimSpace(v) == "" {
				inBlock = true
				headingIndent = indent
			}
			continue
		}

		// fin del bloque: línea con indentación menor o igual al heading
		if indent <= headingIndent {
			break
		}
		k, v, ok := strings.Cut(trimmed, ":")
		if !ok {
			return nil, fmt.Errorf("línea %d no es clave: valor", i+offset)
		}
		out = append(out, Pair{
			Key:   strings.TrimSpace(k),
			Value: strings.TrimSpace(v),
			Line:  i + offset,
		})
	}
	return out, nil
}

// Pairs splits a document into its key/value lines, skipping blanks and
// comments. offset is added to every line number, so a caller parsing a
// frontmatter block can report positions in the whole file.
func Pairs(lines []string, offset int) ([]Pair, error) {
	var out []Pair
	for i, raw := range lines {
		line := strings.TrimSpace(StripComment(raw))
		if line == "" {
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			return nil, fmt.Errorf("línea %d no es clave: valor", i+offset)
		}
		out = append(out, Pair{
			Key:   strings.TrimSpace(key),
			Value: strings.TrimSpace(value),
			Line:  i + offset,
		})
	}
	return out, nil
}
