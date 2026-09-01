package loop

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Pastranauwu/devclean/internal/room"
	"github.com/Pastranauwu/devclean/internal/task"
)

// agenteFalso simula un ejecutor: por defecto no hace nada; hacer puede
// escribir en el cuarto según la cantidad de invocaciones.
type agenteFalso struct {
	nombre string
	tokens Tokens
	veces  int
	hacer  func(veces int, req Request) (string, int, error)
}

func (a *agenteFalso) Name() string { return a.nombre }

func (a *agenteFalso) Run(_ context.Context, req Request) (Result, error) {
	a.veces++
	if a.hacer == nil {
		return Result{Tokens: a.tokens}, nil
	}
	stdout, code, err := a.hacer(a.veces, req)
	return Result{Stdout: stdout, ExitCode: code, Tokens: a.tokens}, err
}

func tareaDePrueba() task.Task {
	return task.Task{
		Version:        task.Version,
		ID:             "T-001",
		Titulo:         "exportar",
		ListoCuando:    "test -f src/done.txt",
		TocarSolo:      []string{"src/**"},
		LimiteIntentos: 3,
		LimiteLineas:   200,
	}
}

func optsDePrueba(t *testing.T, root string, ag Agent, tk task.Task) Options {
	t.Helper()
	r, err := room.Create(context.Background(), root, tk.ID, "main")
	if err != nil {
		t.Fatalf("room.Create: %v", err)
	}
	t.Cleanup(func() { _ = room.Destroy(context.Background(), root, tk.ID) })
	return Options{
		Agent:          ag,
		Root:           root,
		Room:           r,
		Task:           tk,
		Model:          "falso-1",
		Base:           "main",
		AgentTimeout:   5 * time.Second,
		PruebaTimeout:  5 * time.Second,
		PatronesPrueba: []string{"*_test.go", "test/**"},
	}
}

func TestRunVerdeEnSegundoIntento(t *testing.T) {
	root := repoConCommit(t)
	ag := &agenteFalso{nombre: "falso", tokens: Tokens{Entrada: 10, Salida: 5}}
	ag.hacer = func(veces int, req Request) (string, int, error) {
		if veces == 2 {
			escribir(t, req.RoomPath, "src/done.txt", "x\n")
		}
		return "", 0, nil
	}

	outcome, err := Run(context.Background(), optsDePrueba(t, root, ag, tareaDePrueba()))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !outcome.Verde || outcome.Intentos != 2 {
		t.Fatalf("Outcome = %+v, quiero verde en el intento 2", outcome)
	}

	attempts, err := ReadAttempts(root, "T-001")
	if err != nil {
		t.Fatal(err)
	}
	if len(attempts) != 2 {
		t.Fatalf("attempts = %d, quiero 2", len(attempts))
	}
	if attempts[0].SalidaCodigo == nil || *attempts[0].SalidaCodigo != 1 {
		t.Errorf("intento 1 salida_codigo = %v, quiero 1", attempts[0].SalidaCodigo)
	}
	if attempts[1].SalidaCodigo == nil || *attempts[1].SalidaCodigo != 0 {
		t.Errorf("intento 2 salida_codigo = %v, quiero 0", attempts[1].SalidaCodigo)
	}
	if len(attempts[1].ArchivosTocados) != 1 || attempts[1].ArchivosTocados[0] != "src/done.txt" {
		t.Errorf("intento 2 archivos = %v", attempts[1].ArchivosTocados)
	}
	if attempts[1].LineasMas != 1 {
		t.Errorf("intento 2 lineas_mas = %d, quiero 1", attempts[1].LineasMas)
	}
	if attempts[0].Tokens.Entrada != 10 || attempts[1].Modelo != "falso-1" {
		t.Errorf("tokens/modelo mal anotados: %+v", attempts)
	}
}

func TestRunAgotaIntentos(t *testing.T) {
	root := repoConCommit(t)
	ag := &agenteFalso{nombre: "falso"}
	tk := tareaDePrueba()
	tk.ListoCuando = "false"
	tk.LimiteIntentos = 2

	outcome, err := Run(context.Background(), optsDePrueba(t, root, ag, tk))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if outcome.Verde {
		t.Fatal("una tarea que siempre falla salió verde")
	}
	if outcome.Intentos != 2 {
		t.Errorf("Intentos = %d, quiero 2", outcome.Intentos)
	}
	if !strings.Contains(outcome.Pregunta, "agotó 2 intentos") {
		t.Errorf("Pregunta = %q, quiero la pregunta concreta", outcome.Pregunta)
	}

	attempts, err := ReadAttempts(root, "T-001")
	if err != nil {
		t.Fatal(err)
	}
	if len(attempts) != 2 {
		t.Fatalf("attempts = %d, quiero 2 (uno por intento)", len(attempts))
	}
}

func TestRunRevierteArchivosFueraDeAlcance(t *testing.T) {
	root := repoConCommit(t)
	ag := &agenteFalso{nombre: "falso"}
	ag.hacer = func(veces int, req Request) (string, int, error) {
		escribir(t, req.RoomPath, "src/done.txt", "dentro\n")
		escribir(t, req.RoomPath, "docs/notas.txt", "fuera\n")
		return "", 0, nil
	}

	outcome, err := Run(context.Background(), optsDePrueba(t, root, ag, tareaDePrueba()))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !outcome.Verde {
		t.Fatalf("Outcome = %+v, quiero verde", outcome)
	}

	attempts, _ := ReadAttempts(root, "T-001")
	if len(attempts) != 1 {
		t.Fatalf("attempts = %d, quiero 1", len(attempts))
	}
	revertidos := attempts[0].RevertidosFueraDeAlcance
	if len(revertidos) != 1 || revertidos[0] != "docs/notas.txt" {
		t.Errorf("revertidos = %v, quiero [docs/notas.txt]", revertidos)
	}
	if _, err := os.Stat(filepath.Join(root, ".devclean", "rooms", "T-001", "docs", "notas.txt")); !os.IsNotExist(err) {
		t.Error("el archivo fuera de alcance quedó en el cuarto")
	}
}

func TestAttemptsJSONListasVacias(t *testing.T) {
	root := repoConCommit(t)
	ag := &agenteFalso{nombre: "falso"}
	tk := tareaDePrueba()
	tk.ListoCuando = "false"
	tk.LimiteIntentos = 1

	if _, err := Run(context.Background(), optsDePrueba(t, root, ag, tk)); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(RunsDir(root), "T-001", "attempts.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if !strings.Contains(s, `"archivos_tocados":[]`) {
		t.Errorf("archivos vacíos no serializan []: %s", s)
	}
	if !strings.Contains(s, `"revertidos_fuera_de_alcance":[]`) {
		t.Errorf("revertidos vacíos no serializan []: %s", s)
	}
	if !strings.Contains(s, `"simbolos_exportados":null`) {
		t.Errorf("símbolos sin soportar no serializan null: %s", s)
	}
	if !strings.Contains(s, `"tests_pasaron":null`) {
		t.Errorf("tests sin parsear no serializan null: %s", s)
	}
}

func TestPromptIncluyeContratoYErrorPrevio(t *testing.T) {
	tk := tareaDePrueba()
	p := promptPara(tk, nil, "", nil, "", "fallo de prueba")
	for _, want := range []string{"Tarea T-001", "Listo cuando: test -f src/done.txt", "Solo puedes tocar", "El intento anterior falló", "fallo de prueba"} {
		if !strings.Contains(p, want) {
			t.Errorf("prompt sin %q:\n%s", want, p)
		}
	}
}

func TestPromptIncluyeSkills(t *testing.T) {
	tk := tareaDePrueba()
	p := promptPara(tk, nil, "", []string{"go", "refactor"}, "", "")
	want := "Habilidades de este rol: go, refactor"
	if !strings.Contains(p, want) {
		t.Errorf("prompt sin %q:\n%s", want, p)
	}
}

func TestPromptIncluyeContenidoDeSkills(t *testing.T) {
	tk := tareaDePrueba()
	p := promptPara(tk, nil, "", nil, "sé breve y directo", "")
	if !strings.Contains(p, "sé breve y directo") {
		t.Errorf("prompt sin el contenido de la skill:\n%s", p)
	}
}
