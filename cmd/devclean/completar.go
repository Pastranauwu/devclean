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
	generar := func() error {
		var err error
		bs, err = plan.Completar(context.Background(), generadorPlan{ex: ex, modelo: modelo, root: root, effort: "medium"}, pctx, s.Feature, s.Reglas, s.Tasks)
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

func planearRequirements(root string, s *spec.Spec) error {
	cfg, err := config.Load(root)
	if err != nil {
		return err
	}
	ex, err := elegirEjecutor(cfg.Cli)
	if err != nil {
		return err
	}
	pctx, zonas, patrones, err := contextoPlan(root, cfg)
	if err != nil {
		return err
	}
	var pedido strings.Builder
	fmt.Fprintf(&pedido, "Feature: %s\nRequerimientos obligatorios:\n", s.Feature)
	for i, r := range s.Requirements {
		fmt.Fprintf(&pedido, "R%d. %s\n", i+1, r)
	}
	if len(s.Reglas) > 0 {
		pedido.WriteString("Reglas obligatorias:\n")
		for _, r := range s.Reglas {
			fmt.Fprintf(&pedido, "- %s\n", r)
		}
	}
	if len(s.Acceptance) > 0 {
		pedido.WriteString("Aceptación global (cada criterio debe quedar cubierto por listo_cuando o por una tarea final de integración):\n")
		for _, a := range s.Acceptance {
			fmt.Fprintf(&pedido, "- %s", a.Criterion)
			if a.Command != "" {
				fmt.Fprintf(&pedido, " · comando: %s", a.Command)
			}
			pedido.WriteByte('\n')
		}
	}
	var bs []plan.Borrador
	modelo := config.ModeloRol(cfg, "planificador")
	generar := func() error {
		var err error
		bs, err = plan.Generar(context.Background(), generadorPlan{ex: ex, modelo: modelo, root: root, effort: "medium"}, pctx, pedido.String())
		return err
	}
	if esTUI() {
		err = tui.Esperar("diseñando arquitectura y tareas · "+modelo, generar)
	} else {
		out.Line("· diseñando arquitectura y tareas · %s · %d requisitos", modelo, len(s.Requirements))
		err = generar()
	}
	if err != nil {
		return err
	}
	sanearAlcance(bs, zonas, patrones, pctx.Ocupados)
	sanearSkills(bs, pctx.Skills)
	if pctx.PruebasPropias {
		ampliarPruebasPropias(bs)
	}
	ids, err := idsCorrelativos(config.TasksDir(root), len(bs))
	if err != nil {
		return err
	}
	traducirDependencias(bs, ids)
	intentos := s.Limites.Intentos
	if intentos < 1 {
		intentos = task.DefaultLimiteIntentos
	}
	for i, b := range bs {
		s.Tasks = append(s.Tasks, task.Task{Version: task.Version, ID: ids[i], Titulo: b.Titulo, Porque: b.Porque, ListoCuando: b.ListoCuando, TocarSolo: b.TocarSolo, NoTocar: b.NoTocar, DependeDe: b.DependeDe, Expone: b.Expone, Usa: b.Usa, Riesgos: b.Riesgos, Peso: b.Peso, Agente: b.Agente, Skills: b.Skills, Notas: b.Como, LimiteIntentos: intentos, LimiteLineas: plan.AcotarLimiteLineas(b.LimiteLineas, s.Limites.Lineas)})
	}
	out.Line("· Requirements Analyzer + Planner generaron %d contratos internos", len(s.Tasks))

	// el plan no cierra la costura entre tareas: cada una prueba lo que su
	// contrato pide y ninguna prueba la cadena completa. La tarea derivada
	// entra por la frontera final, y su comando vuelve a correr sobre el
	// conjunto integrado como aceptación del feature.
	ids, err = idsCorrelativos(config.TasksDir(root), len(s.Tasks)+1)
	if err != nil {
		return err
	}
	if integracion, aceptacion, ok := spec.TareaDeIntegracion(*s, s.Tasks, config.DetectLanguage(root), ids[len(ids)-1]); ok {
		// la misma arquitectura que sus hermanas: sin ella su prompt
		// empieza distinto y no comparte el caché del resto del plan
		if arq := plan.SepararNotas(s.Tasks[0].Notas).Arquitectura; arq != "" {
			integracion.Notas = plan.MarcaArquitectura + arq + "\n\n" + plan.MarcaImplementacion + integracion.Notas
		}
		integracion.LimiteIntentos = intentos
		integracion.LimiteLineas = task.DefaultLimiteLineas
		s.Tasks = append(s.Tasks, integracion)
		s.Acceptance = append(s.Acceptance, aceptacion)
		out.Line("· %s prueba la costura entre tareas · %s", integracion.ID, integracion.ListoCuando)
	}
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
