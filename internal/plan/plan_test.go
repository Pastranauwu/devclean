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

func TestAcotarLimiteLineas(t *testing.T) {
	// el modelo no lo estimó: se usa el default del proyecto
	if got := AcotarLimiteLineas(0, 200); got != 200 {
		t.Errorf("sin propuesta = %d, quiero 200", got)
	}
	if got := AcotarLimiteLineas(-50, 200); got != 200 {
		t.Errorf("negativo = %d, quiero 200", got)
	}
	// una estimación razonable manda sobre el default
	if got := AcotarLimiteLineas(1200, 200); got != 1200 {
		t.Errorf("propuesto = %d, quiero 1200 · el planificador decide", got)
	}
	// absurdos acotados por los dos lados
	if got := AcotarLimiteLineas(3, 200); got != LimiteLineasMin {
		t.Errorf("minimo = %d, quiero %d", got, LimiteLineasMin)
	}
	if got := AcotarLimiteLineas(999999, 200); got != LimiteLineasMax {
		t.Errorf("maximo = %d, quiero %d", got, LimiteLineasMax)
	}
}

// El prompt tiene que pedir el presupuesto, o el modelo nunca lo manda y
// todas las tareas vuelven a caer en la constante.
func TestPromptPideLimiteLineas(t *testing.T) {
	p := Prompt("lo que sea", Contexto{Lenguaje: "go"})
	if !strings.Contains(p, `"limite_lineas"`) {
		t.Error("el prompt no pide limite_lineas")
	}
	// y el ejemplo debe traerlo, que es de donde el modelo copia el formato
	if !strings.Contains(p, `"limite_lineas": 250`) {
		t.Error("el ejemplo del prompt no incluye limite_lineas")
	}
}

func TestParseLeeLimiteLineas(t *testing.T) {
	bs, err := Parse(`[{"titulo":"x","listo_cuando":"go test ./...","limite_lineas":1200}]`)
	if err != nil {
		t.Fatal(err)
	}
	if bs[0].LimiteLineas != 1200 {
		t.Errorf("limite_lineas = %d, quiero 1200", bs[0].LimiteLineas)
	}
}

// El prompt tiene que exigir que listo_cuando falle hoy. Sin eso el
// planificador copia el comando de pruebas del proyecto (`npm test`,
// `go test ./...`), que en un repo con la suite verde pasa de entrada, y
// la esclusa de entrada rechaza TODAS las tareas antes de gastar un token.
func TestPromptExigeQueListoCuandoFalleHoy(t *testing.T) {
	p := Prompt("lo que sea", Contexto{Lenguaje: "node", Pruebas: "npm test"})
	for _, quiero := range []string{"HOY FALLE", "rechaza la tarea si ya pasa", "NO copies ese comando tal cual"} {
		if !strings.Contains(p, quiero) {
			t.Errorf("el prompt no dice %q", quiero)
		}
	}
	// y da ejemplos acotados de cada stack, que es de donde el modelo copia
	for _, ej := range []string{"node --test test/", "pytest tests/", "go test ./internal/"} {
		if !strings.Contains(p, ej) {
			t.Errorf("el prompt no da el ejemplo %q", ej)
		}
	}
}

// En greenfield el aviso ya estaba; que no se pierda.
func TestPromptGreenfieldSigueExigiendoloTambien(t *testing.T) {
	p := Prompt("algo", Contexto{EsVacio: true})
	if !strings.Contains(p, "HOY FALLE") {
		t.Error("el requisito vale igual en un repo vacío")
	}
}

// Sin examinador ciego la tarea escribe sus propias pruebas, así que su
// archivo tiene que entrar en tocar_solo. Sin decírselo, el planificador
// apunta el listo_cuando a un archivo que deja fuera de alcance y que
// nadie puede crear: la tarea agota los intentos con "Could not find".
func TestPromptPideLasPruebasEnAlcanceSinExaminador(t *testing.T) {
	con := Prompt("algo", Contexto{Lenguaje: "node", PruebasPropias: true})
	if !strings.Contains(con, "ESE archivo tiene que estar también en \"tocar_solo\"") {
		t.Error("el prompt no pide meter el archivo de prueba en tocar_solo")
	}
	// con examinador la regla es la contraria y no debe aparecer
	sin := Prompt("algo", Contexto{Lenguaje: "go"})
	if strings.Contains(sin, "las pruebas las escribe la propia tarea") {
		t.Error("con examinador ciego el implementador NO escribe las pruebas")
	}
}
