package main

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/Pastranauwu/devclean/internal/config"
	"github.com/Pastranauwu/devclean/internal/gate"
	"github.com/Pastranauwu/devclean/internal/state"
	"github.com/Pastranauwu/devclean/internal/task"
)

func newCheckCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "check <id>",
		Short: "corre la esclusa de entrada sobre una tarea",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCheck(args[0])
		},
	}
}

func newTaskCheckCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "check <id>",
		Short: "corre la esclusa de entrada sobre una tarea",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCheck(args[0])
		},
	}
}

func runCheck(id string) error {
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
	todas, err := task.List(config.TasksDir(root))
	if err != nil {
		return err
	}

	// el chequeo de cruce solo mira tareas activas (§6.3)
	var activas []task.Task
	for _, o := range todas {
		if o.ID == id {
			continue
		}
		s, err := state.Get(root, o.ID)
		if err != nil {
			return err
		}
		if s.Estado == state.EnCurso {
			activas = append(activas, o)
		}
	}

	timeout := gate.DefaultTimeout
	if cfg.TimeoutEsclusa > 0 {
		timeout = time.Duration(cfg.TimeoutEsclusa) * time.Second
	}
	res := gate.Run(context.Background(), root, cfg, t, activas, timeout)
	if err := out.Data(res); err != nil {
		return err
	}
	for _, c := range res.Chequeos {
		if c.OK {
			out.Line("✓ %s", c.Nombre)
		} else {
			out.Line("✗ %s · %s", c.Nombre, c.Motivo)
		}
	}
	if !res.Aprobada {
		return fmt.Errorf("tarea rechazada · %s", res.PrimerMotivo())
	}
	out.Line("tarea aceptada · lista para asignar")
	return nil
}
