package ship

import (
	"strings"
	"testing"
)

func TestResumenPruebasConservaAsercionAntesDelEnlaceFinal(t *testing.T) {
	salida := " FAIL  tests/screens/tutoriales.test.ts > muestra intro\n" +
		"AssertionError: expected texto to contain frase\n" +
		"Expected: Cada tutorial dura 1 o 2 pantallas\n" +
		"Received: Tutoriales de juego y de teoría musical: cada uno dura 1 o 2 pantallas\n" +
		"learn more: https://vitest.dev/guide/improving-performance\n"
	got := resumenPruebas(salida)
	for _, want := range []string{"tests/screens/tutoriales.test.ts", "Expected: Cada tutorial", "Received: Tutoriales"} {
		if !strings.Contains(got, want) {
			t.Errorf("falta %q en %q", want, got)
		}
	}
	if strings.Contains(got, "learn more") {
		t.Errorf("el enlace no explica el fallo: %q", got)
	}
}
