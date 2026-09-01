package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/Pastranauwu/devclean/internal/config"
	"github.com/Pastranauwu/devclean/internal/loop"
	"github.com/Pastranauwu/devclean/internal/metrics"
	"github.com/Pastranauwu/devclean/internal/revisor"
	"github.com/Pastranauwu/devclean/internal/room"
	"github.com/Pastranauwu/devclean/internal/ship"
	"github.com/Pastranauwu/devclean/internal/state"
	"github.com/Pastranauwu/devclean/internal/task"
	"github.com/Pastranauwu/devclean/internal/tui"
)

func newShipCmd() *cobra.Command {
	var dryRun bool
	var todas bool
	var titulo string
	var integrar, revisar bool
	cmd := &cobra.Command{
		Use:   "ship [id]",
		Short: "esclusa de salida y PR",
		Long: `Con un id entrega esa tarea en su propio PR.

Con --todas entrega en UN solo PR todas las tareas que quedaron listas:
cada una pasa su esclusa de salida por separado y aporta exactamente un
commit, en orden de dependencia. Es la forma de no terminar con N pull
requests que se pisan entre sí.`,
		Example: `  devclean ship T-001
  devclean ship --todas
  devclean ship --todas --dry-run`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if todas {
				if len(args) > 0 {
					return errors.New("--todas entrega todas las tareas listas · no lleva id")
				}
				return runShipTodas(dryRun, titulo, integrar, revisar)
			}
			if len(args) == 0 {
				return errors.New("falta el id · usa devclean ship T-001 o devclean ship --todas")
			}
			return runShip(args[0], dryRun)
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "corre la esclusa sin abrir el PR")
	cmd.Flags().BoolVar(&todas, "todas", false, "entrega todas las tareas listas en un solo PR")
	cmd.Flags().StringVar(&titulo, "titulo", "", "título del PR conjunto (por defecto, el de la primera tarea)")
	cmd.Flags().BoolVar(&revisar, "revisar", false, "un modelo revisa el diff y deja el informe en el PR; apruebas tú")
	cmd.Flags().BoolVar(&integrar, "integrar", false, "revisa y, si no pide cambios, mergea el PR por rebase sin esperarte")
	return cmd
}

// runShipTodas entrega en un solo PR todas las tareas que quedaron
// listas. Cada una pasa su propia esclusa de salida antes de integrarse:
// el PR conjunto no baja el listón, solo evita repartirlo en N PRs que
// después hay que mergear en el orden correcto a mano.
func runShipTodas(dryRun bool, titulo string, integrar, revisar bool) error {
	root, err := projectRoot()
	if err != nil {
		return err
	}
	cfg, err := config.Load(root)
	if err != nil {
		return err
	}
	tareas, err := task.List(config.TasksDir(root))
	if err != nil {
		return err
	}

	var listas []task.Task
	var pendientes, detenidas []string
	modelos := map[string]string{}
	commits := map[string]string{}
	for _, t := range tareas {
		st, err := state.Get(root, t.ID)
		if err != nil {
			return err
		}
		switch st.Estado {
		case state.Lista:
			listas = append(listas, t)
			modelos[t.ID] = ultimoModelo(root, t.ID)
			commits[t.ID] = st.Commit
		case state.Detenida:
			detenidas = append(detenidas, t.ID)
		case state.Pendiente, state.EnCurso:
			pendientes = append(pendientes, t.ID)
		}
	}
	if len(listas) == 0 {
		return errors.New("ninguna tarea está lista · corre devclean run primero")
	}
	// entregar la mitad de un plan deja el PR incoherente: las tareas que
	// faltan son justo las que otras consumen
	if len(detenidas) > 0 || len(pendientes) > 0 {
		var partes []string
		if len(detenidas) > 0 {
			partes = append(partes, "detenidas: "+strings.Join(detenidas, ", ")+" (revívelas con devclean run --reintentar)")
		}
		if len(pendientes) > 0 {
			partes = append(partes, "sin correr: "+strings.Join(pendientes, ", "))
		}
		return fmt.Errorf("no todas las tareas están listas · %s", strings.Join(partes, " · "))
	}

	opciones := ship.OpcionesEntrega{
		Root:    root,
		Config:  cfg,
		Base:    cfg.Base,
		Tareas:  listas,
		Modelos: modelos,
		Commits: commits,
		Titulo:  titulo,
		DryRun:  dryRun,
		Progreso: func(p ship.Paso) {
			if p.OK {
				out.Line("✓ %s  · %s", p.Nombre, p.Detalle)
			} else {
				out.Line("✗ %s  · %s", p.Nombre, p.Detalle)
			}
		},
	}
	if cfg.TimeoutPruebas > 0 {
		opciones.Timeout = time.Duration(cfg.TimeoutPruebas) * time.Second
	}
	if integrar || revisar {
		if dryRun {
			return errors.New("--revisar e --integrar necesitan un PR donde dejar el informe · quita --dry-run")
		}
		rev, err := revisorDelProyecto(root, cfg)
		if err != nil {
			return err
		}
		opciones.Revisor, opciones.Integrar = rev, integrar
	}

	e := ship.EntregarTodas(context.Background(), opciones)
	if err := out.Data(e); err != nil {
		return err
	}
	if !e.Aprobado {
		return fmt.Errorf("entrega frenada · %s", e.PrimerMotivo())
	}
	if dryRun {
		out.Line("entregable · %d tareas en la rama %s · --dry-run, sin PR", len(listas), e.Rama)
		return nil
	}
	if e.Integrado {
		out.Line("integrado · %d tareas en %s · %s", len(listas), cfg.Base, e.PR)
		return nil
	}
	out.Line("entregado · %d tareas en un PR · %s", len(listas), e.PR)
	if opciones.Revisor != nil {
		out.Line("· el informe del revisor está en el PR · la aprobación es tuya")
	}
	return nil
}

// revisorAgente adapta el ejecutor al generador de texto del revisor,
// igual que hace el planificador.
type revisorAgente struct{ gen generadorPlan }

func (r revisorAgente) Revisar(ctx context.Context, diff string, tareas []task.Task) (bool, string, string, error) {
	v, err := revisor.Revisar(ctx, revisor.Opciones{
		Generador: r.gen,
		Tareas:    tareas,
		Diff:      diff,
	})
	if err != nil {
		return false, "", "", err
	}
	return v.Aprobado(), v.Informe(), v.Resumen(), nil
}

// revisorDelProyecto arma el revisor con el modelo del rol `revisor`, o
// el del planificador si no se declaró uno propio. El rol lleva
// declarado en config desde el principio y hasta ahora no lo usaba nadie.
func revisorDelProyecto(root string, cfg config.Config) (ship.Revisor, error) {
	ex, err := elegirEjecutor(cfg.Cli)
	if err != nil {
		return nil, err
	}
	modelo := config.ModeloRol(cfg, "revisor")
	if modelo == "" {
		modelo = config.ModeloRol(cfg, "planificador")
	}
	if modelo == "" {
		modelo = cfg.ModeloPeso("pesada") // revisar todo el diff no es tarea liviana
	}
	return revisorAgente{gen: generadorPlan{ex: ex, modelo: modelo, root: root}}, nil
}

func runShip(id string, dryRun bool) error {
	if err := validTaskID(id); err != nil {
		return err
	}
	root, err := projectRoot()
	if err != nil {
		return err
	}
	cfg, err := config.Load(root)
	if err != nil {
		return err
	}
	t, err := task.Load(config.TasksDir(root), id)
	if err != nil {
		return err
	}
	st, err := state.Get(root, id)
	if err != nil {
		return err
	}
	if st.Estado != state.Lista {
		return errorShipNoLista(id, st)
	}

	r := room.Room{ID: id, Path: filepath.Join(room.Dir(root), id), Rama: room.Branch(id)}
	if _, err := os.Stat(r.Path); err != nil {
		return fmt.Errorf("no existe el cuarto de %s · la tarea no corrió", id)
	}

	opciones := ship.Opciones{
		Root:   root,
		Room:   r,
		Task:   t,
		Config: cfg,
		Modelo: ultimoModelo(root, id),
		Base:   cfg.Base,
		DryRun: dryRun,
	}
	if cfg.TimeoutPruebas > 0 {
		opciones.Timeout = time.Duration(cfg.TimeoutPruebas) * time.Second
	}

	var res ship.Resultado
	if esTUI() {
		res, err = tui.CorrerGate(opciones)
		if err != nil {
			return err
		}
	} else {
		res = ship.Run(context.Background(), opciones)
	}

	// la entrega alimenta las métricas de ruido y roce, aunque frene
	if !dryRun {
		_ = metrics.GuardarEntrega(root, metrics.Entrega{
			ID:          id,
			Fecha:       time.Now().UTC(),
			LineasMas:   res.LineasMas,
			LineasMenos: res.LineasMenos,
			Ruido:       res.Ruido,
			Conflicto:   res.Conflicto,
			PR:          res.PR,
			Aprobado:    res.Aprobado,
			Brecha:      res.Brecha,
		})
	}

	if err := out.Data(res); err != nil {
		return err
	}
	if esTUI() {
		// la compuerta ya mostró los pasos; queda solo la línea final
		if !res.Aprobado {
			return fmt.Errorf("esclusa frenada · %s", res.PrimerMotivo())
		}
		if dryRun {
			out.Line("entregable · --dry-run, sin PR")
		} else {
			out.Line("entregado · %s", res.PR)
		}
		return nil
	}
	for _, p := range res.Pasos {
		if p.OK {
			if p.Detalle != "" {
				out.Line("✓ %s  · %s", p.Nombre, p.Detalle)
			} else {
				out.Line("✓ %s", p.Nombre)
			}
		} else {
			out.Line("✗ %s  · %s", p.Nombre, p.Detalle)
		}
	}
	if !res.Aprobado {
		return fmt.Errorf("esclusa frenada · %s", res.PrimerMotivo())
	}
	if dryRun {
		out.Line("entregable · --dry-run, sin PR")
	} else {
		out.Line("entregado · %s", res.PR)
	}
	return nil
}

func errorShipNoLista(id string, st state.State) error {
	switch st.Estado {
	case state.Detenida:
		razon := st.Pregunta
		if razon == "" {
			razon = st.UltimoError
		}
		if razon == "" {
			razon = "agotó intentos"
		}
		return fmt.Errorf("%s está detenida · %s · sube limite_intentos, revisa con devclean logs %s o edita la tarea", id, razon, id)
	case state.EnCurso:
		return fmt.Errorf("%s sigue en curso · espera a que termine o mira devclean board", id)
	case state.Pendiente:
		return fmt.Errorf("%s está pendiente · corre devclean run primero", id)
	default:
		return fmt.Errorf("%s no está lista (estado %s) · corre devclean run primero", id, st.Estado)
	}
}

// ultimoModelo devuelve el modelo del último intento, para el trailer
// Agent: del commit.
func ultimoModelo(root, id string) string {
	attempts, err := loop.ReadAttempts(root, id)
	if err != nil || len(attempts) == 0 {
		return ""
	}
	return attempts[len(attempts)-1].Modelo
}
