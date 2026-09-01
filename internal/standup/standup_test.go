package standup

import (
	"strings"
	"testing"
	"time"

	"github.com/Pastranauwu/devclean/internal/loop"
	"github.com/Pastranauwu/devclean/internal/state"
	"github.com/Pastranauwu/devclean/internal/task"
)

func activa(id string) ([]task.Task, map[string]state.State) {
	return []task.Task{{ID: id, Titulo: "t"}},
		map[string]state.State{id: {ID: id, Estado: state.EnCurso}}
}

// El caso que motivó el latido: una invocación lleva 40 minutos sin
// volver. attempts.jsonl está vacío, así que el parte informaba "dentro
// de contrato" de una tarea colgada.
func TestAnalizarDetectaFaseColgada(t *testing.T) {
	tareas, estados := activa("T-003")
	latidos := map[string]loop.Latido{
		"T-003": {
			ID: "T-003", Intento: 1, Limite: 3, Fase: loop.FaseAgente,
			Modelo: "opencode/big-pickle", DesdeFase: time.Now().Add(-40 * time.Minute),
		},
	}

	eventos := Analizar(tareas, estados, nil, latidos)
	if len(eventos) != 1 {
		t.Fatalf("eventos = %+v, quiero 1", eventos)
	}
	if eventos[0].Tipo != EventoAtasco {
		t.Fatalf("tipo = %v, quiero EventoAtasco", eventos[0].Tipo)
	}
	for _, quiero := range []string{"T-003", "40m", loop.FaseAgente} {
		if !strings.Contains(eventos[0].Detalle, quiero) {
			t.Errorf("detalle %q no menciona %q", eventos[0].Detalle, quiero)
		}
	}
}

// Una fase recién arrancada no es un atasco.
func TestAnalizarNoAlarmaFaseJoven(t *testing.T) {
	tareas, estados := activa("T-001")
	latidos := map[string]loop.Latido{
		"T-001": {ID: "T-001", Intento: 1, Fase: loop.FaseAgente, DesdeFase: time.Now().Add(-1 * time.Minute)},
	}

	eventos := Analizar(tareas, estados, nil, latidos)
	if len(eventos) != 1 || eventos[0].Tipo != EventoOK {
		t.Errorf("eventos = %+v, quiero un solo EventoOK", eventos)
	}
}

// Sin latidos el parte sigue funcionando como antes: los comandos que no
// los cargan pasan nil.
func TestAnalizarSinLatidos(t *testing.T) {
	tareas, estados := activa("T-001")
	eventos := Analizar(tareas, estados, nil, nil)
	if len(eventos) != 1 || eventos[0].Tipo != EventoOK {
		t.Errorf("eventos = %+v", eventos)
	}
}
