package tui

import (
	"sort"
	"strings"

	"github.com/charmbracelet/bubbletea"

	"github.com/Pastranauwu/devclean/internal/config"
	"github.com/Pastranauwu/devclean/internal/state"
	"github.com/Pastranauwu/devclean/internal/task"
)

// Fila es una tarea del tablero.
type Fila struct {
	ID     string
	Titulo string
	Estado string
}

// Tablero lee las tareas y sus estados del disco.
func Tablero(root string) ([]Fila, error) {
	tasks, err := task.List(config.TasksDir(root))
	if err != nil {
		return nil, err
	}
	filas := make([]Fila, 0, len(tasks))
	for _, t := range tasks {
		s, err := state.Get(root, t.ID)
		if err != nil {
			return nil, err
		}
		filas = append(filas, Fila{ID: t.ID, Titulo: t.Titulo, Estado: s.Estado})
	}
	sort.Slice(filas, func(i, j int) bool { return filas[i].ID < filas[j].ID })
	return filas, nil
}

// renderBoard dibuja el tablero con la paleta y el logotipo, en tarjeta.
func renderBoard(filas []Fila, width int) string {
	var cuerpo strings.Builder

	if len(filas) == 0 {
		cuerpo.WriteString(estiloApagado.Render("sin tareas · empieza con devclean plan \"lo que necesitas\"") + "\n")
		return Logo(width) + "\n" + caja(cuerpo.String())
	}

	orden := []struct {
		nombre string
		estado string
		glifo  string
	}{
		{"LISTO PARA ENTREGAR", state.Lista, estiloPresion.Render("✓")},
		{"EN CURSO", state.EnCurso, estiloEspera.Render("◐")},
		{"DETENIDO", state.Detenida, estiloAlerta.Render("⏸")},
		{"PENDIENTE", state.Pendiente, estiloApagado.Render("·")},
	}
	for _, col := range orden {
		var suyos []Fila
		for _, f := range filas {
			if f.Estado == col.estado {
				suyos = append(suyos, f)
			}
		}
		cuerpo.WriteString(col.glifo + " " + estiloBold.Render(col.nombre) + "\n")
		if len(suyos) == 0 {
			cuerpo.WriteString("  " + estiloApagado.Render("—") + "\n\n")
			continue
		}
		for _, f := range suyos {
			cuerpo.WriteString("  " + estiloTinta.Render(f.ID) + "  " + f.Titulo + "\n")
		}
		cuerpo.WriteString("\n")
	}
	return Logo(width) + "\n" + caja(strings.TrimRight(cuerpo.String(), "\n"))
}

// CorrerBoard muestra el tablero en modo interactivo; sale con q o esc.
func CorrerBoard(root string) error {
	filas, err := Tablero(root)
	if err != nil {
		return err
	}
	m := boardModel{filas: filas}
	_, err = tea.NewProgram(m).Run()
	return err
}

type boardModel struct {
	filas []Fila
}

func (m boardModel) Init() tea.Cmd { return nil }

func (m boardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m boardModel) View() string {
	return renderBoard(m.filas, 80)
}
