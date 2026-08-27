package main

import (
	"testing"

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
