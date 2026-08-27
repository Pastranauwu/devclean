package tui

import (
	"strings"
	"testing"

	"github.com/Pastranauwu/devclean/internal/state"
)

func TestRenderBoard(t *testing.T) {
	filas := []Fila{
		{ID: "T-001", Titulo: "exportar", Estado: state.Lista},
		{ID: "T-002", Titulo: "login", Estado: state.EnCurso},
		{ID: "T-003", Titulo: "docs", Estado: state.Pendiente},
	}
	vista := renderBoard(filas, 80)
	for _, want := range []string{"LISTO PARA ENTREGAR", "T-001", "exportar", "EN CURSO", "T-002", "PENDIENTE", "T-003", "DETENIDO", "—"} {
		if !strings.Contains(vista, want) {
			t.Errorf("tablero sin %q:\n%s", want, vista)
		}
	}
}

func TestRenderBoardVacio(t *testing.T) {
	vista := renderBoard(nil, 80)
	if !strings.Contains(vista, "sin tareas") {
		t.Errorf("tablero vacío sin invitación a actuar:\n%s", vista)
	}
}
