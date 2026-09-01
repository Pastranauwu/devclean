package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/Pastranauwu/devclean/internal/config"
	"github.com/Pastranauwu/devclean/internal/loop"
	"github.com/Pastranauwu/devclean/internal/metrics"
	"github.com/Pastranauwu/devclean/internal/room"
	"github.com/Pastranauwu/devclean/internal/ship"
	"github.com/Pastranauwu/devclean/internal/state"
	"github.com/Pastranauwu/devclean/internal/task"
	"github.com/Pastranauwu/devclean/internal/tui"
)

func newShipCmd() *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "ship <id>",
		Short: "esclusa de salida y PR",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runShip(args[0], dryRun)
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "corre la esclusa sin abrir el PR")
	return cmd
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
