package plan

import (
	"context"
	"strings"
	"testing"
)

type generadorFijo struct {
	texto string
	err   error
}

func (g generadorFijo) Generar(_ context.Context, _ string) (string, error) {
	return g.texto, g.err
}

func TestPromptPideJSON(t *testing.T) {
	p := Prompt("exportar clientes")
	for _, want := range []string{"exportar clientes", "listo_cuando", "tocar_solo", "array JSON"} {
		if !strings.Contains(p, want) {
			t.Errorf("prompt sin %q", want)
		}
	}
}

func TestParseArrayLimpio(t *testing.T) {
	bs, err := Parse(`[{"titulo":"exportar","listo_cuando":"go test ./..."}]`)
	if err != nil {
		t.Fatal(err)
	}
	if len(bs) != 1 || bs[0].Titulo != "exportar" {
		t.Fatalf("bs = %+v", bs)
	}
}

func TestParseConVallasYTexto(t *testing.T) {
	texto := "Aquí tienes:\n```json\n[{\"titulo\":\"arreglar login\",\"listo_cuando\":\"npm test -- auth\",\"tocar_solo\":[\"src/auth/**\"]}]\n```\nEspero que sirva."
	bs, err := Parse(texto)
	if err != nil {
		t.Fatal(err)
	}
	if len(bs) != 1 || bs[0].TocarSolo[0] != "src/auth/**" {
		t.Fatalf("bs = %+v", bs)
	}
}

func TestParseSinArray(t *testing.T) {
	if _, err := Parse("no hay array aquí"); err == nil {
		t.Fatal("texto sin array debió fallar")
	}
}

func TestParseBorradorIncompleto(t *testing.T) {
	if _, err := Parse(`[{"titulo":"sin listo"}]`); err == nil {
		t.Fatal("borrador sin listo_cuando debió fallar")
	}
}

func TestGenerar(t *testing.T) {
	g := generadorFijo{texto: `[{"titulo":"a","listo_cuando":"true"}]`}
	bs, err := Generar(context.Background(), g, "haz a")
	if err != nil || len(bs) != 1 {
		t.Fatalf("Generar = %v, %v", bs, err)
	}
}
