package main

import (
	"time"

	"github.com/spf13/cobra"

	"github.com/Pastranauwu/devclean/internal/config"
	"github.com/Pastranauwu/devclean/internal/loop"
	"github.com/Pastranauwu/devclean/internal/standup"
	"github.com/Pastranauwu/devclean/internal/state"
	"github.com/Pastranauwu/devclean/internal/task"
)

func newStandupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "standup",
		Short: "parte de datos de las tareas en curso (§6.7)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStandup()
		},
	}
}

func runStandup() error {
	root, err := projectRoot()
	if err != nil {
		return err
	}
	tareas, err := task.List(config.TasksDir(root))
	if err != nil {
		return err
	}
	estados, err := state.List(root)
	if err != nil {
		return err
	}
	estadosPorID := make(map[string]state.State, len(estados))
	var nActivas int
	for _, s := range estados {
		estadosPorID[s.ID] = s
		if s.Estado == state.EnCurso {
			nActivas++
		}
	}
	attempts := make(map[string][]loop.Attempt, len(tareas))
	for _, t := range tareas {
		as, err := loop.ReadAttempts(root, t.ID)
		if err != nil {
			return err
		}
		attempts[t.ID] = as
	}

	eventos := standup.Analizar(tareas, estadosPorID, attempts)
	if err := out.Data(eventos); err != nil {
		return err
	}
	out.Line("%s", standup.Formatear(eventos, time.Now(), nActivas))
	return nil
}
