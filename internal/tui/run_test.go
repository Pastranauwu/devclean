package tui

import (
	"strings"
	"testing"
	"time"
)

func TestRenderFilaRun(t *testing.T) {
	f := FilaRun{ID: "T-001", Titulo: "exportar", Limite: 3}

	if s := renderFilaRun(f, nil, time.Time{}, 0); !strings.Contains(s, "pendiente") || !strings.Contains(s, "T-001") {
		t.Errorf("pendiente = %q", s)
	}
	if s := renderFilaRun(f, &tareaViva{estado: "lista", intentos: 2}, time.Time{}, 0); !strings.Contains(s, "verde en 2 intentos") {
		t.Errorf("lista = %q", s)
	}
	if s := renderFilaRun(f, &tareaViva{estado: "detenida"}, time.Time{}, 0); !strings.Contains(s, "detenida") {
		t.Errorf("detenida = %q", s)
	}
	if s := renderFilaRun(f, &tareaViva{estado: "trabajando"}, time.Now().Add(-time.Minute), 0); !strings.Contains(s, "1m0s") {
		t.Errorf("trabajando con reloj = %q", s)
	}
}

func TestReloj(t *testing.T) {
	if reloj(90*time.Second) != "1m30s" {
		t.Errorf("reloj(90s) = %q", reloj(90*time.Second))
	}
	if reloj(5*time.Second) != "5s" {
		t.Errorf("reloj(5s) = %q", reloj(5*time.Second))
	}
}
