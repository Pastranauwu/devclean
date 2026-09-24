package spec

import (
	"strings"
	"testing"

	"github.com/Pastranauwu/devclean/internal/task"
)

func planCalculadora() []task.Task {
	return []task.Task{
		{Version: task.Version, ID: "T-001", Titulo: "lexer", ListoCuando: "go test ./internal/lexer/...", TocarSolo: []string{"internal/lexer/**"}, Expone: []string{"lexer.Tokenize(s string) []Token"}},
		{Version: task.Version, ID: "T-002", Titulo: "parser", ListoCuando: "go test ./internal/parser/...", TocarSolo: []string{"internal/parser/**"}, DependeDe: []string{"T-001"}, Expone: []string{"parser.Parse(t []Token) Node"}, Usa: []string{"lexer.Tokenize(s string) []Token"}},
		{Version: task.Version, ID: "T-003", Titulo: "evaluador", ListoCuando: "go test ./internal/eval/...", TocarSolo: []string{"internal/eval/**"}, DependeDe: []string{"T-002"}, Expone: []string{"eval.Eval(n Node) float64"}, Usa: []string{"parser.Parse(t []Token) Node"}},
	}
}

// La costura: cinco tareas verdes y `-2^2` roto porque el evaluador nunca
// pidió el operador que el lexer prometió. La tarea derivada entra por la
// frontera final y su comando vuelve como aceptación del feature.
func TestTareaDeIntegracionCierraLaCostura(t *testing.T) {
	s := Spec{Version: 1, Feature: "calculadora", Requirements: []string{"evaluar potencias con signo"}}
	tasks := planCalculadora()

	got, aceptacion, ok := TareaDeIntegracion(s, tasks, "go", "T-004")
	if !ok {
		t.Fatal("con cadena de tareas y sin aceptación ejecutable, la costura necesita prueba")
	}
	if got.ListoCuando != "go test ./test/integracion/t-004/..." {
		t.Errorf("listo_cuando = %q", got.ListoCuando)
	}
	if aceptacion.Command != got.ListoCuando {
		t.Errorf("la aceptación corre el mismo comando sobre el conjunto: %q", aceptacion.Command)
	}
	// corre al final: depende de todas
	if len(got.DependeDe) != 3 || got.DependeDe[0] != "T-001" || got.DependeDe[2] != "T-003" {
		t.Errorf("depende_de = %v", got.DependeDe)
	}
	// consume todo lo que el plan promete, y no expone nada: sin expone no
	// hay examen ciego que proteger, y por eso puede tocar rutas de prueba
	if len(got.Usa) != 3 {
		t.Errorf("usa = %v", got.Usa)
	}
	if len(got.Expone) != 0 {
		t.Errorf("la tarea de integración no expone nada: %v", got.Expone)
	}
	// un solo archivo con tope de líneas: sin eso la del snake escribió
	// 1576 líneas que repetían la suite de cada tarea
	if len(got.TocarSolo) != 1 || got.TocarSolo[0] != "test/integracion/t-004/costura_test.go" {
		t.Errorf("tocar_solo = %v", got.TocarSolo)
	}
	if got.LimiteLineas != LimiteLineasIntegracion {
		t.Errorf("limite_lineas = %d", got.LimiteLineas)
	}
	// las notas llevan la derivación: cada costura, lo que prometió cada
	// tarea y el requerimiento como contexto
	for _, quiero := range []string{"T-002 usa de T-001: Tokenize", "T-003 usa de T-002: Parse", "evaluar potencias con signo", "eval.Eval(n Node) float64", "nunca le pidió", "costura_test.go", "no repitas"} {
		if !strings.Contains(got.Notas, quiero) {
			t.Errorf("las notas no llevan %q:\n%s", quiero, got.Notas)
		}
	}
}

// En un repo vacío no hay lenguaje que detectar: el stack lo eligió el plan
// y sus comandos son la única evidencia.
func TestTareaDeIntegracionDeduceElStackDelPlan(t *testing.T) {
	for _, c := range []struct{ comando, quiero string }{
		{"pytest tests/test_lexer.py", "pytest test/integracion/t-004"},
		{"npm test -- lexer", "node --test test/integracion/t-004/"},
		{"go test ./internal/lexer/...", "go test ./test/integracion/t-004/..."},
	} {
		tasks := planCalculadora()
		tasks[0].ListoCuando = c.comando
		tasks[1].ListoCuando = c.comando
		tasks[2].ListoCuando = c.comando
		got, _, ok := TareaDeIntegracion(Spec{Version: 1, Feature: "x"}, tasks, "", "T-004")
		if !ok || got.ListoCuando != c.quiero {
			t.Errorf("%q → %q (ok=%v), quiero %q", c.comando, got.ListoCuando, ok, c.quiero)
		}
	}
}

func TestTareaDeIntegracionNoInventaCuandoNoToca(t *testing.T) {
	base := Spec{Version: 1, Feature: "calculadora"}
	casos := []struct {
		nombre string
		s      Spec
		tasks  []task.Task
		leng   string
	}{
		{"una sola tarea no tiene costura", base, planCalculadora()[:1], "go"},
		{"sin relación entre tareas tampoco", base, []task.Task{
			{Version: task.Version, ID: "T-001", Titulo: "a", ListoCuando: "true", Expone: []string{"a.A()"}},
			{Version: task.Version, ID: "T-002", Titulo: "b", ListoCuando: "true", Expone: []string{"b.B()"}},
		}, "go"},
		{"con aceptación del humano la costura es suya", Spec{Version: 1, Feature: "calculadora",
			Acceptance: []Acceptance{{Criterion: "todo junto", Command: "make e2e"}}}, planCalculadora(), "go"},
		{"un stack sin comando conocido no se inventa", base, planCalculadora(), "rust"},
	}
	for _, c := range casos {
		if _, _, ok := TareaDeIntegracion(c.s, c.tasks, c.leng, "T-004"); ok {
			t.Errorf("%s: no debía derivar tarea", c.nombre)
		}
	}
}

// Un stack desconocido con comandos que tampoco lo delatan no deriva nada:
// mejor sin prueba de costura que con un comando inventado que nunca pasa.
func TestTareaDeIntegracionSinIdsNoPuedeOrdenarse(t *testing.T) {
	tasks := planCalculadora()
	tasks[1].ID = ""
	if _, _, ok := TareaDeIntegracion(Spec{Version: 1, Feature: "x"}, tasks, "go", "T-004"); ok {
		t.Error("sin ids no se puede declarar depende_de")
	}
}

// El planificador ya escribe sus propias pruebas de integración: la tarea
// derivada vive en su propio directorio para no cruzar con ellas, y si el
// plan reclamó la zona entera, la costura es de esa tarea.
func TestTareaDeIntegracionNoCruzaConElPlan(t *testing.T) {
	tasks := planCalculadora()
	tasks[2].TocarSolo = append(tasks[2].TocarSolo, "test/integracion/uxui.test.js")
	got, _, ok := TareaDeIntegracion(Spec{Version: 1, Feature: "x"}, tasks, "node", "T-004")
	if !ok {
		t.Fatal("un archivo del plan en test/integracion no impide la costura")
	}
	for _, issue := range ValidatePlan(Spec{}, append(tasks, got)) {
		if issue.Code == "write_overlap" {
			t.Errorf("la tarea derivada cruza con el plan: %s", issue.Message)
		}
	}
	tasks[2].TocarSolo = []string{"test/integracion/**"}
	if _, _, ok := TareaDeIntegracion(Spec{Version: 1, Feature: "x"}, tasks, "node", "T-004"); ok {
		t.Error("el plan reclamó la zona entera: derivar otra tarea ahí es un plan inválido")
	}
}
