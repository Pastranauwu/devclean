package revisor

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Pastranauwu/devclean/internal/task"
)

type genFijo struct {
	texto string
	err   error
}

func (g genFijo) Generar(context.Context, string) (string, error) { return g.texto, g.err }

func tareas() []task.Task {
	return []task.Task{{ID: "T-001", Titulo: "validar mac", ListoCuando: "go test ./...", Expone: []string{"wol.Parse(s string) error"}}}
}

func TestRevisarApruebaYVeta(t *testing.T) {
	v, err := Revisar(context.Background(), Opciones{
		Generador: genFijo{texto: "```json\n{\"aprobado\": true, \"motivo\": \"hace lo que pide\"}\n```"},
		Tareas:    tareas(), Diff: "+ algo",
	})
	if err != nil || !v.Aprobado {
		t.Fatalf("v = %+v, err = %v", v, err)
	}

	v, err = Revisar(context.Background(), Opciones{
		Generador: genFijo{texto: `{"aprobado": false, "motivo": "no valida la entrada", "hallazgos": ["wol.go:12 — acepta mac vacía"]}`},
		Tareas:    tareas(), Diff: "+ algo",
	})
	if err != nil {
		t.Fatal(err)
	}
	if v.Aprobado || !strings.Contains(v.Resumen(), "wol.go:12") {
		t.Errorf("v = %+v · el veto debe traer sus hallazgos", v)
	}
}

// Falla cerrado: lo que no se puede revisar no se integra.
func TestRevisarFallaCerrado(t *testing.T) {
	casos := []struct {
		nombre string
		o      Opciones
	}{
		{"sin generador", Opciones{Tareas: tareas(), Diff: "+x"}},
		{"sin diff", Opciones{Generador: genFijo{texto: "{}"}, Tareas: tareas()}},
		{"el modelo falla", Opciones{Generador: genFijo{err: errors.New("503")}, Tareas: tareas(), Diff: "+x"}},
		{"respuesta ilegible", Opciones{Generador: genFijo{texto: "pues no sé"}, Tareas: tareas(), Diff: "+x"}},
		{"json inválido", Opciones{Generador: genFijo{texto: "{aprobado: si}"}, Tareas: tareas(), Diff: "+x"}},
		{"veta sin motivo", Opciones{Generador: genFijo{texto: `{"aprobado": false}`}, Tareas: tareas(), Diff: "+x"}},
		{"diff enorme", Opciones{Generador: genFijo{texto: `{"aprobado": true}`}, Tareas: tareas(), Diff: strings.Repeat("x", MaxDiff+1)}},
	}
	for _, c := range casos {
		if v, err := Revisar(context.Background(), c.o); err == nil || v.Aprobado {
			t.Errorf("%s: v = %+v, err = %v · debe fallar sin aprobar", c.nombre, v, err)
		}
	}
}

// El prompt tiene que decirle qué juzga y qué no: si veta por estilo,
// ninguna entrega se integra nunca.
func TestPromptAcotaElVeto(t *testing.T) {
	p := Prompt(tareas(), "+ codigo")
	for _, quiero := range []string{"T-001", "validar mac", "wol.Parse", "+ codigo", "NO vetes por estilo", "aprobado"} {
		if !strings.Contains(p, quiero) {
			t.Errorf("el prompt no menciona %q", quiero)
		}
	}
}
