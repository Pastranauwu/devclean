// Package config handles .devclean/config.yml, the project settings of
// devclean (§8.1), and the layout of the .devclean directory.
//
// The file uses a small yaml subset: flat `clave: valor` scalars and
// inline lists `clave: ["a", "b"]`. Values with commas inside quotes are
// not supported. Provider and key settings arrive with the agent engine.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Pastranauwu/devclean/internal/kv"
)

// DirName is the directory devclean creates at the repository root.
const DirName = ".devclean"

// Config is the content of .devclean/config.yml.
type Config struct {
	Base            string               `json:"base"`
	Pruebas         string               `json:"pruebas"`
	// Cli fija el CLI de agente por defecto (opencode | claude). Se llama
	// "cli" y no "ejecutor" a propósito: ese nombre ya lo usa el rol
	// `ejecutor` dentro de `proveedores` (§8.1), y kv.Pairs no distingue
	// indentación — dos claves iguales a distinta profundidad se pisan.
	Cli string `json:"cli,omitempty"`
	ZonasProhibidas []string             `json:"zonas_prohibidas"`
	TimeoutEsclusa  int                  `json:"timeout_esclusa"` // segundos para el chequeo "falla hoy"
	PatronesPrueba  []string             `json:"patrones_prueba"` // rutas que ninguna tarea puede editar
	Proveedores     map[string]Proveedor `json:"proveedores,omitempty"`
	Agentes         map[string]Agente    `json:"agentes,omitempty"`
	Estrategia      string               `json:"estrategia,omitempty"` // ligera | equilibrada | pesada
	Modelos         map[string]string    `json:"modelos,omitempty"`    // peso -> modelo (liviana/media/pesada)
	// ReglasImport declara la dirección permitida entre módulos (§6.10):
	// cada string es una cadena como "api → dominio → datos". Se verifica
	// en la esclusa de salida que ningún import del diff viole el orden.
	ReglasImport []string `json:"reglas_import,omitempty"`
}

// Proveedor es un rol del motor de agentes (§8.1): qué modelo usa y de
// qué variable de entorno sale su key. Los roles son planificador,
// ejecutor y revisor.
type Proveedor struct {
	Modelo string `json:"modelo"`
	KeyEnv string `json:"key_env"`
}

// Agente es un agente del motor (§8.1 / Fase 1): qué proveedor usa, qué
// modelo, variable de entorno para su API key y habilidades asociadas.
type Agente struct {
	Provider string   `json:"provider"`
	Modelo   string   `json:"model"`
	KeyEnv   string   `json:"key_env,omitempty"`
	Skills   []string `json:"skills,omitempty"`
}

// DefaultForbiddenZones implements §6.3: lockfiles, migrations, CI and
// changelog never belong to a task.
func DefaultForbiddenZones() []string {
	return []string{
		"package-lock.json",
		"yarn.lock",
		"pnpm-lock.yaml",
		"poetry.lock",
		"Pipfile.lock",
		"go.sum",
		"Cargo.lock",
		"migrations/**",
		".github/**",
		"CHANGELOG*",
	}
}

// DefaultTestPatterns are the test paths no contract may claim (adenda
// A.3): en v0.2 las escribe un examinador ciego, así que el
// implementador nunca puede editarlas.
func DefaultTestPatterns() []string {
	return []string{
		"*_test.go",
		"test/**",
		"tests/**",
		"spec/**",
		"*.spec.ts",
		"*.test.ts",
		"test_*.py",
	}
}

// Dir returns the .devclean directory of the repository at root.
func Dir(root string) string { return filepath.Join(root, DirName) }

// Path returns the config.yml path of the repository at root.
func Path(root string) string { return filepath.Join(Dir(root), "config.yml") }

// TasksDir returns the tasks directory of the repository at root.
func TasksDir(root string) string { return filepath.Join(Dir(root), "tasks") }

// Exists reports whether root already holds a .devclean directory.
func Exists(root string) bool {
	info, err := os.Stat(Dir(root))
	return err == nil && info.IsDir()
}

// Load reads .devclean/config.yml under root.
func Load(root string) (Config, error) {
	data, err := os.ReadFile(Path(root))
	if errors.Is(err, os.ErrNotExist) {
		return Config{}, errors.New("sin configuración · corre devclean init primero")
	}
	if err != nil {
		return Config{}, err
	}
	return Parse(data)
}

// Save writes the config to .devclean/config.yml under root.
func (c Config) Save(root string) error {
	var b strings.Builder
	fmt.Fprintf(&b, "base: %s\n", c.Base)
	fmt.Fprintf(&b, "pruebas: %s\n", c.Pruebas)
	if c.Cli != "" {
		fmt.Fprintf(&b, "cli: %s\n", c.Cli)
	}
	fmt.Fprintf(&b, "zonas_prohibidas: %s\n", kv.MarshalList(c.ZonasProhibidas))
	if len(c.PatronesPrueba) > 0 {
		fmt.Fprintf(&b, "patrones_prueba: %s\n", kv.MarshalList(c.PatronesPrueba))
	}
	if c.TimeoutEsclusa > 0 {
		fmt.Fprintf(&b, "timeout_esclusa: %d\n", c.TimeoutEsclusa)
	}
	if len(c.Proveedores) > 0 {
		fmt.Fprintf(&b, "proveedores:\n")
		for _, rol := range sortedRoles(c.Proveedores) {
			p := c.Proveedores[rol]
			fmt.Fprintf(&b, "  %s: { modelo: %s, key_env: %s }\n", rol, kv.Quote(p.Modelo), kv.Quote(p.KeyEnv))
		}
	}
	if len(c.Agentes) > 0 {
		fmt.Fprintf(&b, "agentes:\n")
		for _, nombre := range sortedAgentNames(c.Agentes) {
			a := c.Agentes[nombre]
			var parts []string
			if a.Provider != "" {
				parts = append(parts, fmt.Sprintf("provider: %s", kv.Quote(a.Provider)))
			}
			if a.Modelo != "" {
				parts = append(parts, fmt.Sprintf("model: %s", kv.Quote(a.Modelo)))
			}
			if a.KeyEnv != "" {
				parts = append(parts, fmt.Sprintf("key_env: %s", kv.Quote(a.KeyEnv)))
			}
			if len(a.Skills) > 0 {
				parts = append(parts, fmt.Sprintf("skills: %s", kv.MarshalList(a.Skills)))
			}
			fmt.Fprintf(&b, "  %s: { %s }\n", nombre, strings.Join(parts, ", "))
		}
	}
	if c.Estrategia != "" {
		fmt.Fprintf(&b, "estrategia: %s\n", c.Estrategia)
	}
	if len(c.ReglasImport) > 0 {
		fmt.Fprintf(&b, "reglas_import: %s\n", kv.MarshalList(c.ReglasImport))
	}
	if len(c.Modelos) > 0 {
		fmt.Fprintf(&b, "modelos:\n")
		for _, peso := range sortedStringKeys(c.Modelos) {
			fmt.Fprintf(&b, "  %s: %s\n", peso, kv.Quote(c.Modelos[peso]))
		}
	}
	return os.WriteFile(Path(root), []byte(b.String()), 0o644)
}

// sortedStringKeys devuelve las claves de un mapa en orden estable.
func sortedStringKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// sortedRoles devuelve los roles en orden estable para un Save
// determinista.
func sortedRoles(m map[string]Proveedor) []string {
	roles := make([]string, 0, len(m))
	for r := range m {
		roles = append(roles, r)
	}
	sort.Strings(roles)
	return roles
}

// sortedAgentNames devuelve los nombres de agentes en orden estable para Save.
func sortedAgentNames(m map[string]Agente) []string {
	names := make([]string, 0, len(m))
	for n := range m {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// ModeloRol devuelve el modelo declarado para un rol (planificador,
// ejecutor, revisor) en la configuración, o "" si no está. Si existe un
// agente con ese nombre en `agentes:`, su modelo tiene prioridad sobre
// `proveedores:`.
func ModeloRol(c Config, rol string) string {
	if a, ok := c.Agentes[rol]; ok && a.Modelo != "" {
		return a.Modelo
	}
	if c.Proveedores == nil {
		return ""
	}
	return c.Proveedores[rol].Modelo
}

// PesoPorDefecto devuelve el peso que usa una tarea sin peso explícito,
// a partir de la estrategia global (Fase 3). Por defecto, media.
func (c Config) PesoPorDefecto() string {
	switch c.Estrategia {
	case "ligera":
		return "liviana"
	case "pesada":
		return "pesada"
	default:
		return "media"
	}
}

// ModeloPeso devuelve el modelo asociado a un peso (liviana/media/pesada)
// en la configuración, o "" si no está declarado.
func (c Config) ModeloPeso(peso string) string {
	if c.Modelos == nil {
		return ""
	}
	return c.Modelos[peso]
}

// Parse reads the config yaml subset. Unknown keys are ignored.
func Parse(data []byte) (Config, error) {
	var cfg Config
	pares, err := kv.Pairs(strings.Split(string(data), "\n"), 1)
	if err != nil {
		return cfg, fmt.Errorf("config.yml: %s", err)
	}
	for _, p := range pares {
		switch p.Key {
		case "base":
			cfg.Base = kv.Unquote(p.Value)
		case "pruebas":
			cfg.Pruebas = kv.Unquote(p.Value)
		case "cli":
			cfg.Cli = kv.Unquote(p.Value)
		case "zonas_prohibidas":
			list, err := kv.ParseList(p.Value)
			if err != nil {
				return cfg, fmt.Errorf("config.yml: línea %d · %s", p.Line, err)
			}
			cfg.ZonasProhibidas = list
		case "patrones_prueba":
			list, err := kv.ParseList(p.Value)
			if err != nil {
				return cfg, fmt.Errorf("config.yml: línea %d · %s", p.Line, err)
			}
			cfg.PatronesPrueba = list
		case "timeout_esclusa":
			seg, err := kv.ParseInt(p.Value)
			if err != nil || seg < 1 {
				return cfg, fmt.Errorf("config.yml: línea %d · timeout_esclusa inválido: %s · segundos, mínimo 1", p.Line, p.Value)
			}
			cfg.TimeoutEsclusa = seg
		case "estrategia":
			cfg.Estrategia = kv.Unquote(p.Value)
		case "reglas_import":
			list, err := kv.ParseList(p.Value)
			if err != nil {
				return cfg, fmt.Errorf("config.yml: línea %d · %s", p.Line, err)
			}
			cfg.ReglasImport = list
		}
	}

	proveedores, err := parseProveedores(data)
	if err != nil {
		return cfg, err
	}
	cfg.Proveedores = proveedores

	agentes, err := parseAgentes(data)
	if err != nil {
		return cfg, err
	}
	cfg.Agentes = agentes

	modelos, err := parseModelos(data)
	if err != nil {
		return cfg, err
	}
	cfg.Modelos = modelos
	return cfg, nil
}

// parseModelos lee el bloque anidado `modelos:` (Fase 3), con un peso
// por línea: `liviana: glm-4`.
func parseModelos(data []byte) (map[string]string, error) {
	children, err := kv.Nested(strings.Split(string(data), "\n"), "modelos", 1)
	if err != nil {
		return nil, fmt.Errorf("config.yml: %s", err)
	}
	if len(children) == 0 {
		return nil, nil
	}
	out := make(map[string]string, len(children))
	for _, c := range children {
		out[c.Key] = kv.Unquote(c.Value)
	}
	return out, nil
}

// parseProveedores lee el bloque anidado `proveedores:` (§8.1), con un
// rol por línea: `planificador: { modelo: X, key_env: Y }`.
func parseProveedores(data []byte) (map[string]Proveedor, error) {
	children, err := kv.Nested(strings.Split(string(data), "\n"), "proveedores", 1)
	if err != nil {
		return nil, fmt.Errorf("config.yml: %s", err)
	}
	if len(children) == 0 {
		return nil, nil
	}
	out := make(map[string]Proveedor, len(children))
	for _, c := range children {
		fields, err := kv.ParseInlineMap(c.Value)
		if err != nil {
			return nil, fmt.Errorf("config.yml: línea %d · %s", c.Line, err)
		}
		out[c.Key] = Proveedor{Modelo: fields["modelo"], KeyEnv: fields["key_env"]}
	}
	return out, nil
}

// parseAgentes lee el bloque anidado `agentes:` (Fase 1), con un agente
// por línea: `architect: { provider: claude, model: claude-sonnet, skills: ["diseno"] }`.
func parseAgentes(data []byte) (map[string]Agente, error) {
	children, err := kv.Nested(strings.Split(string(data), "\n"), "agentes", 1)
	if err != nil {
		return nil, fmt.Errorf("config.yml: %s", err)
	}
	if len(children) == 0 {
		return nil, nil
	}
	out := make(map[string]Agente, len(children))
	for _, c := range children {
		fields, err := kv.ParseInlineMap(c.Value)
		if err != nil {
			return nil, fmt.Errorf("config.yml: línea %d · %s", c.Line, err)
		}
		provider := fields["provider"]
		if provider == "" {
			provider = fields["proveedor"]
		}
		if provider != "claude" && provider != "opencode" {
			return nil, fmt.Errorf("config.yml: línea %d · provider desconocido: %s · usa claude u opencode", c.Line, provider)
		}

		modelo := fields["model"]
		if modelo == "" {
			modelo = fields["modelo"]
		}

		agente := Agente{
			Provider: provider,
			Modelo:   modelo,
			KeyEnv:   fields["key_env"],
		}
		if rawSkills, ok := fields["skills"]; ok && rawSkills != "" {
			skills, err := kv.ParseList(rawSkills)
			if err != nil {
				return nil, fmt.Errorf("config.yml: línea %d · %s", c.Line, err)
			}
			agente.Skills = skills
		}
		out[c.Key] = agente
	}
	return out, nil
}

// DefaultAgentes devuelve el catálogo de arquetipos predefinidos con modelos y skills listos para usar.
func DefaultAgentes(cli string) map[string]Agente {
	provider := cli
	if provider == "" {
		provider = "claude"
	}

	modeloSonnet := "claude-sonnet"
	modeloHaiku := "claude-haiku"
	keyEnv := "ANTHROPIC_API_KEY"

	if provider == "opencode" {
		modeloSonnet = "glm-5.2"
		modeloHaiku = "glm-4"
		keyEnv = "OPENCODE_API_KEY"
	}

	return map[string]Agente{
		"ejecutor": {
			Provider: provider,
			Modelo:   modeloSonnet,
			KeyEnv:   keyEnv,
			Skills:   []string{"implementacion", "tdd", "refactor"},
		},
		"backend": {
			Provider: provider,
			Modelo:   modeloSonnet,
			KeyEnv:   keyEnv,
			Skills:   []string{"backend", "api", "database", "sql", "performance"},
		},
		"frontend": {
			Provider: provider,
			Modelo:   modeloSonnet,
			KeyEnv:   keyEnv,
			Skills:   []string{"frontend", "ui", "ux", "components", "css", "state"},
		},
		"architect": {
			Provider: provider,
			Modelo:   modeloSonnet,
			KeyEnv:   keyEnv,
			Skills:   []string{"arquitectura", "diseno", "contratos", "clean-code"},
		},
		"tester": {
			Provider: provider,
			Modelo:   modeloHaiku,
			KeyEnv:   keyEnv,
			Skills:   []string{"testing", "cobertura", "edge-cases", "examinador"},
		},
		"refactor": {
			Provider: provider,
			Modelo:   modeloSonnet,
			KeyEnv:   keyEnv,
			Skills:   []string{"refactoring", "simplificacion", "deuda-tecnica"},
		},
	}
}

// ObtenerAgente resuelve un agente por nombre buscando primero en la configuración
// personalizada del usuario (c.Agentes), luego en los arquetipos predefinidos (DefaultAgentes)
// y finalmente en c.Proveedores.
func (c Config) ObtenerAgente(nombre string) (Agente, bool) {
	if nombre == "" {
		nombre = "ejecutor"
	}

	// 1. Configuración explícita del usuario
	if ag, ok := c.Agentes[nombre]; ok {
		if ag.Provider == "" {
			ag.Provider = c.Cli
		}
		if ag.Provider == "" {
			ag.Provider = "claude"
		}
		return ag, true
	}

	// 2. Arquetipos predefinidos estándar
	defaults := DefaultAgentes(c.Cli)
	if ag, ok := defaults[nombre]; ok {
		return ag, true
	}

	// 3. Bloque proveedores clásico
	if p, ok := c.Proveedores[nombre]; ok {
		provider := c.Cli
		if provider == "" {
			provider = "claude"
		}
		return Agente{
			Provider: provider,
			Modelo:   p.Modelo,
			KeyEnv:   p.KeyEnv,
		}, true
	}

	return Agente{}, false
}

// TodosLosAgentes devuelve el mapa completo de agentes disponibles, fusionando
// los arquetipos por defecto con las personalizaciones del usuario.
func (c Config) TodosLosAgentes() map[string]Agente {
	res := DefaultAgentes(c.Cli)
	for k, v := range c.Agentes {
		res[k] = v
	}
	return res
}
