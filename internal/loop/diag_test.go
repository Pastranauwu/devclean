package loop

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFalloDeInfra(t *testing.T) {
	err := errors.New("exit status 1")

	// el caso real: modelo inexistente · sale mal, 0 tokens, 0 archivos
	if !falloDeInfra(Result{ExitCode: 1}, err, nil) {
		t.Error("una invocación que no gastó tokens ni tocó archivos es fallo de infra")
	}
	// el agente sí trabajó aunque el CLI saliera mal: hay que reintentar
	if falloDeInfra(Result{ExitCode: 1, Tokens: Tokens{Entrada: 10}}, err, nil) {
		t.Error("con tokens gastados no es fallo de infra")
	}
	if falloDeInfra(Result{ExitCode: 1}, err, []string{"main.go"}) {
		t.Error("con archivos tocados no es fallo de infra")
	}
	// sin error del proceso nunca es fallo de infra
	if falloDeInfra(Result{}, nil, nil) {
		t.Error("sin error no es fallo de infra")
	}
}

func TestDiagnostico(t *testing.T) {
	err := errors.New("exit status 1")

	if d := diagnostico(Result{Stderr: "model not found\n"}, err); d != "model not found" {
		t.Errorf("stderr manda: %q", d)
	}
	// opencode reporta sus errores en stdout, en modo JSON
	jsonErr := `{"type":"error","error":{"name":"UnknownError"}}`
	if d := diagnostico(Result{Stdout: jsonErr}, err); d != jsonErr {
		t.Errorf("stdout como respaldo: %q", d)
	}
	if d := diagnostico(Result{}, err); d != "exit status 1" {
		t.Errorf("error del proceso como último recurso: %q", d)
	}
	if d := diagnostico(Result{}, nil); d != "sin salida" {
		t.Errorf("d = %q", d)
	}
}

func TestGuardarLogDejaRastroYNoVersionaElVolcado(t *testing.T) {
	root := t.TempDir()
	rel := guardarLog(root, "T-001", 2, "el prompt", Result{
		Stdout: "salida cruda", Stderr: "model not found", ExitCode: 1,
	}, errors.New("exit status 1"))

	if rel == "" {
		t.Fatal("guardarLog debe devolver la ruta relativa del volcado")
	}
	datos, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatalf("no se escribió el volcado · %v", err)
	}
	for _, quiero := range []string{"el prompt", "salida cruda", "model not found", "exit status 1"} {
		if !strings.Contains(string(datos), quiero) {
			t.Errorf("el volcado no contiene %q", quiero)
		}
	}

	// los volcados no entran al repo: attempts.jsonl sí, esto no
	ign, err := os.ReadFile(filepath.Join(RunsDir(root), ".gitignore"))
	if err != nil {
		t.Fatalf("falta el .gitignore de runs/ · %v", err)
	}
	if !strings.Contains(string(ign), "intento-") {
		t.Errorf(".gitignore = %q", ign)
	}
}
