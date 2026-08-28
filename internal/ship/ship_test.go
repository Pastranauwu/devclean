package ship

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Pastranauwu/devclean/internal/config"
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

func escribir(t *testing.T, root, rel, contenido string) {
	t.Helper()
	abs := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte(contenido), 0o644); err != nil {
		t.Fatal(err)
	}
}

func taskTitulo(titulo string) task.Task {
	return task.Task{
		Version:        task.Version,
		ID:             "T-001",
		Titulo:         titulo,
		ListoCuando:    "true",
		LimiteIntentos: 3,
		LimiteLineas:   200,
	}
}

func cuartoConWip(t *testing.T, root string) room.Room {
	t.Helper()
	r, err := room.Create(context.Background(), root, "T-001", "main")
	if err != nil {
		t.Fatalf("room.Create: %v", err)
	}
	t.Cleanup(func() { _ = room.Destroy(context.Background(), root, "T-001") })
	escribir(t, r.Path, "a.go", "package a\n")
	gitCmd(t, r.Path, "add", "-A")
	gitCmd(t, r.Path, "-c", "user.email=d@d", "-c", "user.name=d", "commit", "-m", "wip: T-001 intento 1")
	return r
}

func TestAplanar(t *testing.T) {
	root := repoConCommit(t)
	r := cuartoConWip(t, root)

	cuenta, hash, err := aplanar(context.Background(), r.Path, "main", "exportar a CSV", "feat", "glm-5.2")
	if err != nil {
		t.Fatalf("aplanar: %v", err)
	}
	if cuenta != 1 {
		t.Errorf("cuenta = %d, quiero 1", cuenta)
	}
	if hash == "" {
		t.Error("hash vacío")
	}
	msg := gitCmd(t, r.Path, "log", "-1", "--format=%B")
	if !strings.Contains(msg, "feat: exportar a CSV") {
		t.Errorf("mensaje = %q", msg)
	}
	if !strings.Contains(msg, "Agent: glm-5.2") {
		t.Errorf("sin trailer Agent: %q", msg)
	}
	if n := strings.TrimSpace(gitCmd(t, r.Path, "rev-list", "--count", "main..HEAD")); n != "1" {
		t.Errorf("commits tras aplanar = %s, quiero 1", n)
	}
}

func TestRebaseSinRemoto(t *testing.T) {
	root := repoConCommit(t)
	r := cuartoConWip(t, root)

	target, conflictos, err := rebase(context.Background(), root, r.Path, "main", r.Rama)
	if err != nil {
		t.Fatalf("rebase: %v", err)
	}
	if target != "main" {
		t.Errorf("target = %q, quiero main", target)
	}
	if len(conflictos) != 0 {
		t.Errorf("conflictos = %v", conflictos)
	}
}

func TestRebaseConflicto(t *testing.T) {
	root := repoConCommit(t)
	escribir(t, root, "f.txt", "base\n")
	gitCmd(t, root, "add", "-A")
	gitCmd(t, root, "-c", "user.email=d@d", "-c", "user.name=d", "commit", "-m", "f.txt base")

	r, err := room.Create(context.Background(), root, "T-001", "main")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = room.Destroy(context.Background(), root, "T-001") })

	// main avanza y el cuarto toca el mismo archivo
	escribir(t, root, "f.txt", "cambiado en main\n")
	gitCmd(t, root, "add", "-A")
	gitCmd(t, root, "-c", "user.email=d@d", "-c", "user.name=d", "commit", "-m", "avanza main")

	escribir(t, r.Path, "f.txt", "cambiado en el cuarto\n")
	gitCmd(t, r.Path, "add", "-A")
	gitCmd(t, r.Path, "-c", "user.email=d@d", "-c", "user.name=d", "commit", "-m", "wip")

	_, conflictos, err := rebase(context.Background(), root, r.Path, "main", r.Rama)
	if err == nil {
		t.Fatal("rebase debió conflictuar")
	}
	if len(conflictos) == 0 || conflictos[0] != "f.txt" {
		t.Errorf("conflictos = %v, quiero [f.txt]", conflictos)
	}
}

func TestRunDryRun(t *testing.T) {
	root := repoConCommit(t)
	r := cuartoConWip(t, root)
	cfg := config.Config{Base: "main", Pruebas: "true", PatronesPrueba: config.DefaultTestPatterns()}
	tk := taskTitulo("exportar a CSV")
	tk.TocarSolo = []string{"a.go"}

	res := Run(context.Background(), Opciones{
		Root: root, Room: r, Task: tk, Config: cfg, Modelo: "glm-5.2", Base: "main", DryRun: true,
	})
	if !res.Aprobado {
		t.Fatalf("dry-run no aprobado: %+v", res.Pasos)
	}
	if len(res.Pasos) != 9 {
		t.Fatalf("pasos = %d, quiero 9", len(res.Pasos))
	}
	nombres := []string{"base", "historial", "ruido", "secretos", "presupuesto", "interfaces", "bisectable", "handoff", "pr"}
	for i, n := range nombres {
		if res.Pasos[i].Nombre != n {
			t.Errorf("paso %d = %q, quiero %q", i, res.Pasos[i].Nombre, n)
		}
	}
}

func TestRunSeFrenaEnRuido(t *testing.T) {
	root := repoConCommit(t)
	r := cuartoConWip(t, root)
	// añadir un print de debug al trabajo
	escribir(t, r.Path, "b.go", "package b\nfunc f() {\n\tfmt.Println(\"hola\")\n}\n")
	gitCmd(t, r.Path, "add", "-A")
	gitCmd(t, r.Path, "-c", "user.email=d@d", "-c", "user.name=d", "commit", "-m", "wip: T-001 intento 2")

	cfg := config.Config{Base: "main", Pruebas: "true", PatronesPrueba: config.DefaultTestPatterns()}
	tk := taskTitulo("exportar a CSV")

	res := Run(context.Background(), Opciones{Root: root, Room: r, Task: tk, Config: cfg, Modelo: "glm-5.2", Base: "main", DryRun: true})
	if res.Aprobado {
		t.Fatal("una tarea con print de debug debió frenarse en ruido")
	}
	ultimo := res.Pasos[len(res.Pasos)-1]
	if ultimo.Nombre != "ruido" || ultimo.OK {
		t.Errorf("último paso = %+v, quiero frenarse en ruido", ultimo)
	}
}
