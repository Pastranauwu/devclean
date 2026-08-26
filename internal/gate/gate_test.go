package gate

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Pastranauwu/devclean/internal/config"
	"github.com/Pastranauwu/devclean/internal/task"
)

func tareaValida() task.Task {
	return task.Task{
		ID:             "T-001",
		Titulo:         "x",
		ListoCuando:    "false",
		TocarSolo:      []string{"src/export/**"},
		LimiteIntentos: 3,
		LimiteLineas:   200,
	}
}

func TestGateTodoVerde(t *testing.T) {
	res := Run(context.Background(), t.TempDir(), config.Config{}, tareaValida(), nil, DefaultTimeout)
	if !res.Aprobada {
		t.Fatalf("tarea válida rechazada: %+v", res.Chequeos)
	}
	if len(res.Chequeos) != 4 {
		t.Fatalf("Chequeos = %d, quiero 4", len(res.Chequeos))
	}
}

func TestGateSinListoCuando(t *testing.T) {
	tarea := tareaValida()
	tarea.ListoCuando = ""
	res := Run(context.Background(), t.TempDir(), config.Config{}, tarea, nil, DefaultTimeout)
	if res.Aprobada {
		t.Fatal("tarea sin listo_cuando aprobada")
	}
	if res.Chequeos[1].Nombre != "falla hoy" || res.Chequeos[1].OK {
		t.Errorf("falla hoy debió quedar no evaluado: %+v", res.Chequeos[1])
	}
}

func TestGateComandoInexistente(t *testing.T) {
	tarea := tareaValida()
	tarea.ListoCuando = "binario-que-no-existe-xyz --flag"
	res := Run(context.Background(), t.TempDir(), config.Config{}, tarea, nil, DefaultTimeout)
	if res.Aprobada {
		t.Fatal("tarea con comando inexistente aprobada")
	}
	if got := res.PrimerMotivo(); got == "" {
		t.Error("PrimerMotivo vacío")
	}
}

func TestGateScriptRelativo(t *testing.T) {
	root := t.TempDir()
	script := filepath.Join(root, "test.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	tarea := tareaValida()
	tarea.ListoCuando = "./test.sh"
	res := Run(context.Background(), root, config.Config{}, tarea, nil, DefaultTimeout)
	if !res.Aprobada {
		t.Fatalf("script relativo ejecutable rechazado: %+v", res.Chequeos)
	}

	tarea.ListoCuando = "./no-existe.sh"
	res = Run(context.Background(), root, config.Config{}, tarea, nil, DefaultTimeout)
	if res.Aprobada {
		t.Fatal("script inexistente aprobado")
	}
}

func TestGateYaPasa(t *testing.T) {
	tarea := tareaValida()
	tarea.ListoCuando = "true"
	res := Run(context.Background(), t.TempDir(), config.Config{}, tarea, nil, DefaultTimeout)
	if res.Aprobada {
		t.Fatal("tarea cuyo listo_cuando ya pasa fue aprobada")
	}
	fallaHoy := res.Chequeos[1]
	if fallaHoy.OK || fallaHoy.Motivo == "" {
		t.Errorf("falla hoy debió rechazar: %+v", fallaHoy)
	}
}

func TestGateTimeout(t *testing.T) {
	tarea := tareaValida()
	tarea.ListoCuando = "sleep 5"
	res := Run(context.Background(), t.TempDir(), config.Config{}, tarea, nil, 100*time.Millisecond)
	if res.Aprobada {
		t.Fatal("tarea con timeout aprobada")
	}
	fallaHoy := res.Chequeos[1]
	if fallaHoy.OK {
		t.Error("timeout debió rechazar la tarea")
	}
}

func TestGateCruce(t *testing.T) {
	otra := task.Task{ID: "T-002", TocarSolo: []string{"src/**"}}
	res := Run(context.Background(), t.TempDir(), config.Config{}, tareaValida(), []task.Task{otra}, DefaultTimeout)
	if res.Aprobada {
		t.Fatal("tareas cruzadas aprobadas")
	}

	lejana := task.Task{ID: "T-003", TocarSolo: []string{"docs/**"}}
	res = Run(context.Background(), t.TempDir(), config.Config{}, tareaValida(), []task.Task{lejana}, DefaultTimeout)
	if !res.Aprobada {
		t.Fatalf("tareas sin cruce rechazadas: %+v", res.Chequeos)
	}
}

func TestGateZonasProhibidas(t *testing.T) {
	cfg := config.Config{ZonasProhibidas: config.DefaultForbiddenZones()}

	tarea := tareaValida()
	tarea.TocarSolo = []string{"package-lock.json"}
	res := Run(context.Background(), t.TempDir(), cfg, tarea, nil, DefaultTimeout)
	if res.Aprobada {
		t.Fatal("tarea tocando lockfile aprobada")
	}

	tarea.TocarSolo = []string{"migrations/v2/**"}
	res = Run(context.Background(), t.TempDir(), cfg, tarea, nil, DefaultTimeout)
	if res.Aprobada {
		t.Fatal("tarea tocando migrations aprobada")
	}

	tarea.TocarSolo = []string{"src/export/**"}
	res = Run(context.Background(), t.TempDir(), cfg, tarea, nil, DefaultTimeout)
	if !res.Aprobada {
		t.Fatalf("tarea fuera de zonas prohibidas rechazada: %+v", res.Chequeos)
	}
}

func TestGlobsOverlap(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"src/**", "src/**", true},
		{"src/**", "src/export/**", true},
		{"src/export/**", "src/**", true},
		{"src/auth/**", "src/export/**", false},
		{"package-lock.json", "package-lock.json", true},
		{"CHANGELOG*", "CHANGELOG.md", true},
		{"migrations/**", "migrations/v2/001.sql", true},
		{"src/*", "src/x", true},
		{"docs/**", "src/**", false},
		{"a.go", "b.go", false},
	}
	for _, tc := range cases {
		if got := globsOverlap(tc.a, tc.b); got != tc.want {
			t.Errorf("globsOverlap(%q, %q) = %v, quiero %v", tc.a, tc.b, got, tc.want)
		}
	}
}
