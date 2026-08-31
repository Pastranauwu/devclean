package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPlantillaPruebas(t *testing.T) {
	casos := []struct {
		stack string
		want  string
		ok    bool
	}{
		{"go", "go test ./...", true},
		{"GO", "go test ./...", true},
		{"node", "npm test", true},
		{"jest", "npm test", true},
		{"python", "pytest", true},
		{"pytest", "pytest", true},
		{"rust", "", false},
		{"", "", false},
		{"  go  ", "go test ./...", true},
	}
	for _, c := range casos {
		got, ok := PlantillaPruebas(c.stack)
		if got != c.want || ok != c.ok {
			t.Errorf("PlantillaPruebas(%q) = (%q, %v), quiero (%q, %v)", c.stack, got, ok, c.want, c.ok)
		}
	}
}

func TestPlantillasListoCuando(t *testing.T) {
	for _, stack := range []string{"go", "node", "jest", "python", "pytest"} {
		got := PlantillasListoCuando(stack)
		if len(got) < 2 || len(got) > 3 {
			t.Errorf("%s: %d ejemplos, quiero 2 o 3", stack, len(got))
		}
	}
	if got := PlantillasListoCuando("rust"); got != nil {
		t.Errorf("stack desconocido inventó plantilla: %v", got)
	}
	if got := PlantillasListoCuando(""); got != nil {
		t.Errorf("vacío inventó plantilla: %v", got)
	}
}

func TestPlantillasListoCuandoGoNoEsElComandoGlobal(t *testing.T) {
	for _, e := range PlantillasListoCuando("go") {
		if e == "go test ./..." {
			t.Error("el ejemplo no debe ser el comando global: hoy ya pasa y la esclusa lo rechaza")
		}
	}
}

func TestStackDePlantilla(t *testing.T) {
	casos := map[string]string{
		"go": "go", "jest": "node", "node": "node",
		"pytest": "python", "python": "python", "rust": "", "": "",
	}
	for in, want := range casos {
		if got := StackDePlantilla(in); got != want {
			t.Errorf("StackDePlantilla(%q) = %q, quiero %q", in, got, want)
		}
	}
}

func TestDetectLanguageAlimentaPlantilla(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stack := DetectLanguage(root)
	if PlantillasListoCuando(stack) == nil {
		t.Errorf("DetectLanguage=%q no tiene plantilla", stack)
	}
}
