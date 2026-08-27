package tui

import (
	"github.com/charmbracelet/bubbletea"
)

type esperarMsg struct{ err error }

type esperarModel struct {
	titulo string
	ch     chan error
	tick   int
	err    error
	fin    bool
}

// Esperar muestra un spinner con `titulo` mientras `trabajo` corre en
// segundo plano. No es una barra de progreso inventada (§16.2): no se
// sabe cuánto falta, así que solo gira. Devuelve el error de `trabajo`.
func Esperar(titulo string, trabajo func() error) error {
	ch := make(chan error, 1)
	go func() { ch <- trabajo() }()

	m := esperarModel{titulo: titulo, ch: ch}
	res, err := tea.NewProgram(m).Run()
	if err != nil {
		return err
	}
	return res.(esperarModel).err
}

func (m esperarModel) Init() tea.Cmd {
	return tea.Batch(m.escuchar(), tickCmd())
}

func (m esperarModel) escuchar() tea.Cmd {
	return func() tea.Msg { return esperarMsg{err: <-m.ch} }
}

func (m esperarModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tickMsg:
		if m.fin {
			return m, nil
		}
		m.tick++
		return m, tickCmd()
	case esperarMsg:
		m.fin = true
		m.err = msg.err
		return m, tea.Quit
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			m.err = errInterrumpido
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m esperarModel) View() string {
	g := estiloEspera.Render(spinnerFrames[m.tick%len(spinnerFrames)])
	return Logo(80) + "\n" + caja(g+" "+estiloTinta.Render(m.titulo))
}

var errInterrumpido = errInterrumpidoT{}

type errInterrumpidoT struct{}

func (errInterrumpidoT) Error() string { return "interrumpido" }
