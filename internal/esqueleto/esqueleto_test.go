package esqueleto

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Pastranauwu/devclean/internal/plan"
)

func escribir(t *testing.T, dir, rel, contenido string) {
	t.Helper()
	p := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(contenido), 0o644); err != nil {
		t.Fatal(err)
	}
}

// un esqueleto de python con un stub, su prueba y una prueba rota a
// propósito por cada forma en que un esqueleto sale mal
func esqueletoPython(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("sin python3")
	}
	dir := t.TempDir()
	if out, err := exec.Command("git", "-C", dir, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, out)
	}
	escribir(t, dir, Documento, "# arquitectura\n")
	escribir(t, dir, "calc/__init__.py", "")
	escribir(t, dir, "calc/suma.py", "def suma(a: int, b: int) -> int:\n    \"\"\"Suma a y b.\"\"\"\n    raise NotImplementedError(\""+Marca+"\")\n")
	escribir(t, dir, "tests/__init__.py", "")
	escribir(t, dir, "tests/test_suma.py", "import unittest\nfrom calc.suma import suma\n\nclass T(unittest.TestCase):\n    def test_suma(self):\n        self.assertEqual(suma(2, 3), 5)\n")
	escribir(t, dir, "tests/test_roto.py", "import unittest\nfrom calc.resta import resta\n\nclass T(unittest.TestCase):\n    def test_resta(self):\n        self.assertEqual(resta(3, 2), 1)\n")
	escribir(t, dir, "tests/test_vacuo.py", "import unittest\n\nclass T(unittest.TestCase):\n    def test_nada(self):\n        pass\n")
	return dir
}

func tarea(titulo, listo string, tocar ...string) plan.Borrador {
	return plan.Borrador{Titulo: titulo, ListoCuando: listo, TocarSolo: tocar}
}

func TestProblemasAceptaUnEsqueletoQueFallaPorElStub(t *testing.T) {
	dir := esqueletoPython(t)
	r := Resultado{
		Verificar: "python3 -m compileall -q calc",
		Tareas:    []plan.Borrador{tarea("suma", "python3 -m unittest tests/test_suma.py", "calc/suma.py")},
	}
	if ps := Problemas(context.Background(), dir, r, time.Minute, nil); len(ps) > 0 {
		t.Fatalf("esqueleto bueno rechazado: %v", ps)
	}
}

func TestProblemasDetectaCadaFormaDeEsqueletoMalo(t *testing.T) {
	dir := esqueletoPython(t)
	r := Resultado{
		Verificar: "python3 -c 'raise SystemExit(1)'",
		Tareas: []plan.Borrador{
			tarea("import roto", "python3 -m unittest tests/test_roto.py", "calc/resta.py"),
			tarea("ya pasa", "python3 -m unittest tests/test_vacuo.py", "calc/suma.py"),
			tarea("se toca su prueba", "python3 -m unittest tests/test_suma.py", "calc/suma.py", "tests/test_suma.py"),
		},
	}
	todo := strings.Join(Problemas(context.Background(), dir, r, time.Minute, nil), "\n")
	for _, quiero := range []string{
		"\"verificar\"",
		"calc/resta.py no existe",
		"no carga",
		"ya pasa con el stub",
		"incluye su propia prueba",
	} {
		if !strings.Contains(todo, quiero) {
			t.Errorf("falta %q en:\n%s", quiero, todo)
		}
	}
	os.Remove(filepath.Join(dir, Documento))
	if ps := Problemas(context.Background(), dir, r, time.Minute, nil); !strings.Contains(strings.Join(ps, "\n"), Documento) {
		t.Errorf("sin %s no se quejó: %v", Documento, ps)
	}
}

func TestParseLeeVerificarEIntegracion(t *testing.T) {
	r, err := Parse("listo:\n```json\n{\"verificar\": \"go vet ./...\", \"integracion\": \"go test ./e2e/...\", \"arquitectura\": \"hexagonal\nen dos líneas\", \"tareas\": [{\"titulo\": \"a\", \"listo_cuando\": \"go test ./a/...\", \"tocar_solo\": [\"a/a.go\"], \"como\": \"rellena\"}]}\n```")
	if err != nil {
		t.Fatal(err)
	}
	if r.Verificar != "go vet ./..." || r.Integracion != "go test ./e2e/..." || len(r.Tareas) != 1 {
		t.Fatalf("%+v", r)
	}
	if !strings.Contains(r.Tareas[0].Como, "hexagonal") {
		t.Errorf("la arquitectura no llegó a las notas: %q", r.Tareas[0].Como)
	}
}
