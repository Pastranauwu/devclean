package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

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

func TestArmarTableroMarcaSeleccionYHint(t *testing.T) {
	filas := []Fila{
		{ID: "T-001", Titulo: "exportar", Estado: state.Lista},
		{ID: "T-002", Titulo: "login", Estado: state.Pendiente},
	}
	texto := ""
	for _, l := range armarTablero(filas, "T-001", "") {
		texto += l.texto + "\n"
	}
	if !strings.Contains(texto, "> T-001") {
		t.Errorf("debió marcar T-001:\n%s", texto)
	}
	if !strings.Contains(texto, "s dry-run") {
		t.Errorf("debió mostrar el hint de s:\n%s", texto)
	}
}

func TestCursorInicialPrefiereLista(t *testing.T) {
	filas := []Fila{
		{ID: "T-001", Titulo: "a", Estado: state.Pendiente},
		{ID: "T-002", Titulo: "b", Estado: state.Lista},
	}
	if got := cursorInicial(filas); got != 1 {
		t.Errorf("cursorInicial = %d, quiere 1", got)
	}
	if got := cursorInicial(nil); got != 0 {
		t.Errorf("vacío = %d, quiere 0", got)
	}
}

func TestBoardKeySSobreLista(t *testing.T) {
	m := boardModel{filas: []Fila{
		{ID: "T-001", Titulo: "a", Estado: state.Lista},
		{ID: "T-002", Titulo: "b", Estado: state.Pendiente},
	}}
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	bm := next.(boardModel)
	if bm.shipID != "T-001" {
		t.Errorf("shipID = %q, quiere T-001", bm.shipID)
	}
	if cmd == nil {
		t.Fatal("s sobre lista debió salir para disparar dry-run")
	}
}

func TestBoardKeySSobrePendiente(t *testing.T) {
	m := boardModel{
		filas: []Fila{
			{ID: "T-001", Titulo: "a", Estado: state.Lista},
			{ID: "T-002", Titulo: "b", Estado: state.Pendiente},
		},
		cursor: 1,
	}
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	bm := next.(boardModel)
	if bm.shipID != "" {
		t.Errorf("shipID = %q, pendiente no debe disparar dry-run", bm.shipID)
	}
	if cmd != nil {
		t.Fatal("s sobre pendiente no debe salir")
	}
	if !strings.Contains(bm.aviso, "T-002 no está lista") {
		t.Errorf("aviso = %q", bm.aviso)
	}
}

func TestBoardJKMueve(t *testing.T) {
	m := boardModel{filas: []Fila{
		{ID: "T-001", Titulo: "a", Estado: state.Lista},
		{ID: "T-002", Titulo: "b", Estado: state.Pendiente},
	}}
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	bm := next.(boardModel)
	if bm.cursor != 1 {
		t.Errorf("j = %d, quiere 1", bm.cursor)
	}
	next, _ = bm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	bm = next.(boardModel)
	if bm.cursor != 0 {
		t.Errorf("j wrap = %d, quiere 0", bm.cursor)
	}
	next, _ = bm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	bm = next.(boardModel)
	if bm.cursor != 1 {
		t.Errorf("k wrap = %d, quiere 1", bm.cursor)
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
