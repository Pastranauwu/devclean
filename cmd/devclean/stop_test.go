package main

import (
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
