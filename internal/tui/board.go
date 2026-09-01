package tui

import (
	"sort"

	"github.com/charmbracelet/bubbletea"

	"github.com/Pastranauwu/devclean/internal/config"
	"github.com/Pastranauwu/devclean/internal/loop"
	"github.com/Pastranauwu/devclean/internal/recurse"
	"github.com/Pastranauwu/devclean/internal/standup"
	"github.com/Pastranauwu/devclean/internal/state"
	"github.com/Pastranauwu/devclean/internal/task"
)

// Fila es una tarea del tablero, con sus subtareas si viene de una tarea
// recursiva (§8.3) — Hijos queda vacío en el caso normal.
type Fila struct {
	ID     string
	Titulo string
	Estado string
	Hijos  []Fila

	// Detalle es lo que la tarea está haciendo ahora mismo (intento,
	// fase, modelo, tiempo en fase). Vacío si no está corriendo.
	Detalle string
	// Atascada marca que la fase actual lleva más de standup.UmbralAtasco
	// sin moverse. No la mata: solo avisa.
	Atascada bool
}

// Tablero lee las tareas y sus estados del disco, con el árbol de
// subtareas de cada una que haya recursado (internal/recurse).
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
		nodos, err := recurse.LeerArbol(root, t.ID)
		if err != nil {
			return nil, err
		}
		f := Fila{ID: t.ID, Titulo: t.Titulo, Estado: s.Estado, Hijos: hijosDe(nodos, t.ID)}
		// el latido es lo único que sabe qué pasa DENTRO de un intento:
		// attempts.jsonl no se escribe hasta que el intento termina
		if l, corriendo := loop.LeerLatido(root, t.ID); corriendo {
			f.Detalle = l.Descripcion() + " · " + reloj(l.EnFaseDesde())
			if l.EnFaseDesde() >= standup.UmbralAtasco {
				f.Atascada = true
				f.Detalle = "ATASCO · " + f.Detalle + " sin señal"
			}
		}
		filas = append(filas, f)
	}
	sort.Slice(filas, func(i, j int) bool { return filas[i].ID < filas[j].ID })
	return filas, nil
}

// hijosDe arma recursivamente el árbol de un padre a partir de los nodos
// planos guardados en arbol.json.
func hijosDe(nodos []recurse.NodoArbol, padre string) []Fila {
	var hijos []Fila
	for _, n := range nodos {
		if n.Padre != padre {
			continue
		}
		estado := state.Detenida
		if n.Verde {
			estado = state.Lista
		}
		hijos = append(hijos, Fila{ID: n.ID, Titulo: n.Titulo, Estado: estado, Hijos: hijosDe(nodos, n.ID)})
	}
	sort.Slice(hijos, func(i, j int) bool { return hijos[i].ID < hijos[j].ID })
	return hijos
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

// filaConHijos arma la línea de una fila y, debajo, su árbol de
// subtareas indentado (§8.3) — recursivo, así que una subtarea que a su
// vez recursó también se ve anidada.
func filaConHijos(f Fila, profundidad int, sel string) []lineaSticker {
	marca, color := "  ", rgbTinta
	if f.ID == sel {
		marca, color = "> ", rgbPresion
	}
	sangria := ""
	for i := 0; i < profundidad; i++ {
		sangria += "  "
	}
	glifo := ""
	if profundidad > 0 {
		glifo = "└ "
		if f.Estado == state.Detenida {
			color = rgbAlerta
		} else {
			color = rgbApagado
		}
	}
	texto := marca + sangria + glifo + f.ID + "  " + f.Titulo
	ls := []lineaSticker{{texto: texto, color: color}}
	if f.Detalle != "" {
		c := rgbApagado
		if f.Atascada {
			c = rgbAlerta
		}
		ls = append(ls, lineaSticker{texto: "    " + sangria + f.Detalle, color: c})
	}
	for _, h := range f.Hijos {
		ls = append(ls, filaConHijos(h, profundidad+1, sel)...)
	}
	return ls
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
			ls = append(ls, filaConHijos(f, 0, sel)...)
		}
		ls = append(ls, lineaSticker{})
	}
	if aviso != "" {
		ls = append(ls, lineaSticker{texto: aviso, color: rgbAlerta})
	}
	ls = append(ls, lineaSticker{texto: "s dry-run · j/k mueve · q sale · se refresca solo", color: rgbApagado})
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
	m := boardModel{filas: filas, cursor: cursorInicial(filas), params: DefaultPlasma(), root: root}
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

	// root y ticks sirven al refresco: el tablero releía disco una sola
	// vez al abrirse, así que una corrida en paralelo avanzaba entera sin
	// que se moviera nada en pantalla.
	root  string
	ticks int
}

// ticksPorRefresco: el plasma late cada 100 ms; releer disco a ese ritmo
// es desperdicio, una vez por segundo alcanza para que se sienta vivo.
const ticksPorRefresco = 10

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
		m.ticks++
		if m.ticks%ticksPorRefresco == 0 {
			m.refrescar()
		}
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

// refrescar relee el estado de disco sin perder la selección: el cursor
// se sigue por id, no por posición, porque una tarea puede cambiar de
// columna entre dos refrescos.
func (m *boardModel) refrescar() {
	sel := selectedID(m.filas, m.cursor)
	filas, err := Tablero(m.root)
	if err != nil {
		return // un fallo de lectura no debe tumbar el tablero
	}
	m.filas = filas
	for i, f := range filas {
		if f.ID == sel {
			m.cursor = i
			return
		}
	}
	if m.cursor >= len(filas) {
		m.cursor = 0
	}
}

func (m boardModel) View() string {
	return FondoPlasma(m.ancho, m.alto, m.t, m.params, armarTablero(m.filas, selectedID(m.filas, m.cursor), m.aviso), 2)
}
