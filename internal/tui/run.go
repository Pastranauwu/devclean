package tui

import (
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbletea"
)

// FilaRun es una tarea que va a correr, mostrada en el tablero en vivo.
type FilaRun struct {
	ID     string
	Titulo string
	Limite int
}

// EventoRun es un cambio de estado de una tarea en curso.
type EventoRun struct {
	ID       string
	Estado   string // trabajando | lista | detenida
	Intentos int
	Motivo   string
}

type tareaViva struct {
	estado   string
	intentos int
	motivo   string
}

type runMsg struct {
	evento *EventoRun
	fin    bool
}

type runModel struct {
	filas  []FilaRun
	ch     chan runMsg
	estado map[string]*tareaViva
	inicio map[string]time.Time
	tick   int
	fin    bool
}

// CorrerRun muestra la corrida en vivo mientras `lanzar` ejecuta las
// tareas e informa cada cambio por `emit`.
func CorrerRun(filas []FilaRun, lanzar func(emit func(EventoRun))) error {
	ch := make(chan runMsg)
	go func() {
		lanzar(func(e EventoRun) { ch <- runMsg{evento: &e} })
		ch <- runMsg{fin: true}
		close(ch)
	}()

	m := runModel{
		filas:  filas,
		ch:     ch,
		estado: map[string]*tareaViva{},
		inicio: map[string]time.Time{},
	}
	_, err := tea.NewProgram(m).Run()
	return err
}

func (m runModel) Init() tea.Cmd {
	return tea.Batch(m.escuchar(), tickCmd())
}

func (m runModel) escuchar() tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-m.ch
		if !ok {
			return tea.Quit()
		}
		return msg
	}
}

func (m runModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tickMsg:
		if m.fin {
			return m, nil
		}
		m.tick++
		return m, tickCmd()
	case tea.KeyMsg:
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	case runMsg:
		if msg.fin {
			m.fin = true
			return m, tea.Tick(700*time.Millisecond, func(time.Time) tea.Msg { return tea.Quit() })
		}
		e := msg.evento
		v := m.estado[e.ID]
		if v == nil {
			v = &tareaViva{}
			m.estado[e.ID] = v
		}
		if e.Estado == "trabajando" {
			m.inicio[e.ID] = time.Now()
		}
		v.estado = e.Estado
		v.intentos = e.Intentos
		v.motivo = e.Motivo
		return m, m.escuchar()
	}
	return m, nil
}

func (m runModel) View() string {
	var cuerpo strings.Builder

	hechos := 0
	for _, f := range m.filas {
		if v := m.estado[f.ID]; v != nil && v.estado != "trabajando" {
			hechos++
		}
	}
	total := len(m.filas)
	if total == 0 {
		total = 1
	}
	cuerpo.WriteString(estiloTitulo.Render("CORRIDA") + "\n")
	cuerpo.WriteString(barra(hechos, total, 40) + " " +
		estiloApagado.Render(strconv.Itoa(hechos)+"/"+strconv.Itoa(len(m.filas))) + "\n\n")

	for _, f := range m.filas {
		cuerpo.WriteString(renderFilaRun(f, m.estado[f.ID], m.inicio[f.ID], m.tick))
	}

	return Logo(80) + "\n" + caja(strings.TrimRight(cuerpo.String(), "\n"))
}

func renderFilaRun(f FilaRun, v *tareaViva, inicio time.Time, tick int) string {
	var g, estado string
	switch {
	case v == nil:
		g = estiloApagado.Render("·")
		estado = estiloApagado.Render("pendiente")
	case v.estado == "lista":
		g = estiloPresion.Render("✓")
		estado = estiloPresion.Render("verde en " + strconv.Itoa(v.intentos) + " intentos")
	case v.estado == "detenida":
		g = estiloAlerta.Render("⏸")
		estado = estiloAlerta.Render("detenida")
	default:
		g = estiloEspera.Render(spinnerFrames[tick%len(spinnerFrames)])
		estado = estiloApagado.Render(reloj(time.Since(inicio)))
	}
	return "  " + g + " " + estiloTinta.Render(f.ID) + "  " + f.Titulo + "  " + estado + "\n"
}
