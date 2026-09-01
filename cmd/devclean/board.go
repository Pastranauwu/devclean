package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Pastranauwu/devclean/internal/config"
	"github.com/Pastranauwu/devclean/internal/recurse"
	"github.com/Pastranauwu/devclean/internal/state"
	"github.com/Pastranauwu/devclean/internal/task"
	"github.com/Pastranauwu/devclean/internal/tui"
)

type boardRow struct {
	ID     string     `json:"id"`
	Titulo string     `json:"titulo"`
	Estado string     `json:"estado"`
	Hijos  []boardRow `json:"hijos,omitempty"`
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
		accion, err := tui.CorrerBoard(root)
		if err != nil {
			return err
		}
		// el tablero solo decide; lo que corre lo corre el comando, para
		// que la salida sea la misma que si se hubiera tecleado
		switch accion.Tipo {
		case tui.AccionEntregar:
			return runShip(accion.ID, true)
		case tui.AccionReintentar:
			return reintentarTarea(root, accion.ID)
		case tui.AccionDetalle:
			return runLogs(accion.ID)
		}
		return nil
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
		nodos, err := recurse.LeerArbol(root, t.ID)
		if err != nil {
			return err
		}
		row := boardRow{ID: t.ID, Titulo: t.Titulo, Estado: s.Estado, Hijos: hijosBoardRow(nodos, t.ID)}
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

// hijosBoardRow arma recursivamente el árbol de un padre a partir de los
// nodos planos de arbol.json (§8.3).
func hijosBoardRow(nodos []recurse.NodoArbol, padre string) []boardRow {
	var hijos []boardRow
	for _, n := range nodos {
		if n.Padre != padre {
			continue
		}
		estado := state.Detenida
		if n.Verde {
			estado = state.Lista
		}
		hijos = append(hijos, boardRow{ID: n.ID, Titulo: n.Titulo, Estado: estado, Hijos: hijosBoardRow(nodos, n.ID)})
	}
	sort.Slice(hijos, byID(hijos))
	return hijos
}

func imprimirGrupo(nombre string, rows []boardRow) {
	if len(rows) == 0 {
		out.Line("%-20s —", nombre)
		return
	}
	for _, r := range rows {
		out.Line("%-20s %s  %s", nombre, r.ID, r.Titulo)
		nombre = ""
		imprimirHijos(r.Hijos, 1)
	}
}

// imprimirHijos imprime el árbol de subtareas indentado debajo de su
// padre, recursivo (una subtarea que a su vez recursó también sale
// anidada).
func imprimirHijos(rows []boardRow, profundidad int) {
	sangria := strings.Repeat("  ", profundidad)
	for _, r := range rows {
		out.Line("%-20s %s└ %s  %s", "", sangria, r.ID, r.Titulo)
		imprimirHijos(r.Hijos, profundidad+1)
	}
}

// reintentarTarea vuelve a correr una sola tarea detenida, reusando su
// cuarto y el trabajo parcial que quedó dentro. Es la tecla r del
// tablero: sin esto, revivir una tarea obligaba a correr `run
// --reintentar` sobre todas.
func reintentarTarea(root, id string) error {
	st, err := state.Get(root, id)
	if err != nil {
		return err
	}
	if st.Estado != state.Detenida {
		return fmt.Errorf("%s no está detenida (estado %s) · nada que reintentar", id, st.Estado)
	}
	// volver a pendiente es lo que hace que `run` la recoja; el cuarto y
	// su rama siguen ahí, así que el agente arranca viendo lo que ya
	// escribió en vez de rehacerlo y pagar los mismos tokens otra vez
	if err := state.Save(root, state.State{ID: id, Estado: state.Pendiente, Rama: st.Rama, Puerto: st.Puerto}); err != nil {
		return err
	}
	out.Line("· %s vuelve a la cola · se reusa su cuarto y su trabajo parcial", id)
	return runCmd(1, "", "", true)
}
