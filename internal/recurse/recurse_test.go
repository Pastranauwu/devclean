package recurse

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/Pastranauwu/devclean/internal/loop"
	"github.com/Pastranauwu/devclean/internal/plan"
	"github.com/Pastranauwu/devclean/internal/room"
	"github.com/Pastranauwu/devclean/internal/task"
)

func gitCmd(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

func repoConCommit(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	gitCmd(t, root, "init", "-b", "main")
	gitCmd(t, root, "-c", "user.email=t@t", "-c", "user.name=t", "commit", "--allow-empty", "-m", "init")
	return root
}

// generadorFalso simula al planificador: devuelve el texto fijo que se le
// dio, ignorando el prompt real.
type generadorFalso struct{ texto string }

func (g generadorFalso) Generar(_ context.Context, _ string) (string, error) { return g.texto, nil }

// ejecutorFalso simula al agente hoja: escribe un archivo fijo en el
// cuarto que se le dio, para que el listo_cuando de la subtarea pase.
type ejecutorFalso struct{}

func (ejecutorFalso) Name() string { return "falso" }

func (ejecutorFalso) Run(_ context.Context, req loop.Request) (loop.Result, error) {
	abs := filepath.Join(req.RoomPath, "src", "done.txt")
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return loop.Result{}, err
	}
	if err := os.WriteFile(abs, []byte("ok"), 0o644); err != nil {
		return loop.Result{}, err
	}
	return loop.Result{Tokens: loop.Tokens{Entrada: 1, Salida: 1}}, nil
}

func tareaRecursivaDePrueba() task.Task {
	return task.Task{
		Version:         task.Version,
		ID:              "T-100",
		Titulo:          "feature grande",
		ListoCuando:     "test -f src/done.txt",
		TocarSolo:       []string{"src/**"},
		LimiteIntentos:  3,
		LimiteLineas:    200,
		Recursivo:       true,
		LimiteSubtareas: 2,
	}
}

func TestAgentRunDescomponeYIntegra(t *testing.T) {
	root := repoConCommit(t)
	parentTask := tareaRecursivaDePrueba()
	parentRoom, err := room.Create(context.Background(), root, parentTask.ID, "main")
	if err != nil {
		t.Fatalf("room.Create padre: %v", err)
	}
	t.Cleanup(func() { _ = room.Destroy(context.Background(), root, parentTask.ID) })

	texto := `[{"titulo": "crear done.txt", "listo_cuando": "test -f src/done.txt", "tocar_solo": ["src/**"]}]`
	a := Agent{
		Planificador: generadorFalso{texto: texto},
		Ejecutor:     ejecutorFalso{},
		Task:         parentTask,
	}

	res, err := a.Run(context.Background(), loop.Request{RoomPath: parentRoom.Path})
	if err != nil {
		t.Fatalf("Agent.Run: %v", err)
	}
	if res.Tokens.Entrada != 1 || res.Tokens.Salida != 1 {
		t.Errorf("tokens = %+v, quiero los de la subtarea", res.Tokens)
	}

	if _, err := os.Stat(filepath.Join(parentRoom.Path, "src", "done.txt")); err != nil {
		t.Fatalf("src/done.txt no se integró al cuarto padre: %v", err)
	}
}

func TestAgentRunRechazaDemasiadasSubtareas(t *testing.T) {
	root := repoConCommit(t)
	parentTask := tareaRecursivaDePrueba()
	parentTask.LimiteSubtareas = 1
	parentRoom, err := room.Create(context.Background(), root, parentTask.ID, "main")
	if err != nil {
		t.Fatalf("room.Create padre: %v", err)
	}
	t.Cleanup(func() { _ = room.Destroy(context.Background(), root, parentTask.ID) })

	texto := `[
		{"titulo": "uno", "listo_cuando": "test -f src/a.txt", "tocar_solo": ["src/**"]},
		{"titulo": "dos", "listo_cuando": "test -f src/b.txt", "tocar_solo": ["src/**"]}
	]`
	a := Agent{
		Planificador: generadorFalso{texto: texto},
		Ejecutor:     ejecutorFalso{},
		Task:         parentTask,
	}

	if _, err := a.Run(context.Background(), loop.Request{RoomPath: parentRoom.Path}); err == nil {
		t.Fatal("Agent.Run debió rechazar 2 subtareas con límite 1")
	}
}

func TestRestringirAlcanceRecortaFueraDePermitido(t *testing.T) {
	bs := []plan.Borrador{
		{Titulo: "a", TocarSolo: []string{"src/foo/**", "otro/lugar/**"}},
	}
	restringirAlcance(bs, []string{"src/**"})
	if len(bs[0].TocarSolo) != 1 || bs[0].TocarSolo[0] != "src/foo/**" {
		t.Errorf("TocarSolo = %v, quiero solo [src/foo/**]", bs[0].TocarSolo)
	}
}

func TestDentroDeAlcance(t *testing.T) {
	permitidos := []string{"src/**"}
	if !dentroDeAlcance("src/foo/**", permitidos) {
		t.Error("src/foo/** debió quedar dentro de src/**")
	}
	if dentroDeAlcance("otro/**", permitidos) {
		t.Error("otro/** no debió quedar dentro de src/**")
	}
}
