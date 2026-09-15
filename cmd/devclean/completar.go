package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/Pastranauwu/devclean/internal/config"
	"github.com/Pastranauwu/devclean/internal/plan"
	"github.com/Pastranauwu/devclean/internal/spec"
	"github.com/Pastranauwu/devclean/internal/task"
	"github.com/Pastranauwu/devclean/internal/tui"
)

// faltaContrato reporta si una tarea del spec rápido no llega a contrato:
// sin listo_cuando no hay "listo", y sin tocar_solo la esclusa de entrada
// no la deja correr junto a otras.
func faltaContrato(t task.Task, total int) bool {
	return strings.TrimSpace(t.ListoCuando) == "" || (total > 1 && len(t.TocarSolo) == 0)
}

func contarSinContrato(ts []task.Task) int {
	n := 0
	for _, t := range ts {
		if faltaContrato(t, len(ts)) {
			n++
		}
	}
	return n
}

// completarSpec deja que el planificador escriba lo que el humano no
// escribió. El spec rápido es una lista de títulos: quién decide QUÉ se
// hace sigue siendo el humano, y el modelo pone el cómo se verifica, qué
// toca y de qué depende. Lo que el humano sí escribió no se pisa.
func completarSpec(root string, s *spec.Spec) error {
	n := contarSinContrato(s.Tasks)
	if n == 0 {
		return nil
	}
	cfg, err := config.Load(root)
	if err != nil {
		return err
	}
	ex, err := elegirEjecutor(cfg.Cli)
	if err != nil {
		return err
	}
	modelo := config.ModeloRol(cfg, "planificador")
	pctx, zonas, patrones, err := contextoPlan(root, cfg)
	if err != nil {
		return err
	}
	// los ids van antes que el modelo: así puede escribir depende_de con
	// los ids que de verdad van a existir
	if s.Tasks, err = spec.AssignCorrelativeIDs(config.TasksDir(root), s.Tasks); err != nil {
		return err
	}

	var bs []plan.Borrador
	generar := func() error {
		var err error
		bs, err = plan.Completar(context.Background(), generadorPlan{ex: ex, modelo: modelo, root: root}, pctx, s.Feature, s.Reglas, s.Tasks)
		return err
	}
	if esTUI() {
		err = tui.Esperar(fmt.Sprintf("completando %d tareas · %s", n, modelo), generar)
	} else {
		err = generar()
	}
	if err != nil {
		return err
	}
	sanearAlcance(bs, zonas, patrones, pctx.Ocupados)

	ids := make([]string, len(s.Tasks))
	for i, t := range s.Tasks {
		ids[i] = t.ID
	}
	defLineas := task.DefaultLimiteLineas
	if s.Limites.Lineas > 0 {
		defLineas = s.Limites.Lineas
	}
	total := len(s.Tasks)
	for i := range s.Tasks {
		if faltaContrato(s.Tasks[i], total) {
			s.Tasks[i] = completarTarea(s.Tasks[i], bs[i], ids, defLineas, s.Agente)
		}
	}
	out.Line("· el planificador completó %d de %d tareas · revísalas con devclean board", n, total)
	return nil
}

// completarTarea rellena los campos vacíos de t con los del borrador.
func completarTarea(t task.Task, b plan.Borrador, ids []string, defLineas int, agenteSpec string) task.Task {
	if strings.TrimSpace(t.ListoCuando) == "" {
		t.ListoCuando = b.ListoCuando
	}
	if len(t.TocarSolo) == 0 {
		t.TocarSolo = b.TocarSolo
	}
	if len(t.NoTocar) == 0 {
		t.NoTocar = b.NoTocar
	}
	if len(t.DependeDe) == 0 {
		t.DependeDe = dependenciasDelModelo(b.DependeDe, ids)
	}
	if len(t.Expone) == 0 {
		t.Expone = b.Expone
	}
	if len(t.Usa) == 0 {
		t.Usa = b.Usa
	}
	if t.Porque == "" {
		t.Porque = b.Porque
	}
	if t.Riesgos == "" {
		t.Riesgos = b.Riesgos
	}
	if t.Peso == "" {
		t.Peso = b.Peso
	}
	// el agente por defecto del spec es una decisión del humano
	if t.Agente == "" && agenteSpec == "" {
		t.Agente = b.Agente
	}
	if t.Notas == "" {
		t.Notas = b.Como
	}
	// un límite igual al por defecto es uno que nadie escribió: la
	// estimación del modelo por tarea vale más que un número fijo
	if t.LimiteLineas == 0 || t.LimiteLineas == defLineas {
		t.LimiteLineas = plan.AcotarLimiteLineas(b.LimiteLineas, defLineas)
	}
	return t
}

// dependenciasDelModelo acepta los ids de la lista tal cual y, si el
// modelo numeró por su cuenta ("T-001" sin estar en la lista), lo lee
// como posición.
func dependenciasDelModelo(deps, ids []string) []string {
	enLista := make(map[string]bool, len(ids))
	for _, id := range ids {
		enLista[id] = true
	}
	var out []string
	for _, d := range deps {
		if !enLista[d] {
			d = task.DependenciasPorPosicion([]string{d}, ids)[0]
		}
		out = append(out, d)
	}
	return out
}
