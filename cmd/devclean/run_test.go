package main

import (
	"testing"

	"github.com/Pastranauwu/devclean/internal/config"
	"github.com/Pastranauwu/devclean/internal/task"
)

func tarea(id string, alcance []string) task.Task {
	return task.Task{ID: id, Titulo: id, TocarSolo: alcance}
}

func TestAsignarUnaSola(t *testing.T) {
	aceptadas, rechazos := asignar([]task.Task{tarea("T-001", nil)})
	if len(aceptadas) != 1 || len(rechazos) != 0 {
		t.Fatalf("aceptadas=%v rechazos=%v, quiero una sola aceptada", aceptadas, rechazos)
	}
}

func TestAsignarVacioConMasDeUna(t *testing.T) {
	aceptadas, rechazos := asignar([]task.Task{
		tarea("T-001", []string{"src/export/**"}),
		tarea("T-002", nil),
	})
	if len(aceptadas) != 1 || aceptadas[0].ID != "T-001" {
		t.Fatalf("aceptadas=%v, quiero solo T-001", aceptadas)
	}
	if len(rechazos) != 1 || rechazos[0].ID != "T-002" {
		t.Fatalf("rechazos=%v, quiero T-002 por vacío", rechazos)
	}
}

func TestAsignarCruce(t *testing.T) {
	aceptadas, rechazos := asignar([]task.Task{
		tarea("T-001", []string{"src/**"}),
		tarea("T-002", []string{"src/export/**"}),
		tarea("T-003", []string{"docs/**"}),
	})
	ids := map[string]bool{}
	for _, a := range aceptadas {
		ids[a.ID] = true
	}
	if len(aceptadas) != 2 || !ids["T-001"] || !ids["T-003"] {
		t.Fatalf("aceptadas=%v, quiero T-001 y T-003", aceptadas)
	}
	if len(rechazos) != 1 || rechazos[0].ID != "T-002" {
		t.Fatalf("rechazos=%v, quiero T-002 por cruce", rechazos)
	}
}

func TestDepsVerdes(t *testing.T) {
	verde := map[string]bool{"T-001": true}
	if !depsVerdes(nil, verde) {
		t.Error("sin dependencias debió ser verde")
	}
	if !depsVerdes([]string{"T-001"}, verde) {
		t.Error("dependencia ya verde debió pasar")
	}
	if depsVerdes([]string{"T-002"}, verde) {
		t.Error("dependencia pendiente no debió pasar")
	}
	if got := depsFaltantes([]string{"T-001", "T-003"}, verde); len(got) != 1 || got[0] != "T-003" {
		t.Errorf("depsFaltantes = %v, quiero [T-003]", got)
	}
}

func TestModeloParaTarea(t *testing.T) {
	cfg := config.Config{
		Estrategia: "equilibrada",
		Modelos:    map[string]string{"liviana": "glm-4", "media": "glm-5.2", "pesada": "claude-sonnet"},
		Proveedores: map[string]config.Proveedor{
			"ejecutor": {Modelo: "default-ejecutor"},
		},
	}

	// el flag gana sobre todo
	if got := modeloParaTarea(cfg, "flag-model", task.Task{Peso: "pesada"}); got != "flag-model" {
		t.Errorf("flag = %q, quiero flag-model", got)
	}
	// peso explícito
	if got := modeloParaTarea(cfg, "", task.Task{Peso: "pesada"}); got != "claude-sonnet" {
		t.Errorf("peso pesada = %q, quiero claude-sonnet", got)
	}
	// sin peso, usa la estrategia
	if got := modeloParaTarea(cfg, "", task.Task{}); got != "glm-5.2" {
		t.Errorf("estrategia equilibrada = %q, quiero glm-5.2", got)
	}
	// peso sin modelo mapeado cae al ejecutor
	cfg2 := config.Config{Modelos: map[string]string{"media": "glm-5.2"}, Proveedores: map[string]config.Proveedor{"ejecutor": {Modelo: "default"}}}
	if got := modeloParaTarea(cfg2, "", task.Task{Peso: "pesada"}); got != "default" {
		t.Errorf("peso sin mapeo = %q, quiero default", got)
	}
}
