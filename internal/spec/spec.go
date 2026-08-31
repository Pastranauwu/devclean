// Package spec implementa el modelo declarativo "Requerimientos como Código":
// un archivo YAML (ej. devclean.spec.yml) define la feature, reglas y la lista
// de tareas con sus criterios de aceptación y dependencias, permitiendo sincronizarlas
// y ejecutarlas en paralelo al estilo Docker Compose.
package spec

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Pastranauwu/devclean/internal/kv"
	"github.com/Pastranauwu/devclean/internal/task"
)

// DefaultSpecNames son los nombres por defecto que devclean busca en la raíz del repo.
var DefaultSpecNames = []string{
	"devclean.spec.yml",
	"devclean.spec.yaml",
	"spec.yml",
	"spec.yaml",
	".devclean/spec.yml",
	".devclean/spec.yaml",
}

// Spec es la especificación declarativa de una feature o conjunto de tareas.
type Spec struct {
	Version int         `json:"version"`
	Feature string      `json:"feature"`
	Agente  string      `json:"agente,omitempty"`
	Reglas  []string    `json:"reglas,omitempty"`
	Tasks   []task.Task `json:"tasks"`
}

// Find busca el archivo de especificación en la raíz del repo.
func Find(root string) (string, error) {
	for _, name := range DefaultSpecNames {
		p := filepath.Join(root, name)
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			return p, nil
		}
	}
	return "", fmt.Errorf("no se encontró archivo de especificación (probó: %s)", strings.Join(DefaultSpecNames, ", "))
}

// Load lee y parsea una especificación desde el disco.
func Load(path string) (Spec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Spec{}, err
	}
	return Parse(data)
}

// Parse convierte un documento YAML en un Spec.
func Parse(data []byte) (Spec, error) {
	lines := strings.Split(string(data), "\n")
	s := Spec{Version: 1}

	// 1. Parsear escalares y listas de primer nivel
	inBlock := ""
	var blockLines []string

	for i, raw := range lines {
		trimmed := strings.TrimSpace(kv.StripComment(raw))
		if trimmed == "" {
			continue
		}
		indent := len(raw) - len(strings.TrimLeft(raw, " \t"))

		if inBlock != "" {
			if indent > 0 {
				blockLines = append(blockLines, raw)
				continue
			}
			// Fin del bloque actual
			if err := processBlock(&s, inBlock, blockLines, i); err != nil {
				return s, err
			}
			inBlock = ""
			blockLines = nil
		}

		key, val, ok := strings.Cut(trimmed, ":")
		if !ok {
			return s, fmt.Errorf("línea %d: sintaxis inválida (falta dos puntos): %s", i+1, trimmed)
		}
		k := strings.TrimSpace(key)
		v := strings.TrimSpace(val)

		switch k {
		case "version":
			n, err := kv.ParseInt(v)
			if err != nil {
				return s, fmt.Errorf("línea %d: %w", i+1, err)
			}
			s.Version = n
		case "feature", "titulo":
			s.Feature = kv.Unquote(v)
		case "agente":
			s.Agente = kv.Unquote(v)
		case "reglas":
			if v != "" {
				r, err := kv.ParseList(v)
				if err != nil {
					return s, fmt.Errorf("línea %d: %w", i+1, err)
				}
				s.Reglas = r
			} else {
				inBlock = "reglas"
			}
		case "tasks", "tareas":
			inBlock = "tasks"
		default:
			return s, fmt.Errorf("línea %d: campo desconocido en especificación: %s", i+1, k)
		}
	}

	if inBlock != "" {
		if err := processBlock(&s, inBlock, blockLines, len(lines)); err != nil {
			return s, err
		}
	}

	// 2. Aplicar defaults y agente por defecto a las tareas
	for i := range s.Tasks {
		if s.Tasks[i].Version == 0 {
			s.Tasks[i].Version = task.Version
		}
		if s.Tasks[i].Agente == "" && s.Agente != "" {
			s.Tasks[i].Agente = s.Agente
		}
		if s.Tasks[i].LimiteIntentos == 0 {
			s.Tasks[i].LimiteIntentos = task.DefaultLimiteIntentos
		}
		if s.Tasks[i].LimiteLineas == 0 {
			s.Tasks[i].LimiteLineas = task.DefaultLimiteLineas
		}
	}

	return s, nil
}

func processBlock(s *Spec, blockName string, lines []string, offset int) error {
	switch blockName {
	case "reglas":
		for i, raw := range lines {
			trimmed := strings.TrimSpace(kv.StripComment(raw))
			if trimmed == "" {
				continue
			}
			if !strings.HasPrefix(trimmed, "-") {
				return fmt.Errorf("línea %d: regla debe iniciar con '-'", i+offset-len(lines)+1)
			}
			regla := strings.TrimSpace(strings.TrimPrefix(trimmed, "-"))
			s.Reglas = append(s.Reglas, kv.Unquote(regla))
		}
	case "tasks":
		tasks, err := parseTaskList(lines, offset-len(lines))
		if err != nil {
			return err
		}
		s.Tasks = append(s.Tasks, tasks...)
	}
	return nil
}

// parseTaskList procesa una lista de tareas en YAML, soportando items con '-' o mapas inline.
func parseTaskList(lines []string, startLine int) ([]task.Task, error) {
	var chunks [][]string
	var current []string

	for _, raw := range lines {
		trimmed := strings.TrimSpace(kv.StripComment(raw))
		if trimmed == "" {
			continue
		}
		// Nuevo item de lista comienza con '- ' o '-'
		if strings.HasPrefix(trimmed, "-") {
			if len(current) > 0 {
				chunks = append(chunks, current)
			}
			current = []string{raw}
		} else if len(current) > 0 {
			current = append(current, raw)
		} else {
			return nil, fmt.Errorf("línea %d: elemento de lista de tareas debe comenzar con '-'", startLine)
		}
	}
	if len(current) > 0 {
		chunks = append(chunks, current)
	}

	var tasks []task.Task
	for chunkIdx, chunk := range chunks {
		t, err := parseTaskChunk(chunk, startLine+chunkIdx)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, nil
}

func parseTaskChunk(chunk []string, lineNum int) (task.Task, error) {
	var t task.Task
	t.Version = task.Version
	t.LimiteIntentos = task.DefaultLimiteIntentos
	t.LimiteLineas = task.DefaultLimiteLineas

	firstLine := strings.TrimSpace(kv.StripComment(chunk[0]))
	contentFirst := strings.TrimSpace(strings.TrimPrefix(firstLine, "-"))

	// Caso 1: mapa inline: - { id: T-001, titulo: "..." }
	if strings.HasPrefix(contentFirst, "{") && strings.HasSuffix(contentFirst, "}") {
		m, err := kv.ParseInlineMap(contentFirst)
		if err != nil {
			return t, fmt.Errorf("línea %d: %w", lineNum, err)
		}
		return buildTaskFromMap(m, lineNum)
	}

	// Caso 2: bloque de pares clave-valor
	var kvLines []string
	if contentFirst != "" {
		kvLines = append(kvLines, contentFirst)
	}
	for _, l := range chunk[1:] {
		trimmed := strings.TrimSpace(kv.StripComment(l))
		if trimmed != "" {
			kvLines = append(kvLines, trimmed)
		}
	}

	pairs, err := kv.Pairs(kvLines, lineNum)
	if err != nil {
		return t, err
	}

	for _, p := range pairs {
		var err error
		switch p.Key {
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
		case "notas":
			t.Notas = kv.Unquote(p.Value)
		default:
			return t, fmt.Errorf("línea %d: campo desconocido en tarea: %s", p.Line, p.Key)
		}
		if err != nil {
			return t, fmt.Errorf("línea %d: %s: %w", p.Line, p.Key, err)
		}
	}

	return t, nil
}

func buildTaskFromMap(m map[string]string, lineNum int) (task.Task, error) {
	var t task.Task
	t.Version = task.Version
	t.LimiteIntentos = task.DefaultLimiteIntentos
	t.LimiteLineas = task.DefaultLimiteLineas

	var err error
	for k, v := range m {
		switch k {
		case "id":
			t.ID = v
		case "titulo":
			t.Titulo = v
		case "porque":
			t.Porque = v
		case "listo_cuando":
			t.ListoCuando = v
		case "tocar_solo":
			t.TocarSolo, err = kv.ParseList(v)
		case "no_tocar":
			t.NoTocar, err = kv.ParseList(v)
		case "depende_de":
			t.DependeDe, err = kv.ParseList(v)
		case "expone":
			t.Expone, err = kv.ParseList(v)
		case "usa":
			t.Usa, err = kv.ParseList(v)
		case "limite_intentos":
			t.LimiteIntentos, err = kv.ParseInt(v)
		case "limite_lineas":
			t.LimiteLineas, err = kv.ParseInt(v)
		case "riesgos":
			t.Riesgos = v
		case "peso":
			t.Peso = v
		case "agente":
			t.Agente = v
		case "notas":
			t.Notas = v
		default:
			return t, fmt.Errorf("línea %d: campo desconocido en tarea: %s", lineNum, k)
		}
		if err != nil {
			return t, fmt.Errorf("línea %d: %s: %w", lineNum, k, err)
		}
	}
	return t, nil
}

// AssignCorrelativeIDs asigna IDs correlativos (T-001, T-002, ...) a las tareas que no tengan ID asignado.
func AssignCorrelativeIDs(tasksDir string, tasks []task.Task) ([]task.Task, error) {
	out := make([]task.Task, len(tasks))
	copy(out, tasks)

	var needID []int
	for i, t := range out {
		if strings.TrimSpace(t.ID) == "" {
			needID = append(needID, i)
		}
	}

	if len(needID) > 0 {
		var nextNum int = 1
		if existing, err := task.List(tasksDir); err == nil && len(existing) > 0 {
			var ids []string
			for _, e := range existing {
				ids = append(ids, e.ID)
			}
			sort.Strings(ids)
			last := ids[len(ids)-1]
			var n int
			if _, err := fmt.Sscanf(last, "T-%d", &n); err == nil {
				nextNum = n + 1
			}
		}

		for _, idx := range needID {
			out[idx].ID = fmt.Sprintf("T-%03d", nextNum)
			nextNum++
		}
	}

	return out, nil
}

// Apply valida y guarda las tareas de la especificación en tasksDir (.devclean/tasks/).
func Apply(tasksDir string, s Spec, dryRun bool) ([]task.Task, error) {
	tasksWithIDs, err := AssignCorrelativeIDs(tasksDir, s.Tasks)
	if err != nil {
		return nil, err
	}

	// Aplicar defaults y agente por defecto si no vienen
	for i := range tasksWithIDs {
		if tasksWithIDs[i].Version == 0 {
			tasksWithIDs[i].Version = task.Version
		}
		if tasksWithIDs[i].Agente == "" && s.Agente != "" {
			tasksWithIDs[i].Agente = s.Agente
		}
		if tasksWithIDs[i].LimiteIntentos == 0 {
			tasksWithIDs[i].LimiteIntentos = task.DefaultLimiteIntentos
		}
		if tasksWithIDs[i].LimiteLineas == 0 {
			tasksWithIDs[i].LimiteLineas = task.DefaultLimiteLineas
		}
	}

	// Validar todas las tareas antes de escribir nada
	for _, t := range tasksWithIDs {
		if errs := t.Validate(); len(errs) > 0 {
			var msgs []string
			for _, e := range errs {
				msgs = append(msgs, e.Error())
			}
			return nil, fmt.Errorf("tarea %s inválida: %s", t.ID, strings.Join(msgs, " · "))
		}
	}

	if dryRun {
		return tasksWithIDs, nil
	}

	if err := os.MkdirAll(tasksDir, 0o755); err != nil {
		return nil, err
	}

	for _, t := range tasksWithIDs {
		if err := task.Save(tasksDir, t); err != nil {
			return nil, err
		}
	}

	return tasksWithIDs, nil
}

// Marshal genera el contenido YAML de una especificación.
func Marshal(s Spec) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "version: %d\n", s.Version)
	if s.Feature != "" {
		fmt.Fprintf(&b, "feature: %s\n", kv.Quote(s.Feature))
	}
	if s.Agente != "" {
		fmt.Fprintf(&b, "agente: %s\n", s.Agente)
	}
	if len(s.Reglas) > 0 {
		b.WriteString("reglas:\n")
		for _, r := range s.Reglas {
			fmt.Fprintf(&b, "  - %s\n", kv.Quote(r))
		}
	}
	b.WriteString("\ntasks:\n")
	for _, t := range s.Tasks {
		fmt.Fprintf(&b, "  - id: %s\n", t.ID)
		fmt.Fprintf(&b, "    titulo: %s\n", kv.Quote(t.Titulo))
		if t.Porque != "" {
			fmt.Fprintf(&b, "    porque: %s\n", kv.Quote(t.Porque))
		}
		fmt.Fprintf(&b, "    listo_cuando: %s\n", kv.Quote(t.ListoCuando))
		if len(t.TocarSolo) > 0 {
			fmt.Fprintf(&b, "    tocar_solo: %s\n", kv.MarshalList(t.TocarSolo))
		}
		if len(t.NoTocar) > 0 {
			fmt.Fprintf(&b, "    no_tocar: %s\n", kv.MarshalList(t.NoTocar))
		}
		if len(t.DependeDe) > 0 {
			fmt.Fprintf(&b, "    depende_de: %s\n", kv.MarshalList(t.DependeDe))
		}
		if len(t.Expone) > 0 {
			fmt.Fprintf(&b, "    expone: %s\n", kv.MarshalList(t.Expone))
		}
		if len(t.Usa) > 0 {
			fmt.Fprintf(&b, "    usa: %s\n", kv.MarshalList(t.Usa))
		}
		if t.Peso != "" {
			fmt.Fprintf(&b, "    peso: %s\n", t.Peso)
		}
		if t.Agente != "" && t.Agente != s.Agente {
			fmt.Fprintf(&b, "    agente: %s\n", t.Agente)
		}
		if t.Riesgos != "" {
			fmt.Fprintf(&b, "    riesgos: %s\n", kv.Quote(t.Riesgos))
		}
	}
	return []byte(b.String())
}
