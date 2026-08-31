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

func cursorInicial(filas []Fila) int {
	for i, f := range filas {
		if f.Estado == state.Lista {
			return i
		}
	}
	return 0
}

func selectedID(filas []Fila, cursor int) string {
	if cursor < 0 || cursor >= len(filas) {
		return ""
	}
	return filas[cursor].ID
}

// lineasTablero arma el contenido del sticker: logo, tagline y columnas.
func lineasTablero(filas []Fila) []lineaSticker {
	return armarTablero(filas, "", "")
}

func armarTablero(filas []Fila, sel, aviso string) []lineaSticker {
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
			marca, color := "  ", rgbTinta
			if f.ID == sel {
				marca, color = "> ", rgbPresion
			}
			ls = append(ls, lineaSticker{texto: marca + f.ID + "  " + f.Titulo, color: color})
		}
		ls = append(ls, lineaSticker{})
	}
	if aviso != "" {
		ls = append(ls, lineaSticker{texto: aviso, color: rgbAlerta})
	}
	ls = append(ls, lineaSticker{texto: "s dry-run · j/k mueve · q sale", color: rgbApagado})
	return ls
}

// CorrerBoard muestra el tablero sobre un plasma animado; sale con q o esc.
// Si el usuario pulsa s sobre una tarea lista, devuelve su id para que
// el comando dispare ship --dry-run en la misma terminal.
func CorrerBoard(root string) (string, error) {
	filas, err := Tablero(root)
	if err != nil {
		return "", err
	}
	m := boardModel{filas: filas, cursor: cursorInicial(filas), params: DefaultPlasma()}
	final, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
	if err != nil {
		return "", err
	}
	return final.(boardModel).shipID, nil
}

type boardModel struct {
	filas  []Fila
	cursor int
	shipID string
	aviso  string
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
		case "j", "down":
			if n := len(m.filas); n > 0 {
				m.cursor = (m.cursor + 1) % n
				m.aviso = ""
			}
			return m, nil
		case "k", "up":
			if n := len(m.filas); n > 0 {
				m.cursor = (m.cursor - 1 + n) % n
				m.aviso = ""
			}
			return m, nil
		case "s":
			if m.cursor < 0 || m.cursor >= len(m.filas) {
				return m, nil
			}
			f := m.filas[m.cursor]
			if f.Estado != state.Lista {
				m.aviso = f.ID + " no está lista · s solo entrega las que están en LISTO"
				return m, nil
			}
			m.shipID = f.ID
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m boardModel) View() string {
	return FondoPlasma(m.ancho, m.alto, m.t, m.params, armarTablero(m.filas, selectedID(m.filas, m.cursor), m.aviso), 2)
}
