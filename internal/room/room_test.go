package room

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func gitCmd(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func repoConCommit(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	gitCmd(t, root, "init", "-b", "main")
	gitCmd(t, root, "-c", "user.email=t@t", "-c", "user.name=t", "commit", "--allow-empty", "-m", "init")
	return root
}

func TestCreateYDestroy(t *testing.T) {
	root := repoConCommit(t)
	r, err := Create(context.Background(), root, "T-001", "main")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if r.Rama != "devclean/T-001" {
		t.Errorf("Rama = %q, quiero devclean/T-001", r.Rama)
	}
	if r.Puerto <= 0 {
		t.Errorf("Puerto = %d, quiero > 0", r.Puerto)
	}
	if _, err := os.Stat(r.Path); err != nil {
		t.Errorf("cuarto no existe en disco: %v", err)
	}
	// el worktree está en su propia rama
	out, _ := gitOut(t, r.Path, "branch", "--show-current")
	if out != "devclean/T-001" {
		t.Errorf("rama del cuarto = %q", out)
	}

	if err := Destroy(context.Background(), root, "T-001"); err != nil {
		t.Fatalf("Destroy: %v", err)
	}
	if _, err := os.Stat(r.Path); !os.IsNotExist(err) {
		t.Errorf("el cuarto sigue en disco tras Destroy")
	}
	out, _ = gitOut(t, root, "branch", "--list", "devclean/T-001")
	if out != "" {
		t.Errorf("la rama sigue existiendo tras Destroy")
	}
	// Destroy es idempotente
	if err := Destroy(context.Background(), root, "T-001"); err != nil {
		t.Errorf("Destroy dos veces: %v", err)
	}
}

func TestCreateDosVecesFalla(t *testing.T) {
	root := repoConCommit(t)
	if _, err := Create(context.Background(), root, "T-001", "main"); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := Create(context.Background(), root, "T-001", "main"); err == nil {
		t.Fatal("Create dos veces debió fallar")
	}
	if err := Destroy(context.Background(), root, "T-001"); err != nil {
		t.Fatal(err)
	}
}

func TestCreateSinCommits(t *testing.T) {
	root := t.TempDir()
	gitCmd(t, root, "init", "-b", "main")
	_, err := Create(context.Background(), root, "T-001", "main")
	if err == nil {
		t.Fatal("Create sin commits debió fallar")
	}
	if !strings.Contains(err.Error(), "commits") {
		t.Errorf("error sin pista clara: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(Dir(root), "T-001")); !os.IsNotExist(statErr) {
		t.Error("quedó un cuarto a medias")
	}
}

func TestCreateInstalaDepsGo(t *testing.T) {
	root := repoConCommit(t)
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module x\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, root, "add", "go.mod")
	gitCmd(t, root, "-c", "user.email=t@t", "-c", "user.name=t", "commit", "-m", "go.mod")

	r, err := Create(context.Background(), root, "T-001", "main")
	if err != nil {
		t.Fatalf("Create con go.mod: %v", err)
	}
	if _, err := os.Stat(filepath.Join(r.Path, "go.mod")); err != nil {
		t.Error("el cuarto no tiene go.mod")
	}
	if err := Destroy(context.Background(), root, "T-001"); err != nil {
		t.Fatal(err)
	}
}

func TestIntegrationEncadenaOleadas(t *testing.T) {
	root := repoConCommit(t)
	ctx := context.Background()

	if err := ResetIntegration(ctx, root, "main"); err != nil {
		t.Fatalf("ResetIntegration: %v", err)
	}

	// oleada 1: una tarea crea un archivo en su rama
	r1, err := Create(ctx, root, "T-001", IntegrationBranch)
	if err != nil {
		t.Fatalf("Create T-001: %v", err)
	}
	if err := os.WriteFile(filepath.Join(r1.Path, "base.txt"), []byte("hola\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, r1.Path, "add", "-A")
	gitCmd(t, r1.Path, "-c", "user.email=t@t", "-c", "user.name=t", "commit", "-m", "wip: T-001")

	if salida, err := Integrar(ctx, root, "T-001"); err != nil {
		t.Fatalf("Integrar T-001: %v · %s", err, salida)
	}

	// oleada 2: la siguiente tarea parte de la integración y ve el archivo
	r2, err := Create(ctx, root, "T-002", IntegrationBranch)
	if err != nil {
		t.Fatalf("Create T-002: %v", err)
	}
	if _, err := os.Stat(filepath.Join(r2.Path, "base.txt")); err != nil {
		t.Errorf("la oleada 2 no vio el trabajo de la 1: %v", err)
	}

	for _, id := range []string{"T-001", "T-002"} {
		if err := Destroy(ctx, root, id); err != nil {
			t.Fatal(err)
		}
	}
	if err := Destroy(ctx, root, "_integra"); err != nil {
		t.Fatal(err)
	}
}

func TestIntegrarConflictoAborta(t *testing.T) {
	root := repoConCommit(t)
	ctx := context.Background()
	if err := ResetIntegration(ctx, root, "main"); err != nil {
		t.Fatalf("ResetIntegration: %v", err)
	}
	// dos ramas que tocan el mismo archivo de forma distinta
	for _, id := range []string{"T-001", "T-002"} {
		r, err := Create(ctx, root, id, IntegrationBranch)
		if err != nil {
			t.Fatalf("Create %s: %v", id, err)
		}
		if err := os.WriteFile(filepath.Join(r.Path, "x.txt"), []byte(id+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		gitCmd(t, r.Path, "add", "-A")
		gitCmd(t, r.Path, "-c", "user.email=t@t", "-c", "user.name=t", "commit", "-m", "wip: "+id)
	}
	if salida, err := Integrar(ctx, root, "T-001"); err != nil {
		t.Fatalf("Integrar T-001: %v · %s", err, salida)
	}
	if _, err := Integrar(ctx, root, "T-002"); err == nil {
		t.Fatal("Integrar T-002 con conflicto debió fallar")
	}
	for _, id := range []string{"T-001", "T-002", "_integra"} {
		if err := Destroy(ctx, root, id); err != nil {
			t.Fatal(err)
		}
	}
}

func gitOut(t *testing.T, dir string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}
