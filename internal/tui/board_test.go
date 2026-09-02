package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Pastranauwu/devclean/internal/state"
)

func TestArmarTableroColumnas(t *testing.T) {
	filas := []Fila{
		{ID: "T-001", Titulo: "exportar", Estado: state.Lista},
		{ID: "T-002", Titulo: "login", Estado: state.EnCurso},
		{ID: "T-003", Titulo: "docs", Estado: state.Pendiente},
	}
	ls := armarTablero(filas, "", "", nil)
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

func TestArmarTableroMuestraArbolDeSubtareas(t *testing.T) {
	filas := []Fila{
		{ID: "T-001", Titulo: "feature grande", Estado: state.Lista, Hijos: []Fila{
			{ID: "T-001001", Titulo: "parte uno", Estado: state.Lista},
			{ID: "T-001002", Titulo: "parte dos", Estado: state.Detenida, Hijos: []Fila{
				{ID: "T-001002001", Titulo: "sub de la parte dos", Estado: state.Lista},
			}},
		}},
	}
	ls := armarTablero(filas, "", "", nil)
	texto := ""
	for _, l := range ls {
		texto += l.texto + "\n"
	}
	for _, want := range []string{"T-001", "  └ T-001001", "  └ T-001002", "    └ T-001002001"} {
		if !strings.Contains(texto, want) {
			t.Errorf("tablero sin %q:\n%s", want, texto)
		}
	}
}

func TestArmarTableroVacio(t *testing.T) {
	ls := armarTablero(nil, "", "", nil)
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
	for _, l := range armarTablero(filas, "T-001", "", nil) {
		texto += l.texto + "\n"
	}
	if !strings.Contains(texto, "> T-001") {
		t.Errorf("debió marcar T-001:\n%s", texto)
	}
	if !strings.Contains(texto, "s entrega") {
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
	if bm.accion != (Accion{AccionEntregar, "T-001"}) {
		t.Errorf("accion = %+v, quiere entregar T-001", bm.accion)
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
	if bm.accion.Tipo != "" {
		t.Errorf("accion = %+v, pendiente no debe disparar dry-run", bm.accion)
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

// El tablero tiene que decir qué está haciendo una tarea ahora mismo,
// no solo que está "en curso": una invocación de veinte minutos se veía
// igual que una colgada.
func TestArmarTableroMuestraFaseViva(t *testing.T) {
	filas := []Fila{
		{ID: "T-002", Titulo: "login", Estado: state.EnCurso,
			Detalle: "intento 2/3 · agente · opencode/glm-5.3 · 3m12s"},
	}
	texto := textoTablero(armarTablero(filas, "", "", nil))
	for _, want := range []string{"T-002", "intento 2/3", "agente", "opencode/glm-5.3", "3m12s"} {
		if !strings.Contains(texto, want) {
			t.Errorf("tablero sin %q:\n%s", want, texto)
		}
	}
}

func TestArmarTableroMarcaAtasco(t *testing.T) {
	filas := []Fila{
		{ID: "T-003", Titulo: "cli", Estado: state.EnCurso, Atascada: true,
			Detalle: "ATASCO · intento 1/3 · agente · 42m0s sin señal"},
	}
	ls := armarTablero(filas, "", "", nil)
	if !strings.Contains(textoTablero(ls), "ATASCO") {
		t.Error("el tablero debe marcar el atasco")
	}
	// y en color de alerta, no apagado: es lo que hace que se vea
	var visto bool
	for _, l := range ls {
		if strings.Contains(l.texto, "ATASCO") {
			visto = true
			if l.color != rgbAlerta {
				t.Errorf("color = %v, quiero rgbAlerta", l.color)
			}
		}
	}
	if !visto {
		t.Error("no se encontró la línea del atasco")
	}
}

// Una tarea que no corre no lleva línea de detalle.
func TestArmarTableroSinDetalleSiNoCorre(t *testing.T) {
	filas := []Fila{{ID: "T-001", Titulo: "docs", Estado: state.Pendiente}}
	ls := armarTablero(filas, "", "", nil)
	for _, l := range ls {
		if strings.Contains(l.texto, "intento") {
			t.Errorf("línea de detalle inesperada: %q", l.texto)
		}
	}
}

func textoTablero(ls []lineaSticker) string {
	var b strings.Builder
	for _, l := range ls {
		b.WriteString(l.texto + "\n")
	}
	return b.String()
}

// r revive una tarea detenida; sobre cualquier otro estado avisa.
func TestBoardKeyRSoloSobreDetenida(t *testing.T) {
	filas := []Fila{
		{ID: "T-001", Titulo: "a", Estado: state.Detenida},
		{ID: "T-002", Titulo: "b", Estado: state.Lista},
	}

	m := boardModel{filas: filas}
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if bm := next.(boardModel); bm.accion != (Accion{AccionReintentar, "T-001"}) {
		t.Errorf("accion = %+v, quiere reintentar T-001", bm.accion)
	}
	if cmd == nil {
		t.Fatal("r sobre detenida debe salir para correr el reintento")
	}

	m = boardModel{filas: filas, cursor: 1}
	next, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	bm := next.(boardModel)
	if bm.accion.Tipo != "" || cmd != nil {
		t.Errorf("accion = %+v · r sobre una lista no reintenta nada", bm.accion)
	}
	if !strings.Contains(bm.aviso, "T-002 no está detenida") {
		t.Errorf("aviso = %q", bm.aviso)
	}
}

// d muestra el detalle de cualquier tarea, sea cual sea su estado: es
// justo cuando algo va mal cuando hace falta mirar.
func TestBoardKeyDSiempreDisponible(t *testing.T) {
	for _, estado := range []string{state.Lista, state.Detenida, state.EnCurso, state.Pendiente} {
		m := boardModel{filas: []Fila{{ID: "T-001", Estado: estado}}}
		next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
		bm := next.(boardModel)
		if bm.accion != (Accion{AccionDetalle, "T-001"}) || cmd == nil {
			t.Errorf("estado %s: accion = %+v", estado, bm.accion)
		}
	}
}

func TestBoardAyudaListaLasTeclas(t *testing.T) {
	texto := textoTablero(armarTablero([]Fila{{ID: "T-001", Estado: state.Lista}}, "", "", nil))
	for _, tecla := range []string{"s entrega", "r reintenta", "d detalle", "q sale"} {
		if !strings.Contains(texto, tecla) {
			t.Errorf("la ayuda no menciona %q", tecla)
		}
	}
}
