// Package task implements the task contract of §6.1: one markdown file
// per task under .devclean/tasks/, with a frontmatter block in the same
// yaml subset as config.yml and a free notes body.
package task

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/Pastranauwu/devclean/internal/kv"
)

// Defaults for the optional contract fields (§6.1).
const (
	DefaultLimiteIntentos = 3
	DefaultLimiteLineas   = 200
)

// Version is the contract version this binary understands (adenda A.1).
// A file declaring a higher one is read tolerantly and reported.
const Version = 1

// idPattern validates contract IDs: T-001, T-002, ...
var idPattern = regexp.MustCompile(`^T-\d{3,}$`)

// Task is the task contract. ListoCuando is the only mandatory field
// beyond id and titulo; the rest carry defaults.
type Task struct {
	Version        int      `json:"version"`
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

	// Aviso carries the warning of a contract written by a newer
	// devclean: it parsed, but fields were skipped.
	Aviso string `json:"aviso,omitempty"`
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

	pares, err := kv.Pairs(lines[1:closing], 2)
	if err != nil {
		return t, err
	}

	// la versión se lee primero: decide qué hacer con lo desconocido
	for _, p := range pares {
		if p.Key != "version" {
			continue
		}
		n, err := kv.ParseInt(p.Value)
		if err != nil || n < 1 {
			return t, fmt.Errorf("version inválida: %s · usa version: %d", p.Value, Version)
		}
		t.Version = n
	}
	if t.Version > Version {
		t.Aviso = fmt.Sprintf("contrato versión %d, binario soporta %d · actualiza devclean", t.Version, Version)
	}

	for _, p := range pares {
		var err error
		switch p.Key {
		case "version":
		case "id":
			t.ID = kv.Unquote(p.Value)
		case "titulo":
			t.Titulo = kv.Unquote(p.Value)
		case "porque":
			t.Porque = kv.Unquote(p.Value)
		case "listo_cuando":
			t.ListoCuando = kv.Unquote(p.Value)
		case "tocar_solo":
			t.TocarSolo, err = kv.ParseList(p.Value)
		case "no_tocar":
			t.NoTocar, err = kv.ParseList(p.Value)
		case "limite_intentos":
			t.LimiteIntentos, err = kv.ParseInt(p.Value)
		case "limite_lineas":
			t.LimiteLineas, err = kv.ParseInt(p.Value)
		case "riesgos":
			t.Riesgos = kv.Unquote(p.Value)
		default:
			// campos del futuro: se ignoran solo si el archivo lo es
			if t.Aviso != "" {
				continue
			}
			return t, fmt.Errorf("campo desconocido: %s · el contrato tiene máximo 8 campos", p.Key)
		}
		if err != nil {
			return t, fmt.Errorf("%s: %s", p.Key, err)
		}
	}

	t.Notas = strings.TrimSpace(strings.Join(lines[closing+1:], "\n"))
	return t, nil
}

// Validate checks the contract rules and returns every problem found.
// An empty slice means the contract is valid.
func (t Task) Validate() []error {
	var errs []error
	if t.Version < 1 {
		errs = append(errs, fmt.Errorf("falta version · agrega version: %d al contrato", Version))
	}
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
	if t.Version > 0 {
		fmt.Fprintf(&b, "version: %d\n", t.Version)
	}
	fmt.Fprintf(&b, "id: %s\n", t.ID)
	fmt.Fprintf(&b, "titulo: %s\n", t.Titulo)
	if t.Porque != "" {
		fmt.Fprintf(&b, "porque: %s\n", t.Porque)
	}
	fmt.Fprintf(&b, "listo_cuando: %s\n", t.ListoCuando)
	fmt.Fprintf(&b, "tocar_solo: %s\n", kv.MarshalList(t.TocarSolo))
	fmt.Fprintf(&b, "no_tocar: %s\n", kv.MarshalList(t.NoTocar))
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
