package tui

import (
	"strconv"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func tecla(s string) tea.KeyMsg {
	if s == " " {
		return tea.KeyMsg{Type: tea.KeySpace}
	}
	if s == "enter" {
		return tea.KeyMsg{Type: tea.KeyEnter}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func pulsar(m listaModel, teclas ...string) listaModel {
	for _, t := range teclas {
		nuevo, _ := m.Update(tecla(t))
		m = nuevo.(listaModel)
	}
	return m
}

func opciones() []Opcion {
	return []Opcion{
		{ID: "T-001", Etiqueta: "uno"},
		{ID: "T-002", Etiqueta: "dos"},
		{ID: "T-003", Etiqueta: "tres"},
	}
}

func marcados(m listaModel) []string {
	var ids []string
	for _, o := range m.ops {
		if o.Marcada {
			ids = append(ids, o.ID)
		}
	}
	return ids
}

func TestListaMarcaYConfirma(t *testing.T) {
	m := listaModel{ops: opciones(), ayudaMulti: true}
	m = pulsar(m, " ", "j", "j", " ", "enter")

	if !m.confirmado {
		t.Fatal("enter debe confirmar")
	}
	got := marcados(m)
	if len(got) != 2 || got[0] != "T-001" || got[1] != "T-003" {
		t.Errorf("marcados = %v, quiero [T-001 T-003]", got)
	}
}

func TestListaCancelarNoConfirma(t *testing.T) {
	m := listaModel{ops: opciones(), ayudaMulti: true}
	m = pulsar(m, " ", "q")
	if m.confirmado {
		t.Error("q no debe confirmar · cancelar no crea nada")
	}
}

func TestListaTodasYNinguna(t *testing.T) {
	m := listaModel{ops: opciones(), ayudaMulti: true}
	if m = pulsar(m, "a"); len(marcados(m)) != 3 {
		t.Errorf("a debe marcar todas · %v", marcados(m))
	}
	if m = pulsar(m, "n"); len(marcados(m)) != 0 {
		t.Errorf("n debe desmarcar todas · %v", marcados(m))
	}
}

func TestListaCursorCicla(t *testing.T) {
	m := listaModel{ops: opciones()}
	if m = pulsar(m, "k"); m.cursor != 2 {
		t.Errorf("k desde el tope debe ir al final · cursor = %d", m.cursor)
	}
	if m = pulsar(m, "j"); m.cursor != 0 {
		t.Errorf("j desde el final debe volver al tope · cursor = %d", m.cursor)
	}
}

// En selección única el espacio no marca nada: no hay casillas.
func TestListaUnicaIgnoraEspacio(t *testing.T) {
	m := listaModel{ops: opciones()}
	m = pulsar(m, " ")
	if len(marcados(m)) != 0 {
		t.Errorf("marcados = %v, la lista única no tiene casillas", marcados(m))
	}
}

func TestListaVistaMuestraCasillasYDetalle(t *testing.T) {
	ops := []Opcion{{ID: "a", Etiqueta: "opción a", Detalle: "por qué importa", Marcada: true}}
	vista := listaModel{titulo: "ELIGE", ops: ops, ayudaMulti: true, ayuda: "enter confirma"}.View()
	for _, quiero := range []string{"ELIGE", "opción a", "por qué importa", "[x]", "enter confirma"} {
		if !strings.Contains(vista, quiero) {
			t.Errorf("vista sin %q", quiero)
		}
	}
}

func TestVentanaSigueAlCursor(t *testing.T) {
	// cabe entero: sin recorte
	if d, h := ventana(0, 5, 12); d != 0 || h != 5 {
		t.Errorf("ventana = (%d,%d), quiero (0,5)", d, h)
	}
	// cursor arriba: la ventana arranca en 0
	if d, h := ventana(1, 34, 12); d != 0 || h != 12 {
		t.Errorf("ventana = (%d,%d), quiero (0,12)", d, h)
	}
	// cursor al final: la ventana termina en el total
	if d, h := ventana(33, 34, 12); d != 22 || h != 34 {
		t.Errorf("ventana = (%d,%d), quiero (22,34)", d, h)
	}
	// en medio: el cursor queda dentro
	d, h := ventana(20, 34, 12)
	if 20 < d || 20 >= h {
		t.Errorf("ventana = (%d,%d) deja fuera al cursor 20", d, h)
	}
}

// Un catálogo largo no se pinta entero, y se avisa cuánto queda fuera.
func TestListaVistaRecortaCatalogoLargo(t *testing.T) {
	ops := make([]Opcion, 34)
	for i := range ops {
		ops[i] = Opcion{ID: strconv.Itoa(i), Etiqueta: "modelo-" + strconv.Itoa(i)}
	}
	vista := listaModel{titulo: "MODELOS", ops: ops, cursor: 20}.View()

	if strings.Contains(vista, "modelo-0\n") {
		t.Error("la ventana no debe llegar hasta el primero")
	}
	if !strings.Contains(vista, "modelo-20") {
		t.Error("el cursor debe estar visible")
	}
	for _, quiero := range []string{"más arriba", "más abajo"} {
		if !strings.Contains(vista, quiero) {
			t.Errorf("la vista no avisa de lo que queda fuera (%q)", quiero)
		}
	}
}
