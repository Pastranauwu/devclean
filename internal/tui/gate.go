// Package tui es el modo interactivo de devclean (§16): la compuerta
// animada de la esclusa de salida, el tablero y la corrida en vivo. Cuando
// la salida es una terminal y no hay --plain ni --json, los comandos usan
// esta vista; si no, el texto plano de internal/ui.
package tui

import (
	"context"
	"strconv"
	"strings"
	"time"

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

func renderGlifo(e estadoPaso, tick int) string {
	switch e {
	case verde:
		return estiloPresion.Render("✓")
	case rojo:
		return estiloAlerta.Render("✗")
	case trabajando:
		return estiloEspera.Render(spinnerFrames[tick%len(spinnerFrames)])
	default:
		return estiloApagado.Render("·")
	}
}

func renderNombre(e estadoPaso, nombre string) string {
	switch e {
	case verde:
		return estiloPresion.Render(nombre)
	case rojo:
		return estiloAlerta.Render(nombre)
	case trabajando:
		return estiloEspera.Render(nombre)
	default:
		return estiloApagado.Render(nombre)
	}
}

// renderGate dibuja la compuerta con colores, spinner y barra de progreso.
func renderGate(id string, pasos []ship.Paso, terminado bool, tick, width int) string {
	e := clasificar(pasos, terminado)
	if width <= 0 {
		width = 80
	}
	ancho := width - 4

	var b strings.Builder
	b.WriteString(Logo(width))
	b.WriteString("\n  ")
	b.WriteString(estiloBold.Render("ESCLUSA DE SALIDA · " + id))
	b.WriteString("\n\n")

	var glifos, nombres strings.Builder
	for i := 0; i < len(e); i++ {
		glifos.WriteString(acomodar(renderGlifo(e[i], tick), 8))
		nombres.WriteString(acomodar(renderNombre(e[i], nombresCortos[i]), 8))
	}
	b.WriteString("   " + strings.TrimRight(glifos.String(), " ") + "\n")
	b.WriteString("   " + strings.TrimRight(nombres.String(), " ") + "\n")

	b.WriteString("\n  " + barra(len(pasos), 8, ancho) + " " + estiloApagado.Render(strconv.Itoa(len(pasos))+"/8") + "\n")

	if len(pasos) > 0 {
		b.WriteString("\n  " + estiloApagado.Render(pasos[len(pasos)-1].Detalle) + "\n")
	}
	return b.String()
}

// acomodar rellena s con espacios hasta el ancho visible n (celdas).
func acomodar(s string, n int) string {
	w := lipgloss.Width(s)
	if w >= n {
		return s
	}
	return s + strings.Repeat(" ", n-w)
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

type tickMsg struct{}

type gateModel struct {
	opciones  ship.Opciones
	ch        chan gateMsg
	pasos     []ship.Paso
	terminado bool
	resultado ship.Resultado
	tick      int
}

func (m gateModel) Init() tea.Cmd {
	return tea.Batch(m.escuchar(), tickCmd())
}

func tickCmd() tea.Cmd {
	return tea.Tick(80*time.Millisecond, func(time.Time) tea.Msg { return tickMsg{} })
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
	case tickMsg:
		if m.terminado {
			return m, nil
		}
		m.tick++
		return m, tickCmd()
	case tea.KeyMsg:
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	case gateMsg:
		if msg.fin {
			m.terminado = true
			m.resultado = msg.res
			// deja ver la compuerta en verde un instante antes de salir
			return m, tea.Tick(800*time.Millisecond, func(time.Time) tea.Msg { return tea.Quit() })
		}
		m.pasos = append(m.pasos, msg.paso)
		return m, m.escuchar()
	}
	return m, nil
}

func (m gateModel) View() string {
	return renderGate(m.opciones.Task.ID, m.pasos, m.terminado, m.tick, 80)
}
