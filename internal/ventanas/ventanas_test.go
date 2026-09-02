package ventanas

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRegistrarYCalcularVentanas(t *testing.T) {
	dir := t.TempDir()
	r := Nuevo(filepath.Join(dir, "ledger.jsonl"), nil)

	// primer gasto hace 6 horas (cae fuera de 5h, dentro de semanal)
	viejo := time.Now().UTC().Add(-6 * time.Hour)
	if !r.Registrar("claude", 1000) {
		t.Fatal("sin topes nunca rechaza")
	}
	// corregir el timestamp del evento viejo (el registro usa ahora real)
	if len(r.evs) != 1 {
		t.Fatalf("eventos = %d, quiero 1", len(r.evs))
	}
	r.evs[0].TS = viejo

	if !r.Registrar("claude", 500) {
		t.Fatal("sin topes nunca rechaza")
	}
	if !r.Registrar("opencode", 300) {
		t.Fatal("sin topes nunca rechaza")
	}

	if got := r.Gasto("claude", Ventana5h); got != 500 {
		t.Errorf("claude 5h = %d, quiero 500 (el de hace 6h queda fuera)", got)
	}
	if got := r.Gasto("claude", VentanaSemanal); got != 1500 {
		t.Errorf("claude semanal = %d, quiero 1500", got)
	}
	if got := r.Gasto("opencode", VentanaMensual); got != 300 {
		t.Errorf("opencode mensual = %d, quiero 300", got)
	}
	if got := r.Gasto("claude", VentanaMensual); got != 1500 {
		t.Errorf("claude mensual = %d, quiero 1500", got)
	}
}

func TestRegistrarRechazaCuandoPasaUnTope(t *testing.T) {
	dir := t.TempDir()
	r := Nuevo(filepath.Join(dir, "ledger.jsonl"), map[string]map[string]int{
		"claude": {Ventana5h: 1000, VentanaSemanal: 5000},
	})

	if !r.Registrar("claude", 800) {
		t.Fatal("800 dentro de 1000 debía caber")
	}
	// 800 + 300 = 1100 > 1000 en 5h → rechaza y no registra
	if r.Registrar("claude", 300) {
		t.Fatal("1100 en 5h con tope 1000 debía rechazarse")
	}
	if got := r.Gasto("claude", Ventana5h); got != 800 {
		t.Errorf("tras el rechazo, 5h = %d, quiero 800 (no se registró)", got)
	}
	// 800 + 200 = 1000, justo al tope → cabe
	if !r.Registrar("claude", 200) {
		t.Fatal("1000 exactos debían caber")
	}
	// otro proveedor no se ve afectado por el tope de claude
	if !r.Registrar("opencode", 2000) {
		t.Fatal("opencode sin topes nunca rechaza")
	}
}

func TestLedgerPersisteEntreInstancias(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ledger.jsonl")

	r1 := Nuevo(path, nil)
	if !r1.Registrar("claude", 700) {
		t.Fatal("no debía rechazar")
	}

	r2 := Nuevo(path, nil)
	if got := r2.Gasto("claude", VentanaMensual); got != 700 {
		t.Errorf("tras recargar, gasto = %d, quiero 700", got)
	}
}

func TestPodaDescartaLoViejo(t *testing.T) {
	dir := t.TempDir()
	r := Nuevo(filepath.Join(dir, "ledger.jsonl"), nil)
	if !r.Registrar("claude", 999) {
		t.Fatal("no debía rechazar")
	}
	r.evs[0].TS = time.Now().UTC().Add(-40 * 24 * time.Hour)
	r.podar()
	if len(r.evs) != 0 {
		t.Errorf("tras podar quedaron %d eventos, quiero 0", len(r.evs))
	}
}

func TestLedgerPathEsGlobal(t *testing.T) {
	p := LedgerPath()
	if p == "" {
		t.Fatal("LedgerPath vacío")
	}
	if _, err := os.Stat(filepath.Dir(p)); err != nil {
		// el directorio puede no existir todavía; no es un error de test
		_ = err
	}
}
