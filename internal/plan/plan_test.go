package plan

import (
	"context"
	"strings"
	"testing"

	"github.com/Pastranauwu/devclean/internal/config"
)

type generadorFijo struct {
	texto string
	err   error
}

func (g generadorFijo) Generar(_ context.Context, _ string) (string, error) {
	return g.texto, g.err
}

func TestPromptPideJSON(t *testing.T) {
	p := Prompt("exportar clientes", Contexto{Lenguaje: "go", Pruebas: "go test ./..."})
	for _, want := range []string{"exportar clientes", "listo_cuando", "tocar_solo", "array JSON"} {
		if !strings.Contains(p, want) {
			t.Errorf("prompt sin %q", want)
		}
	}
}

func TestPromptInyectaLenguaje(t *testing.T) {
	p := Prompt("exportar", Contexto{Lenguaje: "go", Pruebas: "go test ./..."})
	for _, want := range []string{"go", "go test ./..."} {
		if !strings.Contains(p, want) {
			t.Errorf("prompt sin lenguaje/pruebas %q:\n%s", want, p)
		}
	}
}

func TestPromptRepoVacio(t *testing.T) {
	p := Prompt("crear wakeonlan", Contexto{EsVacio: true})
	for _, want := range []string{"vacío", "stack", "depende_de"} {
		if !strings.Contains(p, want) {
			t.Errorf("prompt greenfield sin %q:\n%s", want, p)
		}
	}
}

func TestPromptIncluyeRequisitosYStack(t *testing.T) {
	p := Prompt("crear wakeonlan", Contexto{EsVacio: true, Stack: "go", Requisitos: "servidor http local"})
	for _, want := range []string{"go", "servidor http local"} {
		if !strings.Contains(p, want) {
			t.Errorf("prompt sin %q:\n%s", want, p)
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
	bs, err := Generar(context.Background(), g, Contexto{}, "haz a")
	if err != nil || len(bs) != 1 {
		t.Fatalf("Generar = %v, %v", bs, err)
	}
}

func TestPromptInyectaAgentes(t *testing.T) {
	ctx := Contexto{
		Lenguaje: "go",
		Agentes: map[string]config.Agente{
			"architect": {Provider: "claude", Skills: []string{"diseno", "arquitectura"}},
			"tester":    {Provider: "claude"},
		},
	}
	p := Prompt("crear módulo", ctx)
	for _, want := range []string{"architect (habilidades: diseno, arquitectura)", "tester", `"agente"`} {
		if !strings.Contains(p, want) {
			t.Errorf("prompt sin %q:\n%s", want, p)
		}
	}
}

func TestParseBorradorConAgente(t *testing.T) {
	bs, err := Parse(`[{"titulo":"diseño","listo_cuando":"true","agente":"architect"}]`)
	if err != nil {
		t.Fatal(err)
	}
	if len(bs) != 1 || bs[0].Agente != "architect" {
		t.Fatalf("bs = %+v", bs)
	}
}
