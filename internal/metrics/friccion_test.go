package metrics

import (
	"context"
	"testing"
)

func TestMinutosHastaAprobarTomaLaPrimeraAprobacion(t *testing.T) {
	// abierto 10:00; cambios pedidos 10:30; aprobado 11:30; aprobado de
	// nuevo 14:00 — lo que desbloquea es el primer APPROVED: 90 min
	data := []byte(`{
	  "createdAt": "2026-09-14T10:00:00Z",
	  "reviews": [
	    {"state": "CHANGES_REQUESTED", "submittedAt": "2026-09-14T10:30:00Z"},
	    {"state": "APPROVED",          "submittedAt": "2026-09-14T11:30:00Z"},
	    {"state": "APPROVED",          "submittedAt": "2026-09-14T14:00:00Z"}
	  ]
	}`)
	got, ok := minutosHastaAprobar(data)
	if !ok {
		t.Fatal("no midió un PR aprobado")
	}
	if got != 90 {
		t.Errorf("fricción = %v min, quiero 90", got)
	}
}

// Sin nada que medir se dice "no hay dato", nunca un cero: cero minutos
// de fricción es un número excelente y sería mentira.
func TestSinAprobacionNoHayDato(t *testing.T) {
	casos := map[string]string{
		"PR abierto sin revisar":  `{"createdAt":"2026-09-14T10:00:00Z","reviews":[]}`,
		"solo cambios pedidos":    `{"createdAt":"2026-09-14T10:00:00Z","reviews":[{"state":"CHANGES_REQUESTED","submittedAt":"2026-09-14T10:30:00Z"}]}`,
		"solo un comentario":      `{"createdAt":"2026-09-14T10:00:00Z","reviews":[{"state":"COMMENTED","submittedAt":"2026-09-14T10:30:00Z"}]}`,
		"sin fecha de apertura":   `{"reviews":[{"state":"APPROVED","submittedAt":"2026-09-14T11:00:00Z"}]}`,
		"aprobado antes de abrir": `{"createdAt":"2026-09-14T10:00:00Z","reviews":[{"state":"APPROVED","submittedAt":"2026-09-14T09:00:00Z"}]}`,
		"json ilegible":           `no es json`,
	}
	for nombre, data := range casos {
		if got, ok := minutosHastaAprobar([]byte(data)); ok {
			t.Errorf("%s: midió %v min, quiero sin dato", nombre, got)
		}
	}
}

// Sin entregas con PR ni siquiera se llama a gh, y el resultado es nil,
// no 0: es lo que report imprime como "— sin datos".
func TestFriccionSinPRsEsNil(t *testing.T) {
	if f := Friccion(context.Background(), t.TempDir(), []Entrega{{ID: "T-001"}}); f != nil {
		t.Errorf("Friccion sin PRs = %v, quiero nil", *f)
	}
}
