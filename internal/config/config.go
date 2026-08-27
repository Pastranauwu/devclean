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
	"strings"

	"github.com/Pastranauwu/devclean/internal/kv"
)

// DirName is the directory devclean creates at the repository root.
const DirName = ".devclean"

// Config is the content of .devclean/config.yml.
type Config struct {
	Base            string   `json:"base"`
	Pruebas         string   `json:"pruebas"`
	ZonasProhibidas []string `json:"zonas_prohibidas"`
	TimeoutEsclusa  int      `json:"timeout_esclusa"` // segundos para el chequeo "falla hoy"
	PatronesPrueba  []string `json:"patrones_prueba"` // rutas que ninguna tarea puede editar
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
	fmt.Fprintf(&b, "zonas_prohibidas: %s\n", kv.MarshalList(c.ZonasProhibidas))
	if len(c.PatronesPrueba) > 0 {
		fmt.Fprintf(&b, "patrones_prueba: %s\n", kv.MarshalList(c.PatronesPrueba))
	}
	if c.TimeoutEsclusa > 0 {
		fmt.Fprintf(&b, "timeout_esclusa: %d\n", c.TimeoutEsclusa)
	}
	return os.WriteFile(Path(root), []byte(b.String()), 0o644)
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
		}
	}
	return cfg, nil
}
