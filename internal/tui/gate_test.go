package tui

import (
	"strings"
	"testing"

	"github.com/Pastranauwu/devclean/internal/ship"
)

func TestClasificar(t *testing.T) {
	pasos := []ship.Paso{
		{Nombre: "base", OK: true},
		{Nombre: "historial", OK: true},
		{Nombre: "ruido", OK: false},
	}
	e := clasificar(pasos, false)
	if e[0] != verde || e[1] != verde || e[2] != rojo {
		t.Errorf("estados = %v, quiero verde, verde, rojo", e)
	}
	if e[3] != trabajando {
		t.Errorf("el paso siguiente debió quedar trabajando, es %v", e[3])
	}
	if e[7] != pendiente {
		t.Errorf("el último paso debió quedar pendiente, es %v", e[7])
	}
}

func TestClasificarTerminado(t *testing.T) {
	e := clasificar([]ship.Paso{{Nombre: "base", OK: true}}, true)
	for i, s := range e {
		if s == trabajando {
			t.Errorf("terminado no debió dejar nada trabajando (paso %d)", i)
		}
	}
}

func TestRenderGate(t *testing.T) {
	pasos := []ship.Paso{
		{Nombre: "base", OK: true},
		{Nombre: "historial", OK: true, Detalle: "47 guardados → 1 commit"},
	}
	vista := renderGate("T-003", pasos, false, 80)

	for _, want := range []string{"ESCLUSA DE SALIDA · T-003", "base", "hist", "ruido", "secr", "presu", "bisec", "hand", "pr", "✓", "◐", "·"} {
		if !strings.Contains(vista, want) {
			t.Errorf("la vista no contiene %q:\n%s", want, vista)
		}
	}
	if !strings.Contains(vista, "47 guardados → 1 commit") {
		t.Errorf("falta el detalle del último paso:\n%s", vista)
	}
}

func TestRenderGateFreno(t *testing.T) {
	pasos := []ship.Paso{{Nombre: "base", OK: false, Detalle: "rebase en conflicto"}}
	vista := renderGate("T-003", pasos, true, 80)
	if !strings.Contains(vista, "✗") {
		t.Errorf("un paso fallado debió pintarse con ✗:\n%s", vista)
	}
	if !strings.Contains(vista, "rebase en conflicto") {
		t.Errorf("falta el motivo del freno:\n%s", vista)
	}
}
