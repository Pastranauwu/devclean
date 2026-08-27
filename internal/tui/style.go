package tui

import (
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// paleta del cuarto limpio industrial (§16.2)
var (
	presion = lipgloss.Color("#4FB3A2")
	alerta  = lipgloss.Color("#D96C4A")
	espera  = lipgloss.Color("#C9A227")
	apagado = lipgloss.Color("#6B6F72")
	tinta   = lipgloss.Color("#E6E6E1")
)

var (
	estiloPresion = lipgloss.NewStyle().Foreground(presion)
	estiloAlerta  = lipgloss.NewStyle().Foreground(alerta)
	estiloEspera  = lipgloss.NewStyle().Foreground(espera)
	estiloApagado = lipgloss.NewStyle().Foreground(apagado)
	estiloTinta   = lipgloss.NewStyle().Foreground(tinta)
	estiloBold    = lipgloss.NewStyle().Foreground(tinta).Bold(true)
)

// spinnerFrames es la animación del trabajo en curso (braille).
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// Logo devuelve el logotipo de devclean, con borde en presion.
func Logo(ancho int) string {
	if ancho <= 0 || ancho > 120 {
		ancho = 80
	}
	linea := func(txt string) string {
		pad := ancho - 4 - lipgloss.Width(txt)
		if pad < 0 {
			txt = lipgloss.NewStyle().MaxWidth(ancho - 4).Render(txt)
			pad = 0
		}
		return " █ " + txt + strings.Repeat(" ", pad) + " █"
	}
	var b strings.Builder
	b.WriteString("  " + estiloPresion.Render(strings.Repeat("▄", ancho-4)) + "\n")
	b.WriteString(estiloPresion.Render(linea("◉  devclean")) + "\n")
	b.WriteString(estiloPresion.Render(linea("dirige agentes · entrega código limpio")) + "\n")
	b.WriteString("  " + estiloPresion.Render(strings.Repeat("▀", ancho-4)) + "\n")
	return b.String()
}

// barra dibuja una barra de progreso real (hecho/total), nunca inventada.
func barra(hecho, total, ancho int) string {
	if total <= 0 {
		total = 1
	}
	lleno := ancho * hecho / total
	if lleno < 0 {
		lleno = 0
	}
	if lleno > ancho {
		lleno = ancho
	}
	return estiloPresion.Render(strings.Repeat("█", lleno)) +
		estiloApagado.Render(strings.Repeat("░", ancho-lleno))
}

// glifoEstado devuelve el glifo de un estado de tarea (§16.2).
func glifoEstado(estado string) string {
	switch estado {
	case "lista":
		return estiloPresion.Render("✓")
	case "detenida":
		return estiloAlerta.Render("⏸")
	case "en_curso":
		return estiloEspera.Render("◐")
	default:
		return estiloApagado.Render("·")
	}
}

// reloj formatea una duración como 1m14s o 3s.
func reloj(d time.Duration) string {
	s := int(d.Seconds())
	if s < 60 {
		return strconv.Itoa(s) + "s"
	}
	return strconv.Itoa(s/60) + "m" + strconv.Itoa(s%60) + "s"
}
