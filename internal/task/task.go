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
	DependeDe      []string `json:"depende_de,omitempty"`

	// Expone son las firmas públicas que esta tarea produce y que otras
	// consumen: "wol.Send(mac, addr string) error", "POST /wake".
	// Usa son las que consume de sus hermanas (§6.10). Congelarlas en
	// ambos contratos es lo que deja que dos tareas de la misma oleada,
	// que corren ciegas en cuartos separados, se encuentren.
	Expone []string `json:"expone,omitempty"`
	Usa    []string `json:"usa,omitempty"`

	LimiteIntentos int      `json:"limite_intentos"`
	LimiteLineas   int      `json:"limite_lineas"`
	Riesgos        string   `json:"riesgos"`
	Peso           string   `json:"peso,omitempty"` // liviana | media | pesada ("" = estrategia global)
	Agente         string   `json:"agente,omitempty"` // agente asignado (Fase 2)

	// Notas is the free body after the frontmatter.
	Notas string `json:"notas,omitempty"`

	// Aviso carries the warning of a contract written by a newer
	// devclean: it parsed, but fields were skipped.
	Aviso string `json:"aviso,omitempty"`
}

// ValidID reports whether id has the contract format.
func ValidID(id string) bool { return idPattern.MatchString(id) }

// NombreDeFirma reduce una firma de `expone`/`usa` al identificador que
// tiene que aparecer sí o sí en el código: "wol.Send(mac string) error"
// → "Send", "POST /wake" → "/wake".
//
// Deliberadamente no compara la firma completa. El lenguaje reescribe los
// nombres de parámetros y el orden de los tipos, así que exigir el texto
// literal rechazaría implementaciones correctas. Lo que sí atrapa es el
// fallo real: que la tarea no haya implementado la pieza, o la haya
// bautizado distinto de lo que su hermana espera. La verificación por
// AST completo es v0.2 (§6.8).
func NombreDeFirma(firma string) string {
	s := strings.TrimSpace(firma)
	if i := strings.Index(s, "("); i >= 0 {
		s = strings.TrimSpace(s[:i])
	}
	// ruta HTTP: "POST /wake" → "/wake"
	if i := strings.Index(s, "/"); i >= 0 {
		if ruta := strings.TrimSpace(s[i:]); ruta != "/" {
			return ruta
		}
	}
	if i := strings.LastIndex(s, "."); i >= 0 {
		s = s[i+1:]
	}
	if i := strings.LastIndex(s, " "); i >= 0 {
		s = s[i+1:]
	}
	return strings.TrimSpace(s)
}

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
		case "depende_de":
			t.DependeDe, err = kv.ParseList(p.Value)
		case "expone":
			t.Expone, err = kv.ParseList(p.Value)
		case "usa":
			t.Usa, err = kv.ParseList(p.Value)
		case "limite_intentos":
			t.LimiteIntentos, err = kv.ParseInt(p.Value)
		case "limite_lineas":
			t.LimiteLineas, err = kv.ParseInt(p.Value)
		case "riesgos":
			t.Riesgos = kv.Unquote(p.Value)
		case "peso":
			t.Peso = kv.Unquote(p.Value)
		case "agente":
			t.Agente = kv.Unquote(p.Value)
		default:
			// campos del futuro: se ignoran solo si el archivo lo es
			if t.Aviso != "" {
				continue
			}
			return t, fmt.Errorf("campo desconocido: %s · revisa el contrato", p.Key)
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
	for _, d := range t.DependeDe {
		if !ValidID(d) {
			errs = append(errs, fmt.Errorf("depende_de inválido: %q · usa el formato T-001", d))
		}
		if d == t.ID {
			errs = append(errs, fmt.Errorf("depende_de se refiere a sí misma: %s · quítala", d))
		}
	}
	switch t.Peso {
	case "", "liviana", "media", "pesada":
	default:
		errs = append(errs, fmt.Errorf("peso inválido: %q · usa liviana, media o pesada", t.Peso))
	}
	if t.Agente != "" && strings.ContainsAny(t.Agente, " \t\n\r:") {
		errs = append(errs, fmt.Errorf("agente inválido: %q · usa un nombre sin espacios", t.Agente))
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
	if len(t.DependeDe) > 0 {
		fmt.Fprintf(&b, "depende_de: %s\n", kv.MarshalList(t.DependeDe))
	}
	if len(t.Expone) > 0 {
		fmt.Fprintf(&b, "expone: %s\n", kv.MarshalList(t.Expone))
	}
	if len(t.Usa) > 0 {
		fmt.Fprintf(&b, "usa: %s\n", kv.MarshalList(t.Usa))
	}
	fmt.Fprintf(&b, "limite_intentos: %d\n", t.LimiteIntentos)
	fmt.Fprintf(&b, "limite_lineas: %d\n", t.LimiteLineas)
	if t.Riesgos != "" {
		fmt.Fprintf(&b, "riesgos: %s\n", t.Riesgos)
	}
	if t.Peso != "" {
		fmt.Fprintf(&b, "peso: %s\n", t.Peso)
	}
	if t.Agente != "" {
		fmt.Fprintf(&b, "agente: %s\n", t.Agente)
	}
	b.WriteString("---\n")
	if t.Notas != "" {
		fmt.Fprintf(&b, "\n%s\n", t.Notas)
	}
	return []byte(b.String())
}
