package main

import (
	"sort"

	"github.com/spf13/cobra"

	"github.com/Pastranauwu/devclean/internal/config"
	"github.com/Pastranauwu/devclean/internal/state"
	"github.com/Pastranauwu/devclean/internal/task"
	"github.com/Pastranauwu/devclean/internal/tui"
)

type boardRow struct {
	ID     string `json:"id"`
	Titulo string `json:"titulo"`
	Estado string `json:"estado"`
}

func newBoardCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "board",
		Short: "tablero de estado",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBoard()
		},
	}
}

func runBoard() error {
	root, err := projectRoot()
	if err != nil {
		return err
	}
	if esTUI() {
		id, err := tui.CorrerBoard(root)
		if err != nil {
			return err
		}
		if id == "" {
			return nil
		}
		return runShip(id, true)
	}

	tasks, err := task.List(config.TasksDir(root))
	if err != nil {
		return err
	}

	var lista, enCurso, detenidas, pendientes []boardRow
	for _, t := range tasks {
		s, err := state.Get(root, t.ID)
		if err != nil {
			return err
		}
		row := boardRow{ID: t.ID, Titulo: t.Titulo, Estado: s.Estado}
		switch s.Estado {
		case state.Lista:
			lista = append(lista, row)
		case state.EnCurso:
			enCurso = append(enCurso, row)
		case state.Detenida:
			detenidas = append(detenidas, row)
		default:
			pendientes = append(pendientes, row)
		}
	}
	sort.Slice(lista, byID(lista))
	sort.Slice(enCurso, byID(enCurso))
	sort.Slice(detenidas, byID(detenidas))
	sort.Slice(pendientes, byID(pendientes))

	// un solo Data para el modo JSON
	todo := map[string][]boardRow{
		"listo":     lista,
		"en_curso":  enCurso,
		"detenido":  detenidas,
		"pendiente": pendientes,
	}
	if err := out.Data(todo); err != nil {
		return err
	}

	if len(tasks) == 0 {
		out.Line("sin tareas · empieza con devclean task add \"lo que necesitas\"")
		return nil
	}
	imprimirGrupo("LISTO PARA ENTREGAR", lista)
	imprimirGrupo("EN CURSO", enCurso)
	imprimirGrupo("DETENIDO", detenidas)
	imprimirGrupo("PENDIENTE", pendientes)
	return nil
}

func byID(rows []boardRow) func(i, j int) bool {
	return func(i, j int) bool { return rows[i].ID < rows[j].ID }
}

func imprimirGrupo(nombre string, rows []boardRow) {
	if len(rows) == 0 {
		out.Line("%-20s —", nombre)
		return
	}
	for _, r := range rows {
		out.Line("%-20s %s  %s", nombre, r.ID, r.Titulo)
		nombre = ""
	}
}
