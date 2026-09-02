package main

import (
	"bufio"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Pastranauwu/devclean/internal/config"
	"github.com/Pastranauwu/devclean/internal/ui"
)

// sin repo, sin .devclean y sin commits: `up` lo deja todo listo solo.
func TestPrepararEntornoDesdeCero(t *testing.T) {
	if _, err := elegirEjecutor(""); err != nil {
		t.Skip("sin ejecutor instalado")
	}
	dir := t.TempDir()
	out = ui.New(io.Discard, false)
	t.Setenv("HOME", t.TempDir()) // el CLI escribe .cache/.config; fuera del repo
	t.Setenv("GIT_AUTHOR_NAME", "t")
	t.Setenv("GIT_AUTHOR_EMAIL", "t@t")
	t.Setenv("GIT_COMMITTER_NAME", "t")
	t.Setenv("GIT_COMMITTER_EMAIL", "t@t")

	// .devclean se crea sin skills para no tocar la red desde el test
	if _, err := gitEn(dir, "init", "--quiet"); err != nil {
		t.Skip("sin git utilizable")
	}
	root, _ := filepath.EvalSymlinks(dir)
	if err := runInit(root, "", "go", "", nil, true); err != nil {
		t.Fatal(err)
	}
	// una config rota a propósito: cli inexistente, sin base, modelo inventado
	cfg, _ := config.Load(root)
	cfg.Base, cfg.Cli, cfg.Modelos = "", "nadie", map[string]string{"media": "modelo-inventado"}
	if err := cfg.Save(root); err != nil {
		t.Fatal(err)
	}

	got, cfg, err := prepararEntorno(root, nil, false)
	if err != nil {
		t.Fatalf("prepararEntorno: %v", err)
	}
	if got != root {
		t.Errorf("root = %q, quiere %q", got, root)
	}
	if cfg.Base == "" || cfg.Cli == "" || cfg.Cli == "nadie" {
		t.Errorf("config no saneada: base=%q cli=%q", cfg.Base, cfg.Cli)
	}
	if m := cfg.Modelos["media"]; m == "modelo-inventado" {
		t.Errorf("modelo inventado sobrevivió: %q", m)
	}
	if _, err := gitEn(root, "rev-parse", "--verify", "--quiet", "HEAD"); err != nil {
		t.Error("no hizo el commit inicial")
	}
	// segunda corrida: nada que arreglar, nada que romper
	if _, _, err := prepararEntorno(root, nil, false); err != nil {
		t.Fatalf("segunda corrida: %v", err)
	}
}

// una config que apunta a una rama que no existe (base: master en un
// repo con main) detenía la corrida entera sin arreglo desde el CLI.
func TestPrepararEntornoCorrigeBaseInexistente(t *testing.T) {
	if _, err := elegirEjecutor(""); err != nil {
		t.Skip("sin ejecutor instalado")
	}
	root := repoTemporal(t)
	out = ui.New(io.Discard, false)
	t.Setenv("HOME", t.TempDir())
	os.WriteFile(filepath.Join(root, "a.txt"), []byte("x\n"), 0o644)
	if _, err := gitEn(root, "add", "-A"); err != nil {
		t.Fatal(err)
	}
	if salida, err := gitEn(root, "commit", "-q", "-m", "first"); err != nil {
		t.Fatalf("commit: %v · %s", err, salida)
	}
	rama, _ := gitEn(root, "rev-parse", "--abbrev-ref", "HEAD")
	rama = strings.TrimSpace(rama)

	if err := runInit(root, "go test ./...", "", "", nil, true); err != nil {
		t.Fatal(err)
	}
	cfg, _ := config.Load(root)
	cfg.Base = "no-existe-esta-rama"
	if err := cfg.Save(root); err != nil {
		t.Fatal(err)
	}

	_, cfg, err := prepararEntorno(root, nil, false)
	if err != nil {
		t.Fatalf("prepararEntorno: %v", err)
	}
	if cfg.Base != rama {
		t.Errorf("Base = %q, quiere la rama real %q", cfg.Base, rama)
	}
	if guardada, _ := config.Load(root); guardada.Base != rama {
		t.Errorf("no persistió la corrección: %q", guardada.Base)
	}
}

func TestCommitInicialPreguntaSiHayArchivosAjenos(t *testing.T) {
	root := repoTemporal(t)
	out = ui.New(io.Discard, false)
	os.MkdirAll(filepath.Join(root, config.DirName), 0o755)
	os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n"), 0o644)

	if err := commitInicial(root, nil); err == nil {
		t.Error("sin terminal debió cortar en vez de commitear archivos ajenos")
	}
	if err := commitInicial(root, lector("n\n")); err == nil {
		t.Error("con 'n' debió cortar")
	}
	if err := commitInicial(root, lector("s\n")); err != nil {
		t.Fatalf("con 's': %v", err)
	}
	if _, err := gitEn(root, "rev-parse", "--verify", "--quiet", "HEAD"); err != nil {
		t.Error("no hizo el commit inicial")
	}
}

func TestPrepararEntregaSinRemoto(t *testing.T) {
	root := repoTemporal(t)
	out = ui.New(io.Discard, false)
	if _, err := exec.LookPath("gh"); err != nil {
		t.Skip("sin gh")
	}
	if err := prepararEntrega(root, nil); err == nil || !strings.Contains(err.Error(), "origin") {
		t.Errorf("sin remoto y sin terminal debió cortar pidiendo origin, dio: %v", err)
	}
	if err := prepararEntrega(root, lector("git@example.com:x/y.git\n")); err != nil {
		t.Fatalf("con url: %v", err)
	}
	if _, err := gitEn(root, "remote", "get-url", "origin"); err != nil {
		t.Error("no agregó el remoto")
	}
}

func lector(s string) *bufio.Reader { return bufio.NewReader(strings.NewReader(s)) }
