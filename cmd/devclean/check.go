package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Pastranauwu/devclean/internal/config"
	"github.com/Pastranauwu/devclean/internal/gate"
	"github.com/Pastranauwu/devclean/internal/task"
)

func newTaskCheckCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "check <id>",
		Short: "corre la esclusa de entrada sobre una tarea",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
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

			res := gate.Run(context.Background(), root, cfg, t, todas, gate.DefaultTimeout)
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
		},
	}
}
