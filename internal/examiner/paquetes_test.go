package examiner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func cuartoConAst(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	escribirArchivo(t, root, "go.mod", "module github.com/x/calc\n\ngo 1.22\n")
	escribirArchivo(t, root, "internal/ast/ast.go", "package ast\n\ntype Number struct{ Value float64 }\n")
	escribirArchivo(t, root, "internal/eval/eval.go", "package eval\n")
	return root
}

func escribirArchivo(t *testing.T, root, rel, contenido string) {
	t.Helper()
	abs := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte(contenido), 0o644); err != nil {
		t.Fatal(err)
	}
}

// El examinador es un modelo: escribe `ast.Number{}` y se olvida del
// import. Esa suite no compila nunca y el implementador no puede
// arreglarla (A.3), así que quemaba todos sus intentos contra
// "undefined: ast".
func TestImportsFaltantesResuelveElModulo(t *testing.T) {
	root := cuartoConAst(t)
	contenido := `package eval_test

import (
	"testing"
	"github.com/x/calc/internal/eval"
)

func TestEval(t *testing.T) {
	env := eval.NewEnv()
	got, err := env.Eval(ast.Number{Value: 2})
	if err != nil || got != 2 {
		t.Fatal(math.Abs(got))
	}
}
`
	rutas, sinResolver := importsFaltantes(root, contenido)
	if len(sinResolver) != 0 {
		t.Fatalf("sin resolver = %v, quiero ninguno", sinResolver)
	}
	quiero := map[string]bool{"github.com/x/calc/internal/ast": false, "math": false}
	for _, r := range rutas {
		if _, ok := quiero[r]; !ok {
			t.Errorf("ruta inesperada: %s", r)
		}
		quiero[r] = true
	}
	for r, visto := range quiero {
		if !visto {
			t.Errorf("falta resolver %s · rutas=%v", r, rutas)
		}
	}
}

// `env.Eval(...)` con env local no es un paquete: agregarle un import
// rompería la suite en vez de arreglarla.
func TestImportsFaltantesIgnoraVariablesLocales(t *testing.T) {
	root := cuartoConAst(t)
	contenido := `package eval_test

import (
	"testing"
	"github.com/x/calc/internal/eval"
)

func TestEval(t *testing.T) {
	env := eval.NewEnv()
	if _, err := env.Eval(nil); err == nil {
		t.Fatal("quiero error")
	}
	for i, c := range []int{1, 2} {
		_ = i
		_ = c
	}
}
`
	rutas, sinResolver := importsFaltantes(root, contenido)
	if len(rutas) != 0 || len(sinResolver) != 0 {
		t.Fatalf("rutas=%v sinResolver=%v, quiero nada que agregar", rutas, sinResolver)
	}
}

func TestSuiteCompletaDescartaLoQueNoResuelve(t *testing.T) {
	root := cuartoConAst(t)
	funcs := []string{"func TestX(t *testing.T) {\n\t_ = desconocido.Cosa{}\n}"}
	if _, ok := suiteCompleta("go", root, "eval", "github.com/x/calc/internal/eval", nil, funcs); ok {
		t.Error("una suite que referencia un paquete inexistente no debe escribirse")
	}
}

// El import hermano se conservaba solo si la ruta del módulo no tenía
// punto: con "github.com/x/calc" buildGoFile lo tiraba por no parecer
// stdlib y la suite quedaba sin compilar.
func TestSuiteCompletaConservaElImportDelModulo(t *testing.T) {
	root := cuartoConAst(t)
	funcs := []string{"func TestX(t *testing.T) {\n\t_ = ast.Number{Value: 1}\n}"}
	contenido, ok := suiteCompleta("go", root, "eval", "github.com/x/calc/internal/eval", nil, funcs)
	if !ok {
		t.Fatal("la suite debía completarse con el import del módulo")
	}
	if !strings.Contains(contenido, `"github.com/x/calc/internal/ast"`) {
		t.Errorf("falta el import hermano en:\n%s", contenido)
	}
}
