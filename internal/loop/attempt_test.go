package loop

import (
	"encoding/json"
	"testing"
	"time"
)

func TestStoreAppendYRead(t *testing.T) {
	root := t.TempDir()
	s, err := openStore(root, "T-001")
	if err != nil {
		t.Fatal(err)
	}
	uno := 1
	cinco := 5
	cuatro := 4
	if err := s.Append(Attempt{
		Intento:         1,
		Inicio:          time.Date(2026, 8, 26, 14, 31, 2, 0, time.UTC),
		Fin:             time.Date(2026, 8, 26, 14, 33, 47, 0, time.UTC),
		SalidaCodigo:    &uno,
		TestsPasaron:    &cinco,
		TestsFallaron:   &cuatro,
		ArchivosTocados: []string{"src/export/writer.go"},
		LineasMas:       118,
		LineasMenos:     12,
		Tokens:          Tokens{Entrada: 18400, Salida: 3100},
		Modelo:          "glm-5.2",
	}); err != nil {
		t.Fatal(err)
	}

	got, err := ReadAttempts(root, "T-001")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("ReadAttempts = %d líneas, quiero 1", len(got))
	}
	a := got[0]
	if a.Intento != 1 || a.Modelo != "glm-5.2" || a.Tokens.Entrada != 18400 {
		t.Errorf("línea mal leída: %+v", a)
	}
	if a.SalidaCodigo == nil || *a.SalidaCodigo != 1 {
		t.Errorf("salida_codigo = %v, quiero 1", a.SalidaCodigo)
	}
	if a.TestsPasaron == nil || *a.TestsPasaron != 5 {
		t.Errorf("tests_pasaron = %v, quiero 5", a.TestsPasaron)
	}
	if a.SimbolosExportados != nil {
		t.Errorf("simbolos_exportados debió quedar null, es %v", a.SimbolosExportados)
	}
}

func TestAttemptJSONClaves(t *testing.T) {
	// el contrato del formato vive en docs/attempts-jsonl.md: las claves
	// en JSON son las de ese documento, no el nombre del campo en Go.
	data, err := json.Marshal(Attempt{Intento: 2})
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	for _, clave := range []string{"intento", "salida_codigo", "tests_pasaron", "tests_fallaron", "archivos_tocados", "simbolos_exportados", "lineas_mas", "lineas_menos", "revertidos_fuera_de_alcance", "tokens", "modelo"} {
		if _, ok := m[clave]; !ok {
			t.Errorf("falta la clave %q en %s", clave, data)
		}
	}
}

func TestReadAttemptsSinArchivo(t *testing.T) {
	got, err := ReadAttempts(t.TempDir(), "T-999")
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("sin archivo debió devolver nil, dio %v", got)
	}
}
