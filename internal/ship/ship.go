// Package ship implements the exit gate of §6.5: the eight deterministic
// steps a task must pass before devclean opens a PR. Every check is pure
// code; no model is involved in verification (§6.5, reglas del equipo).
//
// Los pasos corren en orden y la compuerta se detiene en el primero que
// falla: sin PR, con la razón exacta.
package ship

import (
	"context"
	"time"

	"github.com/Pastranauwu/devclean/internal/config"
	"github.com/Pastranauwu/devclean/internal/room"
	"github.com/Pastranauwu/devclean/internal/task"
)

// DefaultTimeout is the fallback for the bisectable step's test run.
const DefaultTimeout = 5 * time.Minute

// Paso is the result of one exit-gate step, in order.
type Paso struct {
	Nombre  string `json:"nombre"`
	OK      bool   `json:"ok"`
	Detalle string `json:"detalle,omitempty"`
}

// Resultado is the outcome of the exit gate on one task.
type Resultado struct {
	ID       string `json:"id"`
	Pasos    []Paso `json:"pasos"`
	Aprobado bool   `json:"aprobado"`
	PR       string `json:"pr,omitempty"`

	// Resumen para las métricas, aunque la esclusa frene antes.
	LineasMas   int  `json:"lineas_mas,omitempty"`
	LineasMenos int  `json:"lineas_menos,omitempty"`
	Ruido       int  `json:"ruido,omitempty"`
	Conflicto   bool `json:"conflicto,omitempty"`
}

// Opciones carries the exit gate's dependencies.
type Opciones struct {
	Root    string
	Room    room.Room
	Task    task.Task
	Config  config.Config
	Modelo  string        // del último intento, para el trailer Agent:
	Base    string        // rama base sobre la que rebasear
	Timeout time.Duration // timeout del paso bisectable
	DryRun  bool          // corre todo menos abrir el PR
}

// Run executes the eight steps in order and returns the gate result.
// The first failing step stops the gate (la compuerta se frena ahí).
func Run(ctx context.Context, o Opciones) Resultado {
	res := Resultado{ID: o.Task.ID}
	if o.Base == "" {
		o.Base = "HEAD"
	}
	if o.Timeout <= 0 {
		o.Timeout = DefaultTimeout
	}

	// 1. base — rebase sobre la rama base
	target, conflictos, err := rebase(ctx, o.Root, o.Room.Path, o.Base, o.Room.Rama)
	if err != nil {
		if len(conflictos) > 0 {
			res.Conflicto = true
			res.Pasos = append(res.Pasos, Paso{"base", false, "rebase en conflicto · archivos: " + unir(conflictos) + " · resuélvelo a mano"})
		} else {
			res.Pasos = append(res.Pasos, Paso{"base", false, "no se pudo rebasear · " + err.Error()})
		}
		return res
	}
	res.Pasos = append(res.Pasos, Paso{"base", true, "rebaseado sobre " + target})

	// 2. historial — aplanar los wip en un commit limpio
	tipo := tipoCommit(o.Task.Titulo)
	cuenta, _, err := aplanar(ctx, o.Room.Path, target, o.Task.Titulo, tipo, o.Modelo)
	if err != nil {
		res.Pasos = append(res.Pasos, Paso{"historial", false, err.Error()})
		return res
	}
	res.Pasos = append(res.Pasos, Paso{"historial", true, itoa(cuenta) + " guardados → 1 commit"})

	// el diff del commit aplanado alimenta a los escáneres
	diff, archivos, mas, menos, err := diffAplanado(o.Room.Path, target)
	if err != nil {
		res.Pasos = append(res.Pasos, Paso{"ruido", false, err.Error()})
		return res
	}
	res.LineasMas = mas
	res.LineasMenos = menos

	// 3. ruido — prints de debug, temporales, código comentado
	if h := escanearRuido(diff, archivos); len(h) > 0 {
		res.Ruido = len(h)
		res.Pasos = append(res.Pasos, Paso{"ruido", false, resumenHallazgos(h)})
		return res
	}
	res.Pasos = append(res.Pasos, Paso{"ruido", true, "sin ruido"})

	// 4. secretos — en el diff y en el commit
	if h := escanearSecretos(diff); len(h) > 0 {
		res.Pasos = append(res.Pasos, Paso{"secretos", false, resumenHallazgos(h)})
		return res
	}
	res.Pasos = append(res.Pasos, Paso{"secretos", true, "sin secretos"})

	// 5. presupuesto — limite_lineas y archivos tocados
	if detalle, ok := verificarPresupuesto(mas, menos, len(archivos), o.Task); !ok {
		res.Pasos = append(res.Pasos, Paso{"presupuesto", false, detalle})
		return res
	} else {
		res.Pasos = append(res.Pasos, Paso{"presupuesto", true, detalle})
	}

	// 6. bisectable — el commit compila y pasa las pruebas
	if detalle, ok := verificarBisectable(ctx, o.Room.Path, o.Config.Pruebas, o.Timeout); !ok {
		res.Pasos = append(res.Pasos, Paso{"bisectable", false, detalle})
		return res
	} else {
		res.Pasos = append(res.Pasos, Paso{"bisectable", true, detalle})
	}

	// 7. handoff — qué cambió, qué no, cómo verificar
	cuerpo := generarHandoff(o.Task, archivos, mas, menos)
	res.Pasos = append(res.Pasos, Paso{"handoff", true, ""})

	// 8. pr — abrir y liberar el cuarto
	if o.DryRun {
		res.Pasos = append(res.Pasos, Paso{"pr", true, "dry-run · sin PR"})
		res.Aprobado = true
		return res
	}
	url, err := abrirPR(ctx, o.Root, o.Room, o.Base, o.Task.Titulo, cuerpo)
	if err != nil {
		res.Pasos = append(res.Pasos, Paso{"pr", false, err.Error()})
		return res
	}
	res.Pasos = append(res.Pasos, Paso{"pr", true, url})
	res.PR = url
	res.Aprobado = true
	return res
}

// PrimerMotivo returns the detail of the first failed step.
func (r Resultado) PrimerMotivo() string {
	for _, p := range r.Pasos {
		if !p.OK {
			return p.Detalle
		}
	}
	return ""
}
