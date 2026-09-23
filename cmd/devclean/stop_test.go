package main

import (
	"github.com/Pastranauwu/devclean/internal/config"
	"os"
	"path/filepath"
	"testing"
)

func TestMarcarCorridaAnidada(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".devclean"), 0o755); err != nil {
		t.Fatal(err)
	}
	soltarUp := marcarCorrida(root)
	soltarRun := marcarCorrida(root) // run adentro de up
	soltarRun()
	if _, err := os.Stat(corridaPath(root)); err != nil {
		t.Fatal("el run de adentro borró la marca de up")
	}
	soltarUp()
	if _, err := os.Stat(corridaPath(root)); !os.IsNotExist(err) {
		t.Fatal("up no soltó la marca al terminar")
	}
}

func TestTomarEntregaEsDeUnoALaVez(t *testing.T) {
	root := t.TempDir()
	soltar, err := tomarEntrega(root)
	if err != nil {
		t.Fatal(err)
	}
	// el mismo proceso (up que llama a ship) no se bloquea a sí mismo
	if _, err := tomarEntrega(root); err != nil {
		t.Fatalf("se bloqueó a sí mismo: %v", err)
	}
	soltar()
	// marca de un proceso muerto: se toma encima
	p := filepath.Join(root, ".devclean", "entrega.pid")
	if err := os.WriteFile(p, []byte("999999999"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := tomarEntrega(root); err != nil {
		t.Fatalf("una marca rancia bloqueó la entrega: %v", err)
	}
}

// Repo que arrancó vacío: el package.json solo existe en el cuarto.
func TestPruebasDeLosCuartos(t *testing.T) {
	root := t.TempDir()
	cuarto := filepath.Join(root, ".devclean", "rooms", "T-002")
	if err := os.MkdirAll(cuarto, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cuarto, "package.json"), []byte(`{"scripts":{"test":"node --test"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	var cfg config.Config
	pruebasDeLosCuartos(root, &cfg, []string{"T-001", "T-002"})
	if cfg.Pruebas != "npm test" {
		t.Fatalf("pruebas = %q, quiero npm test", cfg.Pruebas)
	}
	// lo que el humano declaró no se pisa
	cfg.Pruebas = "make check"
	pruebasDeLosCuartos(root, &cfg, []string{"T-002"})
	if cfg.Pruebas != "make check" {
		t.Fatalf("pisó el comando del humano: %q", cfg.Pruebas)
	}
}
