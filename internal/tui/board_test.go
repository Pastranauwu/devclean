package tui

import (
	"strings"
	"testing"

	"github.com/Pastranauwu/devclean/internal/state"
)

func TestLineasTablero(t *testing.T) {
	filas := []Fila{
		{ID: "T-001", Titulo: "exportar", Estado: state.Lista},
		{ID: "T-002", Titulo: "login", Estado: state.EnCurso},
		{ID: "T-003", Titulo: "docs", Estado: state.Pendiente},
	}
	ls := lineasTablero(filas)
	texto := ""
	for _, l := range ls {
		texto += l.texto + "\n"
	}
	for _, want := range []string{"LISTO PARA ENTREGAR", "T-001", "exportar", "EN CURSO", "T-002", "PENDIENTE", "T-003", "DETENIDO", "—", "dirige agentes"} {
		if !strings.Contains(texto, want) {
			t.Errorf("tablero sin %q:\n%s", want, texto)
		}
	}
}

func TestLineasTableroVacio(t *testing.T) {
	ls := lineasTablero(nil)
	texto := ""
	for _, l := range ls {
		texto += l.texto + "\n"
	}
	if !strings.Contains(texto, "sin tareas") {
		t.Errorf("tablero vacío sin invitación a actuar:\n%s", texto)
	}
}

func TestFondoPlasma(t *testing.T) {
	frame := FondoPlasma(40, 10, 0, DefaultPlasma(), []lineaSticker{
		{texto: "devclean", color: rgbPresion},
	}, 1)
	if !strings.Contains(frame, "▀") {
		t.Error("el plasma debió renderizar medio bloque ▀")
	}
	if !strings.Contains(frame, "\x1b[38;2;") {
		t.Error("el plasma debió usar truecolor fg")
	}
	if !strings.Contains(frame, "\x1b[48;2;") {
		t.Error("el plasma debió usar truecolor bg")
	}
	if !strings.Contains(frame, "devclean") {
		t.Error("el sticker debió tapar con el texto")
	}
}
