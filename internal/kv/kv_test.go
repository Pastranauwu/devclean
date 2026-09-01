package kv

import (
	"strings"
	"testing"
)

func TestStripComment(t *testing.T) {
	casos := map[string]string{
		"base: main # la rama":  "base: main ",
		`pruebas: "go test #x"`: `pruebas: "go test #x"`,
		"# todo comentario":     "",
		"base: main#no":         "base: main#no",
	}
	for entrada, quiere := range casos {
		if got := StripComment(entrada); got != quiere {
			t.Errorf("StripComment(%q) = %q, quiere %q", entrada, got, quiere)
		}
	}
}

func TestUnquote(t *testing.T) {
	casos := map[string]string{
		`"go test ./..."`: "go test ./...",
		"go test ./...":   "go test ./...",
		`"sin cierre`:     `"sin cierre`,
		`"a\"b"`:          `a"b`,
	}
	for entrada, quiere := range casos {
		if got := Unquote(entrada); got != quiere {
			t.Errorf("Unquote(%q) = %q, quiere %q", entrada, got, quiere)
		}
	}
}

func TestParseList(t *testing.T) {
	got, err := ParseList(`["src/**", "docs/a.md"]`)
	if err != nil {
		t.Fatalf("ParseList: %v", err)
	}
	if len(got) != 2 || got[0] != "src/**" || got[1] != "docs/a.md" {
		t.Errorf("ParseList = %v", got)
	}

	if l, err := ParseList(""); err != nil || l != nil {
		t.Errorf("ParseList(\"\") = %v, %v · quiere nil sin error", l, err)
	}
	if l, err := ParseList("[]"); err != nil || l == nil || len(l) != 0 {
		t.Errorf("ParseList(\"[]\") = %v, %v · quiere lista vacía", l, err)
	}
	for _, malo := range []string{"src/**", `[src/**]`, `["a", b]`} {
		if _, err := ParseList(malo); err == nil {
			t.Errorf("ParseList(%q) aceptó una lista mal formada", malo)
		}
	}
}

func TestMarshalListIdaYVuelta(t *testing.T) {
	quiere := []string{"src/**", `raro "con" comillas`}
	got, err := ParseList(MarshalList(quiere))
	if err != nil {
		t.Fatalf("ParseList: %v", err)
	}
	if len(got) != 2 || got[0] != quiere[0] || got[1] != quiere[1] {
		t.Errorf("ida y vuelta = %v, quiere %v", got, quiere)
	}
}

func TestParseInt(t *testing.T) {
	if n, err := ParseInt("3"); err != nil || n != 3 {
		t.Errorf("ParseInt(\"3\") = %d, %v", n, err)
	}
	if _, err := ParseInt("tres"); err == nil {
		t.Error("ParseInt aceptó un entero inválido")
	}
}

func TestPairs(t *testing.T) {
	doc := "base: main\n\n# comentario\npruebas: go test ./...   # detectado\n"
	pares, err := Pairs(strings.Split(doc, "\n"), 1)
	if err != nil {
		t.Fatalf("Pairs: %v", err)
	}
	if len(pares) != 2 {
		t.Fatalf("Pairs = %v · quiere 2 pares", pares)
	}
	if pares[0].Key != "base" || pares[0].Value != "main" || pares[0].Line != 1 {
		t.Errorf("primer par = %+v", pares[0])
	}
	if pares[1].Key != "pruebas" || pares[1].Value != "go test ./..." || pares[1].Line != 4 {
		t.Errorf("segundo par = %+v", pares[1])
	}
}

func TestPairsLineaSinDosPuntos(t *testing.T) {
	_, err := Pairs([]string{"base main"}, 1)
	if err == nil || !strings.Contains(err.Error(), "línea 1") {
		t.Errorf("Pairs error = %v · quiere señalar la línea 1", err)
	}
}

func TestParseInlineMap(t *testing.T) {
	got, err := ParseInlineMap(`{ modelo: claude-sonnet, key_env: ANTHROPIC_API_KEY }`)
	if err != nil {
		t.Fatalf("ParseInlineMap: %v", err)
	}
	if got["modelo"] != "claude-sonnet" || got["key_env"] != "ANTHROPIC_API_KEY" {
		t.Errorf("mapa = %v", got)
	}

	if m, err := ParseInlineMap("{}"); err != nil || len(m) != 0 {
		t.Errorf("ParseInlineMap(\"{}\") = %v, %v · quiere mapa vacío", m, err)
	}
	for _, malo := range []string{"modelo: x", "{modelo}", "{"} {
		if _, err := ParseInlineMap(malo); err == nil {
			t.Errorf("ParseInlineMap(%q) aceptó un mapa mal formado", malo)
		}
	}

	conLista, err := ParseInlineMap(`{ provider: claude, model: claude-sonnet, skills: ["diseno", "arquitectura"] }`)
	if err != nil {
		t.Fatalf("ParseInlineMap con lista: %v", err)
	}
	if conLista["provider"] != "claude" || conLista["model"] != "claude-sonnet" || conLista["skills"] != `["diseno", "arquitectura"]` {
		t.Errorf("mapa con lista = %v", conLista)
	}
}

func TestNested(t *testing.T) {
	doc := strings.Split("base: main\n\nproveedores:\n  planificador: { modelo: a }\n  ejecutor: { modelo: b }\nzonas: [\"x\"]\n", "\n")
	children, err := Nested(doc, "proveedores", 1)
	if err != nil {
		t.Fatalf("Nested: %v", err)
	}
	if len(children) != 2 {
		t.Fatalf("Nested = %v · quiere 2 hijos", children)
	}
	if children[0].Key != "planificador" || children[0].Value != "{ modelo: a }" || children[0].Line != 4 {
		t.Errorf("primer hijo = %+v", children[0])
	}
	if children[1].Key != "ejecutor" {
		t.Errorf("segundo hijo = %+v", children[1])
	}
}

func TestNestedAusente(t *testing.T) {
	children, err := Nested([]string{"base: main"}, "proveedores", 1)
	if err != nil {
		t.Fatalf("Nested: %v", err)
	}
	if children != nil {
		t.Errorf("Nested sin bloque = %v, quiere nil", children)
	}
}

func TestParseListComaDentroDeComillas(t *testing.T) {
	got, err := ParseList(`["wol.Send(mac, addr string) error", "POST /wake"]`)
	if err != nil {
		t.Fatalf("ParseList: %v", err)
	}
	want := []string{"wol.Send(mac, addr string) error", "POST /wake"}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("ParseList = %q, quiero %q", got, want)
	}
}
