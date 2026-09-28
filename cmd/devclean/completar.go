package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/Pastranauwu/devclean/internal/config"
	"github.com/Pastranauwu/devclean/internal/esqueleto"
	"github.com/Pastranauwu/devclean/internal/plan"
	"github.com/Pastranauwu/devclean/internal/spec"
	"github.com/Pastranauwu/devclean/internal/task"
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
	// En el modo Requirements as Code no hay tareas humanas que completar:
	// el planificador produce el IR entero a partir de intención y reglas.
	if len(s.Tasks) == 0 && len(s.Requirements) > 0 {
		return planearRequirements(root, s)
	}
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
	err = esperarPlan(fmt.Sprintf("completando %d tareas · %s", n, modelo), func(avance func(string)) error {
		var err error
		bs, err = plan.Completar(context.Background(), planGuardado{generadorPlan{ex: ex, modelo: modelo, root: root, effort: "medium", avance: avance}, root}, pctx, s.Feature, s.Reglas, s.Tasks)
		return err
	})
	if err != nil {
		return err
	}
	sanearAlcance(bs, zonas, patrones, pctx.Ocupados)
	sanearSkills(bs, pctx.Skills)
	if pctx.PruebasPropias {
		ampliarPruebasPropias(bs)
	}

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

// planearRequirements convierte la intención del spec en el pedido del
// arquitecto: requirements, reglas y aceptación, numerados para que el
// esqueleto pueda citar a qué requisito responde cada módulo.
func planearRequirements(root string, s *spec.Spec) error {
	var pedido strings.Builder
	fmt.Fprintf(&pedido, "Feature: %s\n", s.Feature)
	nuevos, hechos, retirados := deltaRequirements(s.Previos, s.Requirements)
	if len(hechos) > 0 {
		pedido.WriteString("Ya implementados en un cambio anterior (están en el código y en " + esqueleto.Documento + "; no generes tareas para ellos salvo que lo nuevo los cambie):\n")
		for _, r := range hechos {
			fmt.Fprintf(&pedido, "- %s\n", r)
		}
	}
	if len(retirados) > 0 {
		pedido.WriteString("Retirados del spec (quítalos del código y de " + esqueleto.Documento + "):\n")
		for _, r := range retirados {
			fmt.Fprintf(&pedido, "- %s\n", r)
		}
	}
	if len(nuevos) > 0 {
		pedido.WriteString("Requerimientos obligatorios:\n")
		for i, r := range nuevos {
			fmt.Fprintf(&pedido, "R%d. %s\n", i+1, r)
		}
	} else {
		pedido.WriteString("Los requerimientos no cambiaron: cambiaron las reglas, la aceptación o las restricciones. Ajusta solo lo que eso exige.\n")
	}
	if len(s.Reglas) > 0 {
		pedido.WriteString("Reglas obligatorias:\n")
		for _, r := range s.Reglas {
			fmt.Fprintf(&pedido, "- %s\n", r)
		}
	}
	if len(s.Acceptance) > 0 {
		pedido.WriteString("Aceptación global (la prueba de integración tiene que cubrirla):\n")
		for _, a := range s.Acceptance {
			fmt.Fprintf(&pedido, "- %s", a.Criterion)
			if a.Command != "" {
				fmt.Fprintf(&pedido, " · comando: %s", a.Command)
			}
			pedido.WriteByte('\n')
		}
	}
	return planearEsqueleto(root, s, pedido.String())
}

// deltaRequirements separa el spec actual contra el último planeado:
// nuevos (o reescritos), los que ya estaban y los que se quitaron. Sin
// plan previo, todo es nuevo.
func deltaRequirements(previos, actuales []string) (nuevos, hechos, retirados []string) {
	antes := map[string]bool{}
	for _, r := range previos {
		antes[r] = true
	}
	ahora := map[string]bool{}
	for _, r := range actuales {
		ahora[r] = true
		if antes[r] {
			hechos = append(hechos, r)
		} else {
			nuevos = append(nuevos, r)
		}
	}
	for _, r := range previos {
		if !ahora[r] {
			retirados = append(retirados, r)
		}
	}
	return nuevos, hechos, retirados
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
		t.DependeDe = dependenciasDelModelo(b.DependeDe, ids, nil)
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
	if t.Skills == nil {
		t.Skills = b.Skills
	}
	if t.Notas == "" {
		t.Notas = b.Como
	} else if b.Como != "" && b.Como != t.Notas {
		t.Notas += "\n\n" + b.Como
	}
	// El límite del contrato o del spec lo decide el humano.
	if t.LimiteLineas == 0 {
		t.LimiteLineas = plan.AcotarLimiteLineas(b.LimiteLineas, defLineas)
	}
	return t
}

// dependenciasDelModelo acepta tal cual los ids de la lista y los de
// tareas previas y, si el modelo numeró por su cuenta ("T-001" sin estar
// en ninguna), lo lee como posición.
func dependenciasDelModelo(deps, ids []string, previas map[string]bool) []string {
	enLista := make(map[string]bool, len(ids))
	for _, id := range ids {
		enLista[id] = true
	}
	var out []string
	for _, d := range deps {
		// "2" con un plan que arranca en T-002 es el id, no la segunda
		// tarea: el prompt dice desde qué id numera. Leído como posición
		// armaba ciclos (el esqueleto de gastos: 2 → T-003)
		if n, err := strconv.Atoi(strings.TrimSpace(d)); err == nil && enLista[fmt.Sprintf("T-%03d", n)] {
			d = fmt.Sprintf("T-%03d", n)
		}
		if !enLista[d] && !previas[d] {
			d = task.DependenciasPorPosicion([]string{d}, ids)[0]
		}
		out = append(out, d)
	}
	return out
}
