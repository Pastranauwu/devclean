// Package tui es el modo interactivo de devclean (§16): la compuerta
// animada de la esclusa de salida. Cuando la salida es una terminal y no
// hay --plain ni --json, los comandos cambian a esta vista; si no, usan
// el texto plano de internal/ui.
package tui

import (
	"context"
	"strings"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Pastranauwu/devclean/internal/ship"
)

// NombresPasos es el orden fijo de la compuerta (§16.3): ocho pasos, de
// izquierda a derecha.
var NombresPasos = []string{"base", "historial", "ruido", "secretos", "presupuesto", "bisectable", "handoff", "pr"}

// nombresCortos son las abreviaturas que se pintan bajo los glifos; el
// nombre completo vive en ship.Paso.Nombre y en el JSON.
var nombresCortos = []string{"base", "hist", "ruido", "secr", "presu", "bisec", "hand", "pr"}

// paleta del cuarto limpio industrial (§16.2)
var (
	presion = lipgloss.Color("#4FB3A2")
	alerta  = lipgloss.Color("#D96C4A")
	espera  = lipgloss.Color("#C9A227")
	apagado = lipgloss.Color("#6B6F72")
	tinta   = lipgloss.Color("#E6E6E1")
)

// estadoPaso es el estado de un paso en la compuerta.
type estadoPaso int

const (
	pendiente estadoPaso = iota
	trabajando
	verde
	rojo
)

// clasificar devuelve el estado de cada uno de los ocho pasos dado lo que
// ya terminó. Mientras la compuerta corre, el paso siguiente al último
// hecho queda "trabajando".
func clasificar(pasos []ship.Paso, terminado bool) [8]estadoPaso {
	var e [8]estadoPaso
	for i := range e {
		e[i] = pendiente
	}
	for i, p := range pasos {
		if i >= len(e) {
			break
		}
		if p.OK {
			e[i] = verde
		} else {
			e[i] = rojo
		}
	}
	if !terminado && len(pasos) < len(e) {
		e[len(pasos)] = trabajando
	}
	return e
}

func glifo(s estadoPaso) string {
	switch s {
	case verde:
		return "✓"
	case rojo:
		return "✗"
	case trabajando:
		return "◐"
	default:
		return "·"
	}
}

// renderGate dibuja la compuerta en texto plano: los glifos de arriba,
// los nombres debajo y el detalle del último paso. Sin colores, para
// poder probarla; el modelo le pone color encima.
func renderGate(id string, pasos []ship.Paso, terminado bool, width int) string {
	e := clasificar(pasos, terminado)
	if width <= 0 {
		width = 80
	}
	ancho := width - 4
	if ancho < 20 {
		ancho = 20
	}

	var b strings.Builder
	titulo := "  ESCLUSA DE SALIDA · " + id
	b.WriteString("  ╭" + strings.Repeat("─", ancho) + "╮\n")
	b.WriteString("  │ " + titulo + strings.Repeat(" ", ancho-1-len(titulo)) + "│\n")
	b.WriteString("  ╰" + strings.Repeat("─", ancho) + "╯\n\n")

	var glifos, nombres strings.Builder
	for i := 0; i < len(e); i++ {
		glifos.WriteString(acomodar(glifo(e[i]), 8))
		nombres.WriteString(acomodar(nombresCortos[i], 8))
	}
	b.WriteString("   " + strings.TrimRight(glifos.String(), " ") + "\n")
	b.WriteString("   " + strings.TrimRight(nombres.String(), " ") + "\n")

	if len(pasos) > 0 {
		b.WriteString("\n  " + pasos[len(pasos)-1].Detalle + "\n")
	}
	return b.String()
}

// acomodar rellena s con espacios hasta el ancho n (en celdas).
func acomodar(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-len(s))
}

// CorrerGate corre la esclusa de salida dentro de la compuerta animada y
// devuelve el resultado cuando termina.
func CorrerGate(o ship.Opciones) (ship.Resultado, error) {
	ch := make(chan gateMsg)
	o.Progreso = func(p ship.Paso) { ch <- gateMsg{paso: p} }
	go func() {
		res := ship.Run(context.Background(), o)
		ch <- gateMsg{fin: true, res: res}
		close(ch)
	}()

	m := gateModel{opciones: o, ch: ch}
	final, err := tea.NewProgram(m).Run()
	if err != nil {
		return ship.Resultado{}, err
	}
	return final.(gateModel).resultado, nil
}

// gateMsg es un evento de la compuerta: un paso que terminó, o el final.
type gateMsg struct {
	paso ship.Paso
	fin  bool
	res  ship.Resultado
}

type gateModel struct {
	opciones  ship.Opciones
	ch        chan gateMsg
	pasos     []ship.Paso
	terminado bool
	resultado ship.Resultado
}

func (m gateModel) Init() tea.Cmd {
	return m.escuchar()
}

func (m gateModel) escuchar() tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-m.ch
		if !ok {
			return tea.Quit()
		}
		return msg
	}
}

func (m gateModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	case gateMsg:
		if msg.fin {
			m.terminado = true
			m.resultado = msg.res
			return m, tea.Quit
		}
		m.pasos = append(m.pasos, msg.paso)
		return m, m.escuchar()
	}
	return m, nil
}

func (m gateModel) View() string {
	s := renderGate(m.opciones.Task.ID, m.pasos, m.terminado, 80)
	return conColor(m.pasos, m.terminado, s)
}

// conColor aplica la paleta §16.2 a la vista en texto plano.
func conColor(pasos []ship.Paso, terminado bool, s string) string {
	e := clasificar(pasos, terminado)
	colores := map[string]lipgloss.Style{}
	colores["·"] = lipgloss.NewStyle().Foreground(apagado)
	colores["◐"] = lipgloss.NewStyle().Foreground(espera)
	colores["✓"] = lipgloss.NewStyle().Foreground(presion)
	colores["✗"] = lipgloss.NewStyle().Foreground(alerta)

	var b strings.Builder
	for _, r := range s {
		ch := string(r)
		if st, ok := colores[ch]; ok {
			b.WriteString(st.Render(ch))
		} else {
			b.WriteString(ch)
		}
	}
	_ = e
	_ = tinta
	return b.String()
}
