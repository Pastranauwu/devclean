package main

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/Pastranauwu/devclean/internal/config"
	"github.com/Pastranauwu/devclean/internal/executor"
	"github.com/Pastranauwu/devclean/internal/gate"
	"github.com/Pastranauwu/devclean/internal/loop"
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
	if modelo == "" {
		modelo = config.ModeloRol(cfg, "ejecutor")
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
		results = append(results, runResult{ID: t.ID, Titulo: t.Titulo, Estado: "rechazada", Motivo: res.PrimerMotivo()})
	}

	asignadas, rechazadas := asignar(aprobadas)
	for _, r := range rechazadas {
		results = append(results, runResult{ID: r.ID, Titulo: r.Titulo, Estado: "rechazada", Motivo: r.Motivo})
	}

	if len(asignadas) == 0 {
		sortRunResults(results)
		_ = emitirResultados(results)
		return errors.New("ninguna tarea superó la esclusa · corrige los contratos y reintenta")
	}

	ex, err := elegirEjecutor(ejecutor)
	if err != nil {
		return err
	}

	if esTUI() {
		_, err := correrConTUI(context.Background(), root, cfg, ex, modelo, asignadas, agentes)
		return err
	}

	results = append(results, correr(context.Background(), root, cfg, ex, modelo, asignadas, agentes, nil)...)
	sortRunResults(results)
	return emitirResultados(results)
}

// correrConTUI ejecuta la corrida dentro del tablero en vivo y devuelve
// los resultados para imprimirlos al final.
func correrConTUI(ctx context.Context, root string, cfg config.Config, ex executor.Executor, modelo string, asignadas []task.Task, agentes int) ([]runResult, error) {
	filas := make([]tui.FilaRun, 0, len(asignadas))
	for _, t := range asignadas {
		filas = append(filas, tui.FilaRun{ID: t.ID, Titulo: t.Titulo, Limite: t.LimiteIntentos})
	}
	var results []runResult
	err := tui.CorrerRun(filas, func(emit func(tui.EventoRun)) {
		results = correr(ctx, root, cfg, ex, modelo, asignadas, agentes, emit)
	})
	return results, err
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
		ExitCode: res.ExitCode,
		Tokens:   loop.Tokens{Entrada: res.Tokens.Input, Salida: res.Tokens.Output},
	}, err
}

// correr lanza las tareas con `agentes` trabajadores en paralelo. onEvent,
// si no es nil, recibe cada transición para el tablero en vivo.
func correr(ctx context.Context, root string, cfg config.Config, ex executor.Executor, modelo string, asignadas []task.Task, agentes int, onEvent func(tui.EventoRun)) []runResult {
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
				r := correrUno(ctx, root, cfg, ex, modelo, t)
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

// correrUno ejecuta una tarea completa: cuarto, esclusa de estado,
// bucle, y deja el estado final (lista o detenida). El cuarto no se
// destruye aquí: ship lo libera al entregar.
func correrUno(ctx context.Context, root string, cfg config.Config, ex executor.Executor, modelo string, t task.Task) runResult {
	r, err := room.Create(ctx, root, t.ID, cfg.Base)
	if err != nil {
		return runResult{ID: t.ID, Titulo: t.Titulo, Estado: "detenida", Motivo: err.Error()}
	}

	if err := state.Save(root, state.State{ID: t.ID, Estado: state.EnCurso, Rama: r.Rama, Puerto: r.Puerto}); err != nil {
		_ = room.Destroy(ctx, root, t.ID)
		return runResult{ID: t.ID, Titulo: t.Titulo, Estado: "detenida", Motivo: err.Error()}
	}

	outcome, err := loop.Run(ctx, loop.Options{
		Agent:          agenteExecutor{ex},
		Root:           root,
		Room:           r,
		Task:           t,
		Model:          modelo,
		Base:           cfg.Base,
		PatronesPrueba: cfg.PatronesPrueba,
		Env:            []string{fmt.Sprintf("PORT=%d", r.Puerto)},
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
