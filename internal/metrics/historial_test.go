package metrics

import (
	"testing"
)

func TestGuardarYLeerHistorial(t *testing.T) {
	root := t.TempDir()
	m := Metricas{IntentosHastaVerde: 2.0, Ruido: 4.5, Roce: 0.0, RechazoEntrada: 25.0, Tokens: 450}
	if err := GuardarHistorial(root, m); err != nil {
		t.Fatal(err)
	}
	m2 := Metricas{IntentosHastaVerde: 3.0, Ruido: 4.5, Roce: 1.0, RechazoEntrada: 25.0, Tokens: 900}
	if err := GuardarHistorial(root, m2); err != nil {
		t.Fatal(err)
	}

	lista, err := LeerHistorial(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(lista) != 2 {
		t.Fatalf("historial = %d entradas, quiero 2", len(lista))
	}
	if lista[0].Metricas.IntentosHastaVerde != 2.0 || lista[1].Metricas.Tokens != 900 {
		t.Errorf("historial = %+v", lista)
	}
	if lista[0].Fecha.IsZero() {
		t.Error("fecha del snapshot no puede ser cero")
	}
	if lista[1].Fecha.Before(lista[0].Fecha) {
		t.Error("orden del historial invertido")
	}
}

func TestLeerHistorialVacio(t *testing.T) {
	lista, err := LeerHistorial(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if lista != nil {
		t.Errorf("sin historial debió dar nil, dio %+v", lista)
	}
}

func TestCompararSinPrev(t *testing.T) {
	tr := Comparar(nil, Metricas{})
	want := Tendencia{IntentosHastaVerde: "·", Ruido: "·", Roce: "·", Friccion: "·", RechazoEntrada: "·"}
	if tr != want {
		t.Errorf("sin prev = %+v, quiero todo ·", tr)
	}
}

func TestCompararFlechas(t *testing.T) {
	f := 10.0
	prev := Metricas{IntentosHastaVerde: 2.0, Ruido: 4.5, Roce: 1.0, Friccion: &f, RechazoEntrada: 25.0}
	curr := Metricas{IntentosHastaVerde: 3.0, Ruido: 4.5, Roce: 0.0, Friccion: nil, RechazoEntrada: 40.0}

	tr := Comparar(&prev, curr)
	if tr.IntentosHastaVerde != "↑" {
		t.Errorf("intentos = %q, quiero ↑", tr.IntentosHastaVerde)
	}
	if tr.Ruido != "·" {
		t.Errorf("ruido = %q, quiero ·", tr.Ruido)
	}
	if tr.Roce != "↓" {
		t.Errorf("roce = %q, quiero ↓", tr.Roce)
	}
	if tr.Friccion != "·" {
		t.Errorf("fricción = %q, quiero · (null actual)", tr.Friccion)
	}
	if tr.RechazoEntrada != "↑" {
		t.Errorf("rechazo = %q, quiero ↑", tr.RechazoEntrada)
	}
}
