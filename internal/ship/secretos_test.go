package ship

import (
	"strings"
	"testing"
)

func diffDe(archivo string, lineas ...string) string {
	d := "--- /dev/null\n+++ b/" + archivo + "\n"
	for _, l := range lineas {
		d += "+" + l + "\n"
	}
	return d
}

// Las credenciales de prueba se arman en tiempo de ejecución a propósito.
// Escritas como literal son válidas en formato, y la protección de push de
// GitHub rechaza el repo entero — cosa que este mismo escáner debería haber
// hecho antes, que es de lo que trata este archivo.
var (
	claveAnthropic = "sk-" + "ant-api03-" + strings.Repeat("AbCd1234", 6)
	claveOpenAI    = "sk-" + "proj-" + strings.Repeat("AbCd1234", 4)
	claveStripe    = "sk_" + "live_" + strings.Repeat("a", 24)
	tokenHF        = "hf_" + strings.Repeat("AbCd1234", 4)
	claveAWS       = "AKIA" + "IOSFODNN7EXAMPLE"
)

// Un .env no lleva comillas, que es justo donde viven las credenciales.
func TestSecretosEnEnvSinComillas(t *testing.T) {
	h := escanearSecretos(diffDe(".env",
		"ANTHROPIC_API_KEY="+claveAnthropic,
		"OPENAI_API_KEY="+claveOpenAI,
		"DB_PASSWORD=sup3rs3cr3t0-produccion",
		`STRIPE_SECRET="`+claveStripe+`"`,
	))
	if len(h) != 4 {
		t.Fatalf("se escaparon %d de 4 credenciales: %+v", 4-len(h), h)
	}
}

// el arquitecto escribe .env.example y la esclusa no debe frenar la
// entrega por sus valores de relleno
func TestSecretosDejanPasarElRellenoDeUnaPlantilla(t *testing.T) {
	relleno := []string{"SECRET_KEY=change-me-before-deploying", "DB_PASSWORD=app_local"}
	if h := escanearSecretos(diffDe(".env.example", relleno...)); len(h) != 0 {
		t.Fatalf("relleno de plantilla frenado: %+v", h)
	}
	if h := escanearSecretos(diffDe(".env", relleno...)); len(h) != 2 {
		t.Fatalf("en un .env real son 2 credenciales, hubo %d", len(h))
	}
	if h := escanearSecretos(diffDe(".env.example", "ANTHROPIC_API_KEY="+claveAnthropic)); len(h) != 1 {
		t.Fatalf("una clave real en la plantilla se escapó: %+v", h)
	}
}

func TestSecretosPorProveedor(t *testing.T) {
	casos := map[string]string{
		"clave Anthropic":    "key = " + claveAnthropic,
		"clave OpenAI":       "key = " + claveOpenAI,
		"token Hugging Face": "HF_TOKEN=" + tokenHF,
		"clave AWS":          "AWS_ACCESS_KEY_ID=" + claveAWS,
	}
	for quiero, linea := range casos {
		h := escanearSecretos(diffDe("x.env", linea))
		if len(h) == 0 {
			t.Errorf("%s: no detectada en %q", quiero, linea)
		}
	}
}

// Lo que importa tanto como detectar: no frenar un PR válido.
func TestSecretosSinFalsosPositivos(t *testing.T) {
	limpias := []string{
		`password = os.Getenv("DB_PASSWORD")`,
		`token := client.FetchToken(ctx)`,
		"DB_PASSWORD=$POSTGRES_PASSWORD",
		"API_KEY=${SECRETS_API_KEY}",
		"api_key: <pon-aqui-tu-clave>",
		"Password string `json:\"password\"`",
		`ANTHROPIC_API_KEY=your-key-here`,
		`token: changeme123456`,
		"const password = process.env.PASSWORD",
		"secret: redacted-por-seguridad",
	}
	for _, l := range limpias {
		if h := escanearSecretos(diffDe("x.go", l)); len(h) != 0 {
			t.Errorf("falso positivo en %q: %+v", l, h)
		}
	}
}

// La documentación muestra código a propósito; marcarlo frena un PR correcto.
func TestRuidoNoFrenaDocumentacion(t *testing.T) {
	docs := []string{"README.md", "docs/guia.md", "doc/api.rst", "examples/uso.md", "CHANGELOG.txt"}
	for _, f := range docs {
		h := escanearRuido(diffDe(f, "console.log(cliente.estado)", "print(resultado)"), nil)
		if len(h) != 0 {
			t.Errorf("%s: documentación marcada como ruido: %+v", f, h)
		}
	}
}

// Pero en código sí tiene que seguir avisando.
func TestRuidoSigueAvisandoEnCodigo(t *testing.T) {
	h := escanearRuido(diffDe("src/cliente.js", "console.log(cliente.estado)"), nil)
	if len(h) != 1 || h[0].Tipo != "print de debug" {
		t.Errorf("print de debug en código no detectado: %+v", h)
	}
	// un archivo temporal sigue siendo ruido aunque sea .md
	if h := escanearRuido("", []string{"notas.md.bak"}); len(h) != 1 {
		t.Errorf("archivo temporal no detectado: %+v", h)
	}
}

// Reportar un error o arrancar un servidor desde un script no es debug.
func TestRuidoNoFrenaErroresNiScripts(t *testing.T) {
	if h := escanearRuido(diffDe("src/app/app.js", "console.error(error);", "console.warn('sin audio')"), nil); len(h) != 0 {
		t.Errorf("reporte de errores marcado como ruido: %+v", h)
	}
	if h := escanearRuido(diffDe("scripts/serve.js", "server.listen(PORT, () => console.log(`en http://localhost:${PORT}`))"), nil); len(h) != 0 {
		t.Errorf("salida de un script marcada como ruido: %+v", h)
	}
}

// Una oración partida en dos líneas cierra en ";" y no es código.
func TestComentarioEnProsaConPuntoYComa(t *testing.T) {
	prosa := "// No repite lo que ya cubren las suites por pieza (engine.test.js, store.test.js, etc.);"
	if esCodigoComentado(prosa) {
		t.Errorf("prosa marcada como código: %q", prosa)
	}
	for _, c := range []string{"// foo(bar);", "// const x = 1;", "// return y;", "# total += 1;"} {
		if !esCodigoComentado(c) {
			t.Errorf("código comentado no detectado: %q", c)
		}
	}
}

// el esqueleto se frenó por pasar la key de settings a un cliente
// y por la clave falsa de una prueba
func TestSecretosNoConfundenVariablesNiFixtures(t *testing.T) {
	limpias := []struct{ archivo, linea string }{
		{"backend/app/config.py", "            typesafe_api_key=typesafe_key,"},
		{"backend/app/container.py", "        sdk = TypeSafeClient(api_key=settings.typesafe_api_key, timeout=5)"},
		{"backend/tests/adapters/test_jev_clients.py", `    return OpenRouterJevClient(http, api_key="sk-or-secret")`},
	}
	for _, c := range limpias {
		if h := escanearSecretos(diffDe(c.archivo, c.linea)); len(h) != 0 {
			t.Errorf("%s: falso positivo %v", c.archivo, h)
		}
	}
	sucias := []struct{ archivo, linea string }{
		{"backend/app/config.py", `API_KEY = "sk-or-v1-9f8e7d6c5b4a"`},
		{"backend/.env", "DB_PASSWORD=hunter2-2024"},
		{"backend/tests/test_x.py", `KEY = "sk-ant-api03-abcdefghijklmnopqrstuvwxyz"`},
	}
	for _, c := range sucias {
		if h := escanearSecretos(diffDe(c.archivo, c.linea)); len(h) == 0 {
			t.Errorf("%s: no vio %q", c.archivo, c.linea)
		}
	}
}
