// Package task implements the task contract of §6.1: one markdown file
// per task under .devclean/tasks/, with a frontmatter block in the same
// yaml subset as config.yml and a free notes body.
package task

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Defaults for the optional contract fields (§6.1).
const (
	DefaultLimiteIntentos = 3
	DefaultLimiteLineas   = 200
)

// idPattern validates contract IDs: T-001, T-002, ...
var idPattern = regexp.MustCompile(`^T-\d{3,}$`)

// Task is the task contract. ListoCuando is the only mandatory field
// beyond id and titulo; the rest carry defaults.
type Task struct {
	ID             string   `json:"id"`
	Titulo         string   `json:"titulo"`
	Porque         string   `json:"porque"`
	ListoCuando    string   `json:"listo_cuando"`
	TocarSolo      []string `json:"tocar_solo"`
	NoTocar        []string `json:"no_tocar"`
	LimiteIntentos int      `json:"limite_intentos"`
	LimiteLineas   int      `json:"limite_lineas"`
	Riesgos        string   `json:"riesgos"`

	// Notas is the free body after the frontmatter.
	Notas string `json:"notas,omitempty"`
}

// ValidID reports whether id has the contract format.
func ValidID(id string) bool { return idPattern.MatchString(id) }

// Parse reads a task file. Unknown keys and malformed values are
// rejected: the contract is the guardrail, so it is strict.
func Parse(data []byte) (Task, error) {
	var t Task
	content := strings.ReplaceAll(string(data), "\r\n", "\n")
	lines := strings.Split(content, "\n")

	if strings.TrimSpace(lines[0]) != "---" {
		return t, errors.New("falta el bloque --- inicial")
	}
	closing := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			closing = i
			break
		}
	}
	if closing == -1 {
		return t, errors.New("falta el bloque --- de cierre")
	}

	for i, raw := range lines[1:closing] {
		line := strings.TrimSpace(stripComment(raw))
		if line == "" {
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			return t, fmt.Errorf("línea %d no es clave: valor", i+2)
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		var err error
		switch key {
		case "id":
			t.ID = unquote(value)
		case "titulo":
			t.Titulo = unquote(value)
		case "porque":
			t.Porque = unquote(value)
		case "listo_cuando":
			t.ListoCuando = unquote(value)
		case "tocar_solo":
			t.TocarSolo, err = parseList(value)
		case "no_tocar":
			t.NoTocar, err = parseList(value)
		case "limite_intentos":
			t.LimiteIntentos, err = parsePositiveInt(value)
		case "limite_lineas":
			t.LimiteLineas, err = parsePositiveInt(value)
		case "riesgos":
			t.Riesgos = unquote(value)
		default:
			return t, fmt.Errorf("campo desconocido: %s · el contrato tiene máximo 8 campos", key)
		}
		if err != nil {
			return t, fmt.Errorf("%s: %s", key, err)
		}
	}

	t.Notas = strings.TrimSpace(strings.Join(lines[closing+1:], "\n"))
	return t, nil
}

// Validate checks the contract rules and returns every problem found.
// An empty slice means the contract is valid.
func (t Task) Validate() []error {
	var errs []error
	if !ValidID(t.ID) {
		errs = append(errs, fmt.Errorf("id inválido: %q · usa el formato T-001", t.ID))
	}
	if strings.TrimSpace(t.Titulo) == "" {
		errs = append(errs, errors.New("falta titulo"))
	}
	if strings.TrimSpace(t.ListoCuando) == "" {
		errs = append(errs, errors.New("falta listo_cuando · escribe el comando que dice \"ya está\""))
	}
	if t.LimiteIntentos < 1 {
		errs = append(errs, fmt.Errorf("limite_intentos inválido: %d · mínimo 1", t.LimiteIntentos))
	}
	if t.LimiteLineas < 1 {
		errs = append(errs, fmt.Errorf("limite_lineas inválido: %d · mínimo 1", t.LimiteLineas))
	}
	return errs
}

// Marshal renders the task file.
func (t Task) Marshal() []byte {
	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "id: %s\n", t.ID)
	fmt.Fprintf(&b, "titulo: %s\n", t.Titulo)
	if t.Porque != "" {
		fmt.Fprintf(&b, "porque: %s\n", t.Porque)
	}
	fmt.Fprintf(&b, "listo_cuando: %s\n", t.ListoCuando)
	writeList(&b, "tocar_solo", t.TocarSolo)
	writeList(&b, "no_tocar", t.NoTocar)
	fmt.Fprintf(&b, "limite_intentos: %d\n", t.LimiteIntentos)
	fmt.Fprintf(&b, "limite_lineas: %d\n", t.LimiteLineas)
	if t.Riesgos != "" {
		fmt.Fprintf(&b, "riesgos: %s\n", t.Riesgos)
	}
	b.WriteString("---\n")
	if t.Notas != "" {
		fmt.Fprintf(&b, "\n%s\n", t.Notas)
	}
	return []byte(b.String())
}

func writeList(b *strings.Builder, key string, values []string) {
	quoted := make([]string, len(values))
	for i, v := range values {
		quoted[i] = strconv.Quote(v)
	}
	fmt.Fprintf(b, "%s: [%s]\n", key, strings.Join(quoted, ", "))
}

func parsePositiveInt(v string) (int, error) {
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("entero inválido: %s", v)
	}
	return n, nil
}

// stripComment removes a trailing ` #` comment outside double quotes.
func stripComment(s string) string {
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

// unquote removes surrounding double quotes if present.
func unquote(v string) string {
	if len(v) >= 2 && v[0] == '"' && v[len(v)-1] == '"' {
		if s, err := strconv.Unquote(v); err == nil {
			return s
		}
	}
	return v
}

// parseList parses an inline list of double-quoted strings: ["a", "b"].
func parseList(v string) ([]string, error) {
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
