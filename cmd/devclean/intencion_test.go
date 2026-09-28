package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Pastranauwu/devclean/internal/spec"
	"github.com/Pastranauwu/devclean/internal/ui"
)

func TestDeltaRequirements(t *testing.T) {
	nuevos, hechos, retirados := deltaRequirements(
		[]string{"login", "exportar csv", "reporte pdf"},
		[]string{"login", "exportar csv con filtros", "reporte pdf"},
	)
	if strings.Join(nuevos, "|") != "exportar csv con filtros" ||
		strings.Join(hechos, "|") != "login|reporte pdf" ||
		strings.Join(retirados, "|") != "exportar csv" {
		t.Fatalf("nuevos=%v hechos=%v retirados=%v", nuevos, hechos, retirados)
	}
	if n, _, _ := deltaRequirements(nil, []string{"a"}); len(n) != 1 {
		t.Fatalf("sin plan previo todo es nuevo: %v", n)
	}
}

// un up sobre un spec de requirements que no cambió no llama al
// arquitecto: antes lo pagaba otra vez y duplicaba las tareas
func TestSpecSinCambiosNoReplanea(t *testing.T) {
	out = ui.New(io.Discard, false)
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".devclean"), 0o755); err != nil {
		t.Fatal(err)
	}
	ruta := filepath.Join(root, "devclean.spec.yml")
	yml := "feature: gastos\nrequirements:\n  - registrar un gasto\n  - listar gastos del mes\n"
	if err := os.WriteFile(ruta, []byte(yml), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := spec.Load(ruta)
	if err != nil {
		t.Fatal(err)
	}
	if err := spec.SaveIntencion(root, spec.IntencionDe(s)); err != nil {
		t.Fatal(err)
	}
	// sin config ni ejecutor: si intentara planear, fallaría
	got, err := runApply(root, ruta, false, false)
	if err != nil {
		t.Fatalf("replaneó un spec sin cambios: %v", err)
	}
	if got.Feature != "gastos" {
		t.Errorf("feature = %q", got.Feature)
	}
	if es, _ := os.ReadDir(filepath.Join(root, ".devclean", "tasks")); len(es) != 0 {
		t.Errorf("escribió %d tareas", len(es))
	}
}
