package loop

import (
	"strings"
	"testing"
)

func TestResumenFalloComandoSilencioso(t *testing.T) {
	code := 1
	// el caso real: `grep -q` no imprime nada, y el agente recibía
	// "sin salida" como único dato del intento anterior
	cmd := "go run . --help 2>&1 | grep -q -- --mac"
	got := resumenFallo(cmd, &code, "")

	for _, quiero := range []string{"código 1", cmd, "silencia su salida"} {
		if !strings.Contains(got, quiero) {
			t.Errorf("resumen %q no menciona %q", got, quiero)
		}
	}
	if strings.Contains(got, "sin salida") {
		t.Errorf("resumen %q sigue sin decir nada útil", got)
	}
}

func TestResumenFalloSinCodigo(t *testing.T) {
	got := resumenFallo("make test", nil, "")
	if !strings.Contains(got, "make test") || strings.Contains(got, "código") {
		t.Errorf("resumen = %q", got)
	}
}

func TestResumenFalloConSalida(t *testing.T) {
	code := 1
	salida := "compilando\n\nFAIL wol_test.go:12: quiero 6 bytes\nFAIL\texit status 1\n"
	got := resumenFallo("go test ./...", &code, salida)

	// manda la salida real, y con contexto: la última línea sola no
	// alcanza para arreglar un test
	if !strings.Contains(got, "wol_test.go:12") {
		t.Errorf("resumen %q perdió la línea que importa", got)
	}
	if strings.Contains(got, "go test ./...") {
		t.Errorf("con salida real no hace falta repetir el comando · %q", got)
	}
	// las líneas vacías no gastan cupo
	if strings.Contains(got, "\n\n") {
		t.Errorf("resumen %q conserva líneas vacías", got)
	}
}

func TestUltimasLineasAcota(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 50; i++ {
		b.WriteString("linea de relleno bastante larga para pasar el tope\n")
	}
	got := ultimasLineas(b.String(), 6, 200)
	if tope := 200 + len("…"); len(got) > tope {
		t.Errorf("largo = %d, quiero <= %d", len(got), tope)
	}
	if !strings.HasPrefix(got, "…") {
		t.Errorf("un recorte debe avisarse · %q", got)
	}
	if ultimasLineas("   \n\n  ", 6, 200) != "" {
		t.Error("solo espacios en blanco es nada")
	}
}
