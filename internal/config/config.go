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
	"strconv"
	"strings"
)

// DirName is the directory devclean creates at the repository root.
const DirName = ".devclean"

// Config is the content of .devclean/config.yml.
type Config struct {
	Base            string   `json:"base"`
	Pruebas         string   `json:"pruebas"`
	ZonasProhibidas []string `json:"zonas_prohibidas"`
	TimeoutEsclusa  int      `json:"timeout_esclusa"` // segundos para el chequeo "falla hoy"
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
	quoted := make([]string, len(c.ZonasProhibidas))
	for i, z := range c.ZonasProhibidas {
		quoted[i] = strconv.Quote(z)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "base: %s\n", c.Base)
	fmt.Fprintf(&b, "pruebas: %s\n", c.Pruebas)
	fmt.Fprintf(&b, "zonas_prohibidas: [%s]\n", strings.Join(quoted, ", "))
	if c.TimeoutEsclusa > 0 {
		fmt.Fprintf(&b, "timeout_esclusa: %d\n", c.TimeoutEsclusa)
	}
	return os.WriteFile(Path(root), []byte(b.String()), 0o644)
}

// Parse reads the config yaml subset. Unknown keys are ignored.
func Parse(data []byte) (Config, error) {
	var cfg Config
	for n, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(stripComment(raw))
		if line == "" {
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			return cfg, fmt.Errorf("config.yml: línea %d no es clave: valor", n+1)
		}
		value = strings.TrimSpace(value)
		switch strings.TrimSpace(key) {
		case "base":
			cfg.Base = unquote(value)
		case "pruebas":
			cfg.Pruebas = unquote(value)
		case "zonas_prohibidas":
			list, err := parseList(value)
			if err != nil {
				return cfg, fmt.Errorf("config.yml: línea %d · %s", n+1, err)
			}
			cfg.ZonasProhibidas = list
		case "timeout_esclusa":
			seg, err := strconv.Atoi(value)
			if err != nil || seg < 1 {
				return cfg, fmt.Errorf("config.yml: línea %d · timeout_esclusa inválido: %s · segundos, mínimo 1", n+1, value)
			}
			cfg.TimeoutEsclusa = seg
		}
	}
	return cfg, nil
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
