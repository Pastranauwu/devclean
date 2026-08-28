// Package ship implements the exit gate of §6.5: the nine deterministic
// steps a task must pass before devclean opens a PR. Every check is pure
// code; no model is involved in verification (§6.5, reglas del equipo).
//
// Los pasos corren en orden y la compuerta se detiene en el primero que
// falla: sin PR, con la razón exacta.
package ship

import (
	"context"
	"strings"
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
	LineasMas   int      `json:"lineas_mas,omitempty"`
	LineasMenos int      `json:"lineas_menos,omitempty"`
	Ruido       int      `json:"ruido,omitempty"`
	Conflicto   bool     `json:"conflicto,omitempty"`
	Brecha      *float64 `json:"brecha,omitempty"` // visible_pct - hidden_pct (§6.8)
}

// Opciones carries the exit gate's dependencies.
type Opciones struct {
	Root     string
	Room     room.Room
	Task     task.Task
	Config   config.Config
	Modelo   string        // del último intento, para el trailer Agent:
	Base     string        // rama base sobre la que rebasear
	Timeout  time.Duration // timeout del paso bisectable
	DryRun   bool          // corre todo menos abrir el PR
	Progreso func(Paso)    // llamado tras cada paso, para el TUI; nil = silencio
}

// Run executes the nine steps in order and returns the gate result.
// The first failing step stops the gate (la compuerta se frena ahí).
func Run(ctx context.Context, o Opciones) Resultado {
	res := Resultado{ID: o.Task.ID}
	if o.Base == "" {
		o.Base = "HEAD"
	}
	if o.Timeout <= 0 {
		o.Timeout = DefaultTimeout
	}
	apuntar := func(p Paso) {
		res.Pasos = append(res.Pasos, p)
		if o.Progreso != nil {
			o.Progreso(p)
		}
	}

	// 1. base — rebase sobre la rama base
	target, conflictos, err := rebase(ctx, o.Root, o.Room.Path, o.Base, o.Room.Rama)
	if err != nil {
		if len(conflictos) > 0 {
			res.Conflicto = true
			apuntar(Paso{"base", false, "rebase en conflicto · archivos: " + unir(conflictos) + " · resuélvelo a mano"})
		} else {
			apuntar(Paso{"base", false, "no se pudo rebasear · " + err.Error()})
		}
		return res
	}
	apuntar(Paso{"base", true, "rebaseado sobre " + target})

	// 2. historial — aplanar los wip en un commit limpio
	tipo := tipoCommit(o.Task.Titulo)
	cuenta, _, err := aplanar(ctx, o.Room.Path, target, o.Task.Titulo, tipo, o.Modelo)
	if err != nil {
		apuntar(Paso{"historial", false, err.Error()})
		return res
	}
	apuntar(Paso{"historial", true, itoa(cuenta) + " guardados → 1 commit"})

	// el diff del commit aplanado alimenta a los escáneres
	diff, archivos, mas, menos, err := diffAplanado(o.Room.Path, target)
	if err != nil {
		apuntar(Paso{"ruido", false, err.Error()})
		return res
	}
	res.LineasMas = mas
	res.LineasMenos = menos

	// 3. ruido — prints de debug, temporales, código comentado
	if h := escanearRuido(diff, archivos); len(h) > 0 {
		res.Ruido = len(h)
		apuntar(Paso{"ruido", false, resumenHallazgos(h)})
		return res
	}
	apuntar(Paso{"ruido", true, "sin ruido"})

	// 4. secretos — en el diff y en el commit
	if h := escanearSecretos(diff); len(h) > 0 {
		apuntar(Paso{"secretos", false, resumenHallazgos(h)})
		return res
	}
	apuntar(Paso{"secretos", true, "sin secretos"})

	// 5. presupuesto — limite_lineas y archivos tocados
	if detalle, ok := verificarPresupuesto(mas, menos, len(archivos), o.Task); !ok {
		apuntar(Paso{"presupuesto", false, detalle})
		return res
	} else {
		apuntar(Paso{"presupuesto", true, detalle})
	}

	// 6. interfaces — entregó lo que sus hermanas consumen (§6.10)
	if faltan := verificarExpone(o.Task.Expone, diff); len(faltan) > 0 {
		apuntar(Paso{"interfaces", false, "no expone lo prometido: " + strings.Join(faltan, "; ")})
		return res
	}
	apuntar(Paso{"interfaces", true, "expone lo prometido"})

	// 6.5 dependencias — verifica el grafo de imports del diff (§6.10)
	if detalle, ok := verificarDependencias(diff, o.Config.ReglasImport); !ok {
		apuntar(Paso{"dependencias", false, detalle})
		return res
	} else if len(o.Config.ReglasImport) > 0 {
		apuntar(Paso{"dependencias", true, detalle})
	}

	// 7. bisectable — el commit compila y pasa las pruebas
	if detalle, ok := verificarBisectable(ctx, o.Room.Path, o.Config.Pruebas, o.Timeout); !ok {
		apuntar(Paso{"bisectable", false, detalle})
		return res
	} else {
		apuntar(Paso{"bisectable", true, detalle})
	}

	// 8. suite_oculta — hidden test gate (§6.8); skipped if no sealed suite
	pruebas := o.Config.Pruebas
	if pruebas == "" {
		pruebas = o.Task.ListoCuando
	}
	if brecha, detalle, suiteOK := verificarSuiteOculta(ctx, o.Root, o.Room.Path, o.Task, pruebas, o.Timeout); brecha != nil || !suiteOK {
		res.Brecha = brecha
		if suiteOK {
			apuntar(Paso{"suite_oculta", true, detalle})
		} else {
			apuntar(Paso{"suite_oculta", false, detalle})
			return res
		}
	}

	// 9. handoff — qué cambió, qué no, cómo verificar
	cuerpo := generarHandoff(o.Task, archivos, mas, menos)
	apuntar(Paso{"handoff", true, ""})

	// 10. pr — abrir y liberar el cuarto
	if o.DryRun {
		apuntar(Paso{"pr", true, "dry-run · sin PR"})
		res.Aprobado = true
		return res
	}
	url, err := abrirPR(ctx, o.Root, o.Room, o.Base, o.Task.Titulo, cuerpo)
	if err != nil {
		apuntar(Paso{"pr", false, err.Error()})
		return res
	}
	apuntar(Paso{"pr", true, url})
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
