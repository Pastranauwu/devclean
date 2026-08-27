package tui

import (
	"sort"

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

// lineasTablero arma el contenido del sticker: logo, tagline y columnas.
func lineasTablero(filas []Fila) []lineaSticker {
	var ls []lineaSticker
	alto := len(logoFilas)
	for i, fila := range logoFilas {
		c := mezclarRGB([3]int{79, 179, 162}, [3]int{44, 110, 99}, float64(i)/float64(alto-1))
		ls = append(ls, lineaSticker{texto: fila, color: c})
	}
	ls = append(ls, lineaSticker{})
	ls = append(ls, lineaSticker{texto: "dirige agentes · entrega código limpio", color: rgbApagado})
	ls = append(ls, lineaSticker{})

	if len(filas) == 0 {
		ls = append(ls, lineaSticker{texto: "sin tareas · empieza con devclean plan \"lo que necesitas\"", color: rgbApagado})
		return ls
	}

	orden := []struct {
		nombre string
		estado string
		glifo  string
		color  [3]int
	}{
		{"LISTO PARA ENTREGAR", state.Lista, "✓", rgbPresion},
		{"EN CURSO", state.EnCurso, "◐", rgbEspera},
		{"DETENIDO", state.Detenida, "⏸", rgbAlerta},
		{"PENDIENTE", state.Pendiente, "·", rgbApagado},
	}
	for _, col := range orden {
		var suyos []Fila
		for _, f := range filas {
			if f.Estado == col.estado {
				suyos = append(suyos, f)
			}
		}
		ls = append(ls, lineaSticker{texto: col.glifo + " " + col.nombre, color: col.color})
		if len(suyos) == 0 {
			ls = append(ls, lineaSticker{texto: "  —", color: rgbApagado})
		}
		for _, f := range suyos {
			ls = append(ls, lineaSticker{texto: "  " + f.ID + "  " + f.Titulo, color: rgbTinta})
		}
		ls = append(ls, lineaSticker{})
	}
	return ls
}

// CorrerBoard muestra el tablero sobre un plasma animado; sale con q o esc.
func CorrerBoard(root string) error {
	filas, err := Tablero(root)
	if err != nil {
		return err
	}
	m := boardModel{filas: filas, params: DefaultPlasma()}
	_, err = tea.NewProgram(m, tea.WithAltScreen()).Run()
	return err
}

type boardModel struct {
	filas  []Fila
	ancho  int
	alto   int
	t      float64
	params PlasmaParams
}

func (m boardModel) Init() tea.Cmd {
	return tickPlasma()
}

func (m boardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.ancho = msg.Width
		m.alto = msg.Height
		return m, nil
	case tickMsg:
		m.t += 0.1
		return m, tickPlasma()
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m boardModel) View() string {
	return FondoPlasma(m.ancho, m.alto, m.t, m.params, lineasTablero(m.filas), 2)
}
