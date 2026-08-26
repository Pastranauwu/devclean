package task

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNextID(t *testing.T) {
	dir := t.TempDir()

	id, err := NextID(dir)
	if err != nil {
		t.Fatalf("NextID: %v", err)
	}
	if id != "T-001" {
		t.Errorf("NextID vacío = %q, quiero T-001", id)
	}

	for _, name := range []string{"T-001.md", "T-002.md", "notas.md"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(""), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	id, err = NextID(dir)
	if err != nil {
		t.Fatalf("NextID: %v", err)
	}
	if id != "T-003" {
		t.Errorf("NextID = %q, quiero T-003", id)
	}

	// con un hueco intermedio el máximo manda; si se borra el último,
	// el correlativo retrocede (las tareas no se referencian fuera aún)
	if err := os.WriteFile(filepath.Join(dir, "T-005.md"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, "T-002.md")); err != nil {
		t.Fatal(err)
	}
	id, err = NextID(dir)
	if err != nil {
		t.Fatalf("NextID: %v", err)
	}
	if id != "T-006" {
		t.Errorf("NextID con hueco intermedio = %q, quiero T-006", id)
	}

	if err := os.Remove(filepath.Join(dir, "T-005.md")); err != nil {
		t.Fatal(err)
	}
	id, err = NextID(dir)
	if err != nil {
		t.Fatalf("NextID: %v", err)
	}
	if id != "T-002" {
		t.Errorf("NextID tras borrar el último = %q, quiero T-002", id)
	}
}

func TestSaveLoadRemove(t *testing.T) {
	dir := t.TempDir()
	task := Task{
		ID:             "T-001",
		Titulo:         "algo",
		ListoCuando:    "make test",
		TocarSolo:      []string{"src/**"},
		LimiteIntentos: 3,
		LimiteLineas:   200,
	}
	if err := Save(dir, task); err != nil {
		t.Fatalf("Save: %v", err)
	}
	loaded, err := Load(dir, "T-001")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Titulo != "algo" || loaded.ListoCuando != "make test" {
		t.Errorf("Load = %+v", loaded)
	}
	if err := Remove(dir, "T-001"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := Load(dir, "T-001"); err == nil {
		t.Error("Load debió fallar tras Remove")
	}
	if err := Remove(dir, "T-001"); err == nil {
		t.Error("Remove debió fallar dos veces")
	}
}

func TestList(t *testing.T) {
	dir := t.TempDir()
	for _, id := range []string{"T-003", "T-001", "T-002"} {
		task := Task{ID: id, Titulo: "x", ListoCuando: "true", LimiteIntentos: 3, LimiteLineas: 200}
		if err := Save(dir, task); err != nil {
			t.Fatal(err)
		}
	}
	tasks, err := List(dir)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(tasks) != 3 {
		t.Fatalf("List devolvió %d tareas, quiero 3", len(tasks))
	}
	for i, want := range []string{"T-001", "T-002", "T-003"} {
		if tasks[i].ID != want {
			t.Errorf("List[%d] = %s, quiero %s", i, tasks[i].ID, want)
		}
	}
}

func TestListConArchivoRoto(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "T-009.md"), []byte("basura\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := List(dir); err == nil {
		t.Fatal("List debió fallar con un archivo mal formado")
	}
}
