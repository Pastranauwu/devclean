package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParseFull(t *testing.T) {
	data := []byte(`
# config de ejemplo
base: main
pruebas: npm test # corre en CI
zonas_prohibidas: ["package-lock.json", "migrations/**"]
unknown_key: se ignora
`)
	cfg, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if cfg.Base != "main" {
		t.Errorf("Base = %q, quiero %q", cfg.Base, "main")
	}
	if cfg.Pruebas != "npm test" {
		t.Errorf("Pruebas = %q, quiero %q", cfg.Pruebas, "npm test")
	}
	want := []string{"package-lock.json", "migrations/**"}
	if !reflect.DeepEqual(cfg.ZonasProhibidas, want) {
		t.Errorf("ZonasProhibidas = %v, quiero %v", cfg.ZonasProhibidas, want)
	}
}

func TestParseBadLine(t *testing.T) {
	_, err := Parse([]byte("esto no es clave valor\n"))
	if err == nil {
		t.Fatal("Parse debió fallar con una línea sin dos puntos")
	}
}

func TestSaveLoadRoundtrip(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(Dir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := Config{
		Base:            "main",
		Pruebas:         "go test ./...",
		ZonasProhibidas: DefaultForbiddenZones(),
	}
	if err := cfg.Save(root); err != nil {
		t.Fatalf("Save: %v", err)
	}
	loaded, err := Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !reflect.DeepEqual(cfg, loaded) {
		t.Errorf("roundtrip = %+v, quiero %+v", loaded, cfg)
	}
}

func TestCliIdaYVuelta(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(Dir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := Config{Base: "main", Pruebas: "go test ./...", Cli: "claude"}
	if err := cfg.Save(root); err != nil {
		t.Fatalf("Save: %v", err)
	}
	loaded, err := Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Cli != "claude" {
		t.Errorf("Cli = %q, quiero claude", loaded.Cli)
	}
}

func TestParseTimeoutEsclusa(t *testing.T) {
	cfg, err := Parse([]byte("timeout_esclusa: 120\n"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if cfg.TimeoutEsclusa != 120 {
		t.Errorf("TimeoutEsclusa = %d, quiero 120", cfg.TimeoutEsclusa)
	}
	for _, bad := range []string{"0", "-5", "mucho"} {
		if _, err := Parse([]byte("timeout_esclusa: " + bad + "\n")); err == nil {
			t.Errorf("Parse debió rechazar timeout_esclusa: %s", bad)
		}
	}
}

func TestLoadMissing(t *testing.T) {
	_, err := Load(t.TempDir())
	if err == nil {
		t.Fatal("Load debió fallar sin config.yml")
	}
}

func TestExists(t *testing.T) {
	root := t.TempDir()
	if Exists(root) {
		t.Fatal("Exists = true sin .devclean")
	}
	if err := os.MkdirAll(Dir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	if !Exists(root) {
		t.Fatal("Exists = false con .devclean creado")
	}
}

func TestPathsStayUnderRoot(t *testing.T) {
	root := "/proyecto"
	if Path(root) != filepath.Join(root, ".devclean", "config.yml") {
		t.Errorf("Path inesperado: %s", Path(root))
	}
	if TasksDir(root) != filepath.Join(root, ".devclean", "tasks") {
		t.Errorf("TasksDir inesperado: %s", TasksDir(root))
	}
}

func TestPatronesPruebaIdaYVuelta(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(Dir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := Config{Base: "main", Pruebas: "go test ./...", PatronesPrueba: DefaultTestPatterns()}
	if err := cfg.Save(root); err != nil {
		t.Fatalf("Save: %v", err)
	}
	loaded, err := Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !reflect.DeepEqual(loaded.PatronesPrueba, DefaultTestPatterns()) {
		t.Errorf("PatronesPrueba = %v", loaded.PatronesPrueba)
	}
}

func TestParseProveedores(t *testing.T) {
	data := []byte(`
base: main
proveedores:
  planificador: { modelo: claude-sonnet, key_env: ANTHROPIC_API_KEY }
  ejecutor:     { modelo: glm-5.2, key_env: OPENCODE_API_KEY }
  revisor:      { modelo: deepseek-v4-flash, key_env: DEEPSEEK_API_KEY }
zonas_prohibidas: ["go.sum"]
`)
	cfg, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(cfg.Proveedores) != 3 {
		t.Fatalf("Proveedores = %v · quiere 3 roles", cfg.Proveedores)
	}
	if cfg.Proveedores["planificador"].Modelo != "claude-sonnet" {
		t.Errorf("planificador = %+v", cfg.Proveedores["planificador"])
	}
	if cfg.Proveedores["ejecutor"].KeyEnv != "OPENCODE_API_KEY" {
		t.Errorf("ejecutor = %+v", cfg.Proveedores["ejecutor"])
	}
	if cfg.Proveedores["revisor"].Modelo != "deepseek-v4-flash" {
		t.Errorf("revisor = %+v", cfg.Proveedores["revisor"])
	}
}

func TestProveedoresIdaYVuelta(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(Dir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := Config{
		Base:    "main",
		Pruebas: "go test ./...",
		Proveedores: map[string]Proveedor{
			"planificador": {Modelo: "claude-sonnet", KeyEnv: "ANTHROPIC_API_KEY"},
			"ejecutor":     {Modelo: "glm-5.2", KeyEnv: "OPENCODE_API_KEY"},
		},
	}
	if err := cfg.Save(root); err != nil {
		t.Fatalf("Save: %v", err)
	}
	loaded, err := Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !reflect.DeepEqual(cfg.Proveedores, loaded.Proveedores) {
		t.Errorf("Proveedores = %+v, quiero %+v", loaded.Proveedores, cfg.Proveedores)
	}
}

func TestModeloRol(t *testing.T) {
	cfg := Config{Proveedores: map[string]Proveedor{
		"planificador": {Modelo: "claude-sonnet"},
	}}
	if got := ModeloRol(cfg, "planificador"); got != "claude-sonnet" {
		t.Errorf("ModeloRol(planificador) = %q", got)
	}
	if got := ModeloRol(cfg, "ejecutor"); got != "" {
		t.Errorf("ModeloRol(ejecutor) = %q, quiere vacío", got)
	}
	if got := ModeloRol(Config{}, "planificador"); got != "" {
		t.Errorf("ModeloRol sin config = %q, quiere vacío", got)
	}
}

func TestParseEstrategiaYModelos(t *testing.T) {
	data := []byte(`
estrategia: pesada
modelos:
  liviana: glm-4
  media: glm-5.2
  pesada: claude-sonnet
`)
	cfg, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if cfg.Estrategia != "pesada" {
		t.Errorf("Estrategia = %q", cfg.Estrategia)
	}
	if cfg.Modelos["pesada"] != "claude-sonnet" || cfg.Modelos["liviana"] != "glm-4" {
		t.Errorf("Modelos = %v", cfg.Modelos)
	}
}

func TestModelosIdaYVuelta(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(Dir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := Config{
		Base:       "main",
		Pruebas:    "go test ./...",
		Estrategia: "equilibrada",
		Modelos:    map[string]string{"liviana": "glm-4", "media": "glm-5.2", "pesada": "claude-sonnet"},
	}
	if err := cfg.Save(root); err != nil {
		t.Fatalf("Save: %v", err)
	}
	loaded, err := Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !reflect.DeepEqual(cfg.Modelos, loaded.Modelos) || loaded.Estrategia != cfg.Estrategia {
		t.Errorf("roundtrip = %+v / %q", loaded.Modelos, loaded.Estrategia)
	}
}

func TestPesoPorDefecto(t *testing.T) {
	cases := map[string]string{"ligera": "liviana", "equilibrada": "media", "pesada": "pesada", "": "media"}
	for estrategia, want := range cases {
		cfg := Config{Estrategia: estrategia}
		if got := cfg.PesoPorDefecto(); got != want {
			t.Errorf("PesoPorDefecto(%q) = %q, quiero %q", estrategia, got, want)
		}
	}
}

func TestModeloPeso(t *testing.T) {
	cfg := Config{Modelos: map[string]string{"pesada": "claude-sonnet"}}
	if got := cfg.ModeloPeso("pesada"); got != "claude-sonnet" {
		t.Errorf("ModeloPeso(pesada) = %q", got)
	}
	if got := cfg.ModeloPeso("liviana"); got != "" {
		t.Errorf("ModeloPeso(liviana) = %q, quiere vacío", got)
	}
	if got := (Config{}).ModeloPeso("media"); got != "" {
		t.Errorf("ModeloPeso sin config = %q, quiere vacío", got)
	}
}
