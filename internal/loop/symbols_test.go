package loop

import (
	"testing"
)

func TestSimbolosExportadosGo(t *testing.T) {
	root := repoConCommit(t)
	escribir(t, root, "src/export/writer.go", `package export

// WriteCSV exporta clientes a CSV.
func WriteCSV(path string) error { return nil }

type csvOptions struct {
	delimiter rune
}

// Options configura la exportación.
type Options struct {
	Delimiter rune
}

const LímiteFilas = 1000

func helperinterno() {}
`)
	if _, err := gitRun(root, "add", "-A"); err != nil {
		t.Fatal(err)
	}

	syms, err := simbolosExportados(root, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if syms == nil {
		t.Fatal("símbolos nil para un diff con Go")
	}
	want := map[string]bool{"WriteCSV": true, "Options": true, "LímiteFilas": true}
	got := map[string]bool{}
	for _, s := range *syms {
		got[s] = true
	}
	if len(got) != len(want) {
		t.Fatalf("símbolos = %v, quiero %v", *syms, want)
	}
	for s := range want {
		if !got[s] {
			t.Errorf("falta el símbolo exportado %s", s)
		}
	}
	if got["csvOptions"] || got["helperinterno"] {
		t.Errorf("símbolos no exportados se colaron: %v", *syms)
	}
}

func TestSimbolosExportadosSinGoEsNull(t *testing.T) {
	root := repoConCommit(t)
	escribir(t, root, "src/app.ts", "export function x() {}\n")
	if _, err := gitRun(root, "add", "-A"); err != nil {
		t.Fatal(err)
	}
	syms, err := simbolosExportados(root, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if syms != nil {
		t.Errorf("sin Go debió devolver nil (null), dio %v", *syms)
	}
}
