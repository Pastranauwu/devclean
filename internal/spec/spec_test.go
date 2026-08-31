package spec

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Pastranauwu/devclean/internal/task"
)

const specEjemplo = `version: 1
feature: "Autenticación JWT"
agente: backend

reglas:
  - "no usar sesiones en memoria"
  - "tokens con rotación"

tasks:
  - id: T-001
    titulo: modelo de usuario y hash de contraseñas
    porque: seguridad básica
    listo_cuando: go test ./internal/auth/... -run TestHash
    tocar_solo: ["internal/auth/**"]
    no_tocar: ["migrations/**"]
    agente: backend
    peso: liviana
    limite_intentos: 3
    limite_lineas: 200

  - id: T-002
    titulo: endpoint de login
    listo_cuando: go test ./internal/api/... -run TestLogin
    tocar_solo: ["internal/api/**"]
    depende_de: ["T-001"]
    expone: ["POST /api/login -> 200 {token}"]
    peso: media
`

func TestParseSpecCompleto(t *testing.T) {
	s, err := Parse([]byte(specEjemplo))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if s.Version != 1 {
		t.Errorf("Version = %d, quiero 1", s.Version)
	}
	if s.Feature != "Autenticación JWT" {
		t.Errorf("Feature = %q, quiero Autenticación JWT", s.Feature)
	}
	if s.Agente != "backend" {
		t.Errorf("Agente = %q, quiero backend", s.Agente)
	}
	if len(s.Reglas) != 2 || s.Reglas[0] != "no usar sesiones en memoria" {
		t.Errorf("Reglas = %v", s.Reglas)
	}
	if len(s.Tasks) != 2 {
		t.Fatalf("len(Tasks) = %d, quiero 2", len(s.Tasks))
	}
	t1 := s.Tasks[0]
	if t1.ID != "T-001" || t1.Titulo != "modelo de usuario y hash de contraseñas" || t1.Agente != "backend" {
		t.Errorf("T1 = %+v", t1)
	}
	t2 := s.Tasks[1]
	// t2 no definió agente, debe heredar s.Agente ("backend")
	if t2.ID != "T-002" || t2.Agente != "backend" || len(t2.DependeDe) != 1 || t2.DependeDe[0] != "T-001" {
		t.Errorf("T2 = %+v", t2)
	}
}

func TestParseSpecInlineMaps(t *testing.T) {
	raw := `version: 1
feature: "Features inline"
tasks:
  - { id: T-001, titulo: "primera", listo_cuando: "make test", tocar_solo: ["src/**"], agente: "architect" }
  - { id: T-002, titulo: "segunda", listo_cuando: "make test2", depende_de: ["T-001"] }
`
	s, err := Parse([]byte(raw))
	if err != nil {
		t.Fatalf("Parse inline: %v", err)
	}
	if len(s.Tasks) != 2 {
		t.Fatalf("len(Tasks) = %d, quiero 2", len(s.Tasks))
	}
	if s.Tasks[0].Agente != "architect" {
		t.Errorf("t0 agente = %q, quiero architect", s.Tasks[0].Agente)
	}
}

func TestParseSpecReglasInline(t *testing.T) {
	raw := `version: 1
feature: "Reglas inline"
reglas: ["regla 1", "regla 2"]
tasks:
  - id: T-001
    titulo: algo
    listo_cuando: true
`
	s, err := Parse([]byte(raw))
	if err != nil {
		t.Fatalf("Parse reglas inline: %v", err)
	}
	if len(s.Reglas) != 2 || s.Reglas[1] != "regla 2" {
		t.Errorf("Reglas = %v", s.Reglas)
	}
}

func TestAssignCorrelativeIDs(t *testing.T) {
	dir := t.TempDir()
	tasks := []task.Task{
		{Titulo: "tarea a", ListoCuando: "true"},
		{ID: "T-005", Titulo: "tarea existente", ListoCuando: "true"},
		{Titulo: "tarea b", ListoCuando: "true"},
	}

	withIDs, err := AssignCorrelativeIDs(dir, tasks)
	if err != nil {
		t.Fatalf("AssignCorrelativeIDs: %v", err)
	}
	if withIDs[0].ID != "T-001" {
		t.Errorf("t0 ID = %q, quiero T-001", withIDs[0].ID)
	}
	if withIDs[1].ID != "T-005" {
		t.Errorf("t1 ID = %q, quiero T-005", withIDs[1].ID)
	}
	if withIDs[2].ID != "T-002" {
		t.Errorf("t2 ID = %q, quiero T-002", withIDs[2].ID)
	}
}

func TestApplyYDryRun(t *testing.T) {
	dir := t.TempDir()
	tasksDir := filepath.Join(dir, ".devclean", "tasks")

	s := Spec{
		Version: 1,
		Feature: "Test Feature",
		Tasks: []task.Task{
			{Titulo: "t1", ListoCuando: "go test ./...", LimiteIntentos: 3, LimiteLineas: 200},
			{Titulo: "t2", ListoCuando: "go test ./...", LimiteIntentos: 3, LimiteLineas: 200},
		},
	}

	// 1. Dry run no debe escribir archivos en disco
	applied, err := Apply(tasksDir, s, true)
	if err != nil {
		t.Fatalf("Apply dry run: %v", err)
	}
	if len(applied) != 2 || applied[0].ID != "T-001" {
		t.Errorf("applied = %+v", applied)
	}
	if _, err := os.Stat(tasksDir); !os.IsNotExist(err) {
		t.Errorf("dry-run creó el directorio %s", tasksDir)
	}

	// 2. Apply real guarda los archivos
	appliedReal, err := Apply(tasksDir, s, false)
	if err != nil {
		t.Fatalf("Apply real: %v", err)
	}
	if len(appliedReal) != 2 {
		t.Fatalf("len(appliedReal) = %d", len(appliedReal))
	}

	loaded, err := task.Load(tasksDir, "T-001")
	if err != nil {
		t.Fatalf("task.Load T-001: %v", err)
	}
	if loaded.Titulo != "t1" {
		t.Errorf("loaded.Titulo = %q, quiero t1", loaded.Titulo)
	}
}

func TestApplyRechazaContratoInvalido(t *testing.T) {
	dir := t.TempDir()
	s := Spec{
		Version: 1,
		Tasks: []task.Task{
			{Titulo: "", ListoCuando: ""}, // Faltan título y listo_cuando
		},
	}
	if _, err := Apply(dir, s, false); err == nil {
		t.Fatal("Apply debió fallar por contrato inválido")
	}
}

func TestFindSpec(t *testing.T) {
	root := t.TempDir()
	if _, err := Find(root); err == nil {
		t.Fatal("Find debió fallar sin archivos")
	}

	specPath := filepath.Join(root, "devclean.spec.yml")
	if err := os.WriteFile(specPath, []byte("version: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := Find(root)
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if got != specPath {
		t.Errorf("Find = %q, quiero %q", got, specPath)
	}
}

func TestMarshalRoundtrip(t *testing.T) {
	original, err := Parse([]byte(specEjemplo))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	marshaled := Marshal(original)
	reparsed, err := Parse(marshaled)
	if err != nil {
		t.Fatalf("reparse: %v\nYAML:\n%s", err, marshaled)
	}
	if !reflect.DeepEqual(original, reparsed) {
		t.Errorf("Roundtrip mismatch:\nOriginal: %+v\nReparsed: %+v", original, reparsed)
	}
}
