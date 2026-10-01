package tui

import (
	"fmt"
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
	if s := renderFilaRun(f, &tareaViva{estado: "lista", intentos: 1}, time.Time{}, 0); !strings.Contains(s, "verde en 1 intento") || strings.Contains(s, "1 intentos") {
		t.Errorf("un solo intento debe decir \"1 intento\", no \"1 intentos\": %q", s)
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

func TestCorridaQueNoCabeMuestraLasQueTrabajan(t *testing.T) {
	m := runModel{estado: map[string]*tareaViva{}, inicio: map[string]time.Time{}, alto: 24}
	for i := 1; i <= 40; i++ {
		id := fmt.Sprintf("T-%03d", i)
		m.filas = append(m.filas, FilaRun{ID: id, Titulo: "tarea"})
		if i <= 10 {
			m.estado[id] = &tareaViva{estado: "lista", intentos: 1}
		}
	}
	m.estado["T-011"] = &tareaViva{estado: "trabajando"}
	v := m.View()
	if n := strings.Count(v, "\n") + 1; n > 24 {
		t.Errorf("la vista tiene %d líneas en una terminal de 24", n)
	}
	if !strings.Contains(v, "T-011") || !strings.Contains(v, "10 verdes") || !strings.Contains(v, "29 pendientes") || strings.Contains(v, "T-040") {
		t.Errorf("vista compacta:\n%s", v)
	}
}
