package main

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/Pastranauwu/devclean/internal/config"
	"github.com/Pastranauwu/devclean/internal/constitution"
	"github.com/Pastranauwu/devclean/internal/examiner"
	"github.com/Pastranauwu/devclean/internal/executor"
	"github.com/Pastranauwu/devclean/internal/gate"
	"github.com/Pastranauwu/devclean/internal/loop"
	"github.com/Pastranauwu/devclean/internal/overlap"
	"github.com/Pastranauwu/devclean/internal/room"
	"github.com/Pastranauwu/devclean/internal/state"
	"github.com/Pastranauwu/devclean/internal/task"
	"github.com/Pastranauwu/devclean/internal/tui"
)

// runResult es una tarea de la corrida, con su desenlace.
type runResult struct {
	ID       string `json:"id"`
	Titulo   string `json:"titulo"`
	Estado   string `json:"estado"` // lista | detenida | rechazada
	Intentos int    `json:"intentos,omitempty"`
	Motivo   string `json:"motivo,omitempty"`
}

func newRunCmd() *cobra.Command {
	var agentes int
	var ejecutor string
	var modelo string
	cmd := &cobra.Command{
		Use:   "run",
		Short: "ejecuta las tareas pendientes en paralelo",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCmd(agentes, ejecutor, modelo)
		},
	}
	cmd.Flags().IntVar(&agentes, "agentes", 1, "tareas en paralelo")
	cmd.Flags().StringVar(&ejecutor, "ejecutor", "", "opencode o claude (por defecto, el primero disponible)")
	cmd.Flags().StringVar(&modelo, "modelo", "", "modelo del ejecutor (por defecto, el suyo)")
	return cmd
}

func runCmd(agentes int, ejecutor, modelo string) error {
	if agentes < 1 {
		return errors.New("--agentes inválido · mínimo 1")
	}
	root, err := projectRoot()
	if err != nil {
		return err
	}
	cfg, err := config.Load(root)
	if err != nil {
		return err
	}
	constitucion, err := constitution.Load(root)
	if err != nil {
		return err
	}
	tareas, err := task.List(config.TasksDir(root))
	if err != nil {
		return err
	}

	var pendientes, existentes []task.Task
	for _, t := range tareas {
		s, err := state.Get(root, t.ID)
		if err != nil {
			return err
		}
		switch s.Estado {
		case state.Pendiente:
			pendientes = append(pendientes, t)
		case state.EnCurso:
			existentes = append(existentes, t)
		}
	}
	if len(pendientes) == 0 {
		out.Line("sin tareas pendientes · empieza con devclean task add \"lo que necesitas\"")
		return nil
	}

	timeout := gate.DefaultTimeout
	if cfg.TimeoutEsclusa > 0 {
		timeout = time.Duration(cfg.TimeoutEsclusa) * time.Second
	}

	results := make([]runResult, 0, len(pendientes))

	// esclusa de entrada por tarea: la que no pasa, se rechaza antes de
	// gastar un solo token (§6.3)
	var aprobadas []task.Task
	for _, t := range pendientes {
		res := gate.Run(context.Background(), root, cfg, t, existentes, timeout)
		if res.Aprobada {
			aprobadas = append(aprobadas, t)
			continue
		}
		results = append(results, runResult{ID: t.ID, Titulo: t.Titulo, Estado: "rechazada", Motivo: motivoRechazo(res)})
	}

	// §6.10: una tarea no puede consumir una firma que nadie promete.
	// gate.Run no puede verlo: solo recibe las tareas en_curso, y quien
	// expone suele estar pendiente en la misma corrida.
	aprobadas, results = rechazarUsaHuerfano(aprobadas, tareas, results)

	if len(aprobadas) == 0 {
		sortRunResults(results)
		_ = emitirResultados(results)
		return errors.New("ninguna tarea superó la esclusa · corrige los contratos y reintenta")
	}

	if ejecutor == "" {
		ejecutor = cfg.Cli
	}
	ex, err := elegirEjecutor(ejecutor)
	if err != nil {
		return err
	}

	if esTUI() {
		if len(results) > 0 {
			sortRunResults(results)
			_ = emitirResultados(results)
		}
		_, err := correrConTUI(context.Background(), root, cfg, ex, modelo, constitucion, aprobadas, agentes)
		return err
	}

	results = append(results, ejecutarOlas(context.Background(), root, cfg, ex, modelo, constitucion, aprobadas, agentes, nil)...)
	sortRunResults(results)
	return emitirResultados(results)
}

// correrConTUI ejecuta la corrida dentro del tablero en vivo y devuelve
// los resultados para imprimirlos al final.
func correrConTUI(ctx context.Context, root string, cfg config.Config, ex executor.Executor, modelo, constitucion string, aprobadas []task.Task, agentes int) ([]runResult, error) {
	filas := make([]tui.FilaRun, 0, len(aprobadas))
	for _, t := range aprobadas {
		filas = append(filas, tui.FilaRun{ID: t.ID, Titulo: t.Titulo, Limite: t.LimiteIntentos})
	}
	var results []runResult
	err := tui.CorrerRun(filas, func(emit func(tui.EventoRun)) {
		results = ejecutarOlas(ctx, root, cfg, ex, modelo, constitucion, aprobadas, agentes, emit)
	})
	return results, err
}

// ejecutarOlas corre las tareas por oleadas según depende_de (Fase 2):
// una ola solo arranca cuando sus dependencias ya están verdes. El
// trabajo verde de cada ola se integra en una rama temporal y la
// siguiente ola arranca desde ahí, así la cadena comparte estado.
func ejecutarOlas(ctx context.Context, root string, cfg config.Config, ex executor.Executor, modelo, constitucion string, aprobadas []task.Task, agentes int, emit func(tui.EventoRun)) []runResult {
	var results []runResult
	procesadas := map[string]bool{}
	integrada := map[string]bool{}

	tieneDeps := false
	for _, t := range aprobadas {
		if len(t.DependeDe) > 0 {
			tieneDeps = true
			break
		}
	}

	base := cfg.Base
	if tieneDeps {
		if err := room.ResetIntegration(ctx, root, cfg.Base); err != nil {
			for _, t := range aprobadas {
				results = append(results, runResult{ID: t.ID, Titulo: t.Titulo, Estado: "detenida", Motivo: err.Error()})
			}
			return results
		}
		base = room.IntegrationBranch
		results = append(results, sembrarVerdesPrevias(ctx, root, aprobadas, integrada)...)
	}

	for {
		var ola []task.Task
		for _, t := range aprobadas {
			if procesadas[t.ID] {
				continue
			}
			if depsVerdes(t.DependeDe, integrada) {
				ola = append(ola, t)
			}
		}

		if len(ola) == 0 {
			for _, t := range aprobadas {
				if procesadas[t.ID] {
					continue
				}
				results = append(results, runResult{
					ID: t.ID, Titulo: t.Titulo, Estado: "rechazada",
					Motivo: "bloqueada · depende de " + strings.Join(depsFaltantes(t.DependeDe, integrada), ", ") + " que no salió verde",
				})
				procesadas[t.ID] = true
			}
			break
		}

		asignadas, rechazadas := asignar(ola)
		for _, r := range rechazadas {
			results = append(results, r)
			procesadas[r.ID] = true
		}

		var verdes []runResult
		if len(asignadas) > 0 {
			r := correr(ctx, root, cfg, ex, modelo, constitucion, base, asignadas, agentes, emit)
			results = append(results, r...)
			for _, rr := range r {
				procesadas[rr.ID] = true
				if rr.Estado == "lista" {
					verdes = append(verdes, rr)
				}
			}
		}

		if !tieneDeps {
			continue
		}
		sort.Slice(verdes, func(i, j int) bool { return verdes[i].ID < verdes[j].ID })
		for _, v := range verdes {
			if salida, err := room.Integrar(ctx, root, v.ID); err != nil {
				results = append(results, runResult{
					ID: v.ID, Titulo: v.Titulo, Estado: "detenida",
					Motivo: "no se pudo integrar con la oleada anterior · " + salida,
				})
				continue
			}
			integrada[v.ID] = true
		}
	}
	return results
}

// rechazarUsaHuerfano saca las tareas que consumen una firma que ningún
// contrato del proyecto expone (§6.10). Es un plan incoherente: el
// agente iba a inventar esa interfaz, y el desajuste recién aparecería
// al juntar las dos ramas.
func rechazarUsaHuerfano(aprobadas, todas []task.Task, results []runResult) ([]task.Task, []runResult) {
	expuestas := map[string]bool{}
	for _, t := range todas {
		for _, f := range t.Expone {
			expuestas[task.NombreDeFirma(f)] = true
		}
	}

	var ok []task.Task
	for _, t := range aprobadas {
		var huerfanas []string
		for _, f := range t.Usa {
			if n := task.NombreDeFirma(f); n != "" && !expuestas[n] {
				huerfanas = append(huerfanas, f)
			}
		}
		if len(huerfanas) == 0 {
			ok = append(ok, t)
			continue
		}
		results = append(results, runResult{
			ID: t.ID, Titulo: t.Titulo, Estado: "rechazada",
			Motivo: "usa una firma que ninguna tarea expone: " + strings.Join(huerfanas, "; ") + " · agrégala al expone de quien la produce",
		})
	}
	return ok, results
}

// sembrarVerdesPrevias marca como verdes las dependencias que ya
// quedaron `lista` en una corrida anterior, e integra su rama para que
// la oleada nueva vea ese código.
//
// Sin esto, `integrada` arrancaba vacío en cada corrida: una tarea que
// dependía de trabajo ya hecho se rechazaba con "bloqueada · depende de
// T-00X que no salió verde" para siempre, y no había forma de avanzar
// salvo rehacer todo de una sola corrida. Una dependencia verde cuya
// rama ya no existe se da por satisfecha: `ship` la entregó y liberó su
// cuarto, así que su código ya está en la rama base.
func sembrarVerdesPrevias(ctx context.Context, root string, aprobadas []task.Task, integrada map[string]bool) []runResult {
	enOla := map[string]bool{}
	for _, t := range aprobadas {
		enOla[t.ID] = true
	}

	var previas []string
	vistas := map[string]bool{}
	for _, t := range aprobadas {
		for _, d := range t.DependeDe {
			if enOla[d] || vistas[d] {
				continue
			}
			vistas[d] = true
			previas = append(previas, d)
		}
	}
	sort.Strings(previas)

	var results []runResult
	for _, id := range previas {
		s, err := state.Get(root, id)
		if err != nil || s.Estado != state.Lista {
			continue
		}
		if !room.RamaExiste(ctx, root, id) {
			// verde y ya entregada: su código vive en la rama base
			integrada[id] = true
			continue
		}
		if salida, err := room.Integrar(ctx, root, id); err != nil {
			results = append(results, runResult{
				ID: id, Estado: "detenida",
				Motivo: "no se pudo reusar el trabajo verde de una corrida anterior · " + salida,
			})
			continue
		}
		integrada[id] = true
	}
	return results
}

// depsVerdes reporta si todas las dependencias de una tarea ya salieron
// verdes. Sin dependencias, siempre verde.
func depsVerdes(deps []string, verde map[string]bool) bool {
	for _, d := range deps {
		if !verde[d] {
			return false
		}
	}
	return true
}

// depsFaltantes lista las dependencias que no salieron verdes.
func depsFaltantes(deps []string, verde map[string]bool) []string {
	var faltantes []string
	for _, d := range deps {
		if !verde[d] {
			faltantes = append(faltantes, d)
		}
	}
	return faltantes
}

// asignar reparte las tareas aprobadas para correr juntas: aplica A.4 al
// conjunto (tocar_solo vacío solo vale en solitario) y descarta las que
// se cruzan, en orden de ID.
func asignar(aprobadas []task.Task) ([]task.Task, []runResult) {
	sort.Slice(aprobadas, func(i, j int) bool { return aprobadas[i].ID < aprobadas[j].ID })
	if len(aprobadas) <= 1 {
		return aprobadas, nil
	}

	var rechazos []runResult
	var conAlcance []task.Task
	for _, t := range aprobadas {
		if len(t.TocarSolo) == 0 {
			rechazos = append(rechazos, runResult{
				ID: t.ID, Titulo: t.Titulo, Estado: "rechazada",
				Motivo: "tocar_solo obligatorio con más de una tarea activa · declara tus rutas",
			})
			continue
		}
		conAlcance = append(conAlcance, t)
	}

	var aceptadas []task.Task
	for _, t := range conAlcance {
		if c := gate.Cruce(t, aceptadas); !c.OK {
			rechazos = append(rechazos, runResult{ID: t.ID, Titulo: t.Titulo, Estado: "rechazada", Motivo: c.Motivo})
			continue
		}
		aceptadas = append(aceptadas, t)
	}
	return aceptadas, rechazos
}

// modeloParaTarea resuelve el modelo de una tarea (Fase 3): el flag
// --modelo gana; si no, el peso de la tarea (o el de la estrategia
// global) busca en `modelos`; y si no hay, cae al modelo del ejecutor.
func modeloParaTarea(cfg config.Config, flagModelo string, t task.Task) string {
	if flagModelo != "" {
		return flagModelo
	}
	peso := t.Peso
	if peso == "" {
		peso = cfg.PesoPorDefecto()
	}
	if m := cfg.ModeloPeso(peso); m != "" {
		return m
	}
	return config.ModeloRol(cfg, "ejecutor")
}

// elegirEjecutor devuelve el ejecutor pedido o el primero instalado.
func elegirEjecutor(nombre string) (executor.Executor, error) {
	if nombre != "" {
		var e executor.Executor
		switch nombre {
		case "opencode":
			e = executor.OpenCode{}
		case "claude":
			e = executor.Claude{}
		default:
			return nil, fmt.Errorf("ejecutor desconocido: %s · usa opencode o claude", nombre)
		}
		if err := e.Available(); err != nil {
			return nil, err
		}
		return e, nil
	}
	for _, e := range []executor.Executor{executor.OpenCode{}, executor.Claude{}} {
		if e.Available() == nil {
			return e, nil
		}
	}
	return nil, errors.New("ningún ejecutor disponible · instala opencode o claude")
}

// agenteExecutor adapta internal/executor a la interfaz loop.Agent. Es el
// único punto donde el comando toca el ejecutor: el bucle no lo importa.
type agenteExecutor struct {
	ex executor.Executor
}

func (a agenteExecutor) Name() string { return a.ex.Name() }

func (a agenteExecutor) Run(ctx context.Context, req loop.Request) (loop.Result, error) {
	res, err := a.ex.Run(ctx, executor.Request{
		RoomPath:     req.RoomPath,
		Prompt:       req.Prompt,
		AllowedGlobs: req.AllowedGlobs,
		Model:        req.Model,
		Timeout:      req.Timeout,
		Env:          req.Env,
	})
	return loop.Result{
		Stdout:   res.Stdout,
		Text:     res.Text,
		ExitCode: res.ExitCode,
		Tokens:   loop.Tokens{Entrada: res.Tokens.Input, Salida: res.Tokens.Output},
	}, err
}

// correr lanza las tareas con `agentes` trabajadores en paralelo. onEvent,
// si no es nil, recibe cada transición para el tablero en vivo.
func correr(ctx context.Context, root string, cfg config.Config, ex executor.Executor, modelo, constitucion, base string, asignadas []task.Task, agentes int, onEvent func(tui.EventoRun)) []runResult {
	// §6.9: solapamiento activo entre tareas de la misma oleada
	alertasOverlap := checkOverlapOla(root, asignadas)
	for _, a := range alertasOverlap {
		out.Line("⚠ SOLAPAMIENTO  %s", a)
	}

	jobs := make(chan task.Task)
	var wg sync.WaitGroup
	var mu sync.Mutex
	results := make([]runResult, 0, len(asignadas))

	for i := 0; i < agentes; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for t := range jobs {
				if onEvent != nil {
					onEvent(tui.EventoRun{ID: t.ID, Estado: "trabajando"})
				}
				r := correrUno(ctx, root, cfg, ex, modelo, base, constitucion, t)
				if onEvent != nil {
					onEvent(tui.EventoRun{ID: r.ID, Estado: r.Estado, Intentos: r.Intentos, Motivo: r.Motivo})
				}
				mu.Lock()
				results = append(results, r)
				mu.Unlock()
			}
		}()
	}
	for _, t := range asignadas {
		jobs <- t
	}
	close(jobs)
	wg.Wait()
	return results
}

// checkOverlapOla runs overlap checks between all pairs of tasks in a wave.
// Returns human-readable alerts for any detected overlap.
func checkOverlapOla(root string, tareas []task.Task) []string {
	var alertas []string
	for i, a := range tareas {
		for _, b := range tareas[i+1:] {
			asA, _ := loop.ReadAttempts(root, a.ID)
			asB, _ := loop.ReadAttempts(root, b.ID)
			r := overlap.CheckPar(root, a.ID, b.ID, asA, asB)
			if alert := r.Alerta(); alert != "" {
				alertas = append(alertas, alert)
			}
		}
	}
	return alertas
}

// resolverAgenteTarea determina el ejecutor, modelo y skills para una tarea concreta (Fase 2 / Zero-Config).
func resolverAgenteTarea(cfg config.Config, defaultEx executor.Executor, flagModelo string, t task.Task) (executor.Executor, string, []string) {
	nombreAgente := t.Agente
	if nombreAgente == "" {
		nombreAgente = "ejecutor"
	}

	cli := cfg.Cli
	if defaultEx != nil {
		cli = defaultEx.Name()
	}

	customAg, isCustom := cfg.Agentes[nombreAgente]

	ex := defaultEx
	if isCustom && customAg.Provider != "" {
		if e, err := elegirEjecutor(customAg.Provider); err == nil {
			ex = e
		}
	} else if ex == nil {
		if ag, ok := cfg.ObtenerAgente(nombreAgente); ok && ag.Provider != "" {
			if e, err := elegirEjecutor(ag.Provider); err == nil {
				ex = e
			}
		}
	}

	modelo := flagModelo
	if modelo == "" {
		if isCustom && customAg.Modelo != "" {
			modelo = customAg.Modelo
		} else if p, ok := cfg.Proveedores[nombreAgente]; ok && p.Modelo != "" {
			modelo = p.Modelo
		} else {
			defaults := config.DefaultAgentes(cli)
			if def, ok := defaults[nombreAgente]; ok && def.Modelo != "" {
				modelo = def.Modelo
			} else {
				modelo = modeloParaTarea(cfg, "", t)
			}
		}
	}

	var skills []string
	if ag, ok := cfg.ObtenerAgente(nombreAgente); ok {
		skills = ag.Skills
	}

	return ex, modelo, skills
}

// correrUno ejecuta una tarea completa: cuarto, esclusa de estado,
// bucle, y deja el estado final (lista o detenida). El cuarto no se
// destruye aquí: ship lo libera al entregar.
func correrUno(ctx context.Context, root string, cfg config.Config, ex executor.Executor, modelo, base, constitucion string, t task.Task) runResult {
	r, err := room.Create(ctx, root, t.ID, base)
	if err != nil {
		return runResult{ID: t.ID, Titulo: t.Titulo, Estado: "detenida", Motivo: err.Error()}
	}

	if err := state.Save(root, state.State{ID: t.ID, Estado: state.EnCurso, Rama: r.Rama, Puerto: r.Puerto}); err != nil {
		_ = room.Destroy(ctx, root, t.ID)
		return runResult{ID: t.ID, Titulo: t.Titulo, Estado: "detenida", Motivo: err.Error()}
	}

	exTarea, modeloTarea, skillsTarea := resolverAgenteTarea(cfg, ex, modelo, t)

	exam := examiner.Runner{Options: examiner.Options{
		Agent:    agenteExecutor{exTarea},
		Task:     t,
		Root:     root,
		Model:    config.ModeloRol(cfg, "planificador"),
		Timeout:  3 * time.Minute,
		Lenguaje: config.DetectLanguage(root),
	}}
	outcome, err := loop.Run(ctx, loop.Options{
		Agent:          agenteExecutor{exTarea},
		Root:           root,
		Room:           r,
		Task:           t,
		Model:          modeloTarea,
		Base:           base,
		PatronesPrueba: cfg.PatronesPrueba,
		Env:            []string{fmt.Sprintf("PORT=%d", r.Puerto)},
		Interfaces:     t.Usa,
		Constitucion:   constitucion,
		Skills:         skillsTarea,
		Examinador:     exam,
	})
	if err != nil {
		_ = state.Save(root, state.State{ID: t.ID, Estado: state.Detenida, Rama: r.Rama, Puerto: r.Puerto, UltimoError: err.Error()})
		return runResult{ID: t.ID, Titulo: t.Titulo, Estado: "detenida", Motivo: err.Error()}
	}

	if outcome.Verde {
		_ = state.Save(root, state.State{ID: t.ID, Estado: state.Lista, Intentos: outcome.Intentos, Rama: r.Rama, Puerto: r.Puerto})
		return runResult{ID: t.ID, Titulo: t.Titulo, Estado: "lista", Intentos: outcome.Intentos}
	}
	_ = state.Save(root, state.State{
		ID: t.ID, Estado: state.Detenida, Intentos: outcome.Intentos,
		Rama: r.Rama, Puerto: r.Puerto, UltimoError: outcome.UltimoError, Pregunta: outcome.Pregunta,
	})
	return runResult{ID: t.ID, Titulo: t.Titulo, Estado: "detenida", Intentos: outcome.Intentos, Motivo: outcome.Pregunta}
}

func motivoRechazo(res gate.Result) string {
	var partes []string
	for _, c := range res.Chequeos {
		if c.OK {
			continue
		}
		if c.Motivo != "" {
			partes = append(partes, c.Nombre+" · "+c.Motivo)
		} else {
			partes = append(partes, c.Nombre)
		}
	}
	if len(partes) == 0 {
		return res.PrimerMotivo()
	}
	return strings.Join(partes, " · ")
}

func sortRunResults(rs []runResult) {
	sort.Slice(rs, func(i, j int) bool { return rs[i].ID < rs[j].ID })
}

func emitirResultados(results []runResult) error {
	if err := out.Data(results); err != nil {
		return err
	}
	for _, r := range results {
		switch r.Estado {
		case "lista":
			out.Line("✓ %s  %s  · verde en %d intentos", r.ID, r.Titulo, r.Intentos)
		case "detenida":
			out.Line("⏸ %s  %s  · %s", r.ID, r.Titulo, r.Motivo)
		case "rechazada":
			out.Line("✗ %s  %s  · %s", r.ID, r.Titulo, r.Motivo)
		}
	}
	return nil
}
