package ship

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

func TestPruebasFallidasPorRunner(t *testing.T) {
	salida := strings.Join([]string{
		"FAIL: test_login (apps.user.tests.UserTest.test_login)",
		"ERROR: test_rol (apps.user.tests.UserTest.test_rol)",
		"FAILED tests/test_api.py::test_crea - AssertionError: 1 != 2",
		"--- FAIL: TestSuma (0.00s)",
		"FAIL\tgithub.com/x/calc\t0.012s",
		" FAIL  src/core/types.test.ts",
		"FAIL",
		"ok  	github.com/x/otro	0.1s",
	}, "\n")
	got := pruebasFallidas(salida)
	for _, quiero := range []string{
		"test_login (apps.user.tests.UserTest.test_login)", "test_rol (apps.user.tests.UserTest.test_rol)",
		"tests/test_api.py::test_crea", "TestSuma", "github.com/x/calc", "src/core/types.test.ts",
	} {
		if !got[quiero] {
			t.Errorf("falta %q en %v", quiero, got)
		}
	}
	if len(got) != 6 {
		t.Errorf("sobran fallos: %v", got)
	}
}

// con pruebas ya rotas en la base ninguna entrega podía pasar; lo que el conjunto rompe de nuevo sí frena
func TestMismosFallosQueLaBase(t *testing.T) {
	dir := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		if out, err := gitRun(dir, append([]string{"-c", "user.email=t@t", "-c", "user.name=t"}, args...)...); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	suite := func(fallos ...string) {
		t.Helper()
		script := "#!/bin/sh\n"
		for _, f := range fallos {
			script += "echo 'FAIL: " + f + "'\n"
		}
		if err := os.WriteFile(filepath.Join(dir, "suite.sh"), []byte(script+"exit 1\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		git("add", "-A")
		git("commit", "-q", "-m", "suite")
	}
	git("init", "-q", "-b", "main")
	suite("vieja_a", "vieja_b")
	git("checkout", "-q", "-b", RamaEntrega)
	if err := os.WriteFile(filepath.Join(dir, "nuevo.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "-A")
	git("commit", "-q", "-m", "trabajo")

	ctx := context.Background()
	crudo, _, _ := correrPruebasCrudo(ctx, dir, "./suite.sh", time.Minute)
	if detalle, ok := mismosFallosQueLaBase(ctx, dir, "main", "./suite.sh", time.Minute, crudo); !ok {
		t.Fatalf("los mismos fallos que la base frenaron: %q", detalle)
	}
	if out, _ := gitRun(dir, "rev-parse", "--abbrev-ref", "HEAD"); strings.TrimSpace(out) != RamaEntrega {
		t.Fatalf("no volvió a la rama de entrega: %s", out)
	}
	suite("vieja_a", "vieja_b", "nueva_c")
	crudo, _, _ = correrPruebasCrudo(ctx, dir, "./suite.sh", time.Minute)
	detalle, ok := mismosFallosQueLaBase(ctx, dir, "main", "./suite.sh", time.Minute, crudo)
	if ok || !strings.Contains(detalle, "nueva_c") {
		t.Fatalf("un fallo nuevo pasó: ok=%v %q", ok, detalle)
	}
}
