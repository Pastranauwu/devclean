package ship

import (
	"strings"
	"testing"
)

func TestEscanearRuidoDebugYTemporal(t *testing.T) {
	diff := `diff --git a/src/x.go b/src/x.go
--- a/src/x.go
+++ b/src/x.go
@@ -0,0 +1,3 @@
+package x
+import "fmt"
+	fmt.Println("hola")
`
	h := escanearRuido(diff, []string{"src/x.go", "notas.tmp"})
	if len(h) != 2 {
		t.Fatalf("hallazgos = %+v, quiero 2", h)
	}
	tipos := map[string]bool{}
	for _, x := range h {
		tipos[x.Tipo] = true
	}
	if !tipos["print de debug"] || !tipos["archivo temporal"] {
		t.Errorf("tipos = %v", tipos)
	}
}

func TestEscanearRuidoCodigoComentado(t *testing.T) {
	diff := `--- a/x.go
+++ b/x.go
+// x = y;
+// TODO: revisar esto
+// esto es una nota narrativa
`
	h := escanearRuido(diff, nil)
	if len(h) != 1 {
		t.Fatalf("hallazgos = %+v, quiero solo el código comentado", h)
	}
	if h[0].Tipo != "código comentado" {
		t.Errorf("tipo = %q", h[0].Tipo)
	}
}

func TestEscanearRuidoLimpio(t *testing.T) {
	diff := `--- a/x.go
+++ b/x.go
+package x
+// Config guarda opciones.
+type Config struct{}
`
	if h := escanearRuido(diff, []string{"x.go"}); len(h) != 0 {
		t.Errorf("diff limpio marcado: %+v", h)
	}
}

func TestEscanearSecretos(t *testing.T) {
	diff := `--- a/x.go
+++ b/x.go
+const awsKey = "AKIAIOSFODNN7EXAMPLE"
+password = "hunter2secret"
+func normal() {}
`
	h := escanearSecretos(diff)
	if len(h) != 2 {
		t.Fatalf("hallazgos = %+v, quiero 2", h)
	}
	if h[0].Tipo != "clave AWS" {
		t.Errorf("primer hallazgo = %q", h[0].Tipo)
	}
}

func TestEscanearSecretosSinSecretos(t *testing.T) {
	diff := `--- a/x.go
+++ b/x.go
+token := os.Getenv("TOKEN")
+const version = "1.2.3"
`
	if h := escanearSecretos(diff); len(h) != 0 {
		t.Errorf("sin secretos pero marcó: %+v", h)
	}
}

func TestTipoCommit(t *testing.T) {
	if tipoCommit("exportar clientes a CSV") != "feat" {
		t.Error("título de feature debió ser feat")
	}
	for _, titulo := range []string{"arreglar el login", "fix del bug de tildes", "corrige la falla"} {
		if tipoCommit(titulo) != "fix" {
			t.Errorf("tipoCommit(%q) = %q, quiero fix", titulo, tipoCommit(titulo))
		}
	}
}

func TestVerificarPresupuesto(t *testing.T) {
	tk := taskTitulo("x")
	tk.LimiteLineas = 200

	d, ok := verificarPresupuesto(118, 12, 3, tk)
	if !ok || !strings.Contains(d, "118") || !strings.Contains(d, "200") {
		t.Errorf("dentro de presupuesto: %q, %v", d, ok)
	}
	if _, ok := verificarPresupuesto(201, 0, 1, tk); ok {
		t.Error("exceder limite_lineas debió fallar")
	}
}

func TestGenerarHandoff(t *testing.T) {
	tk := taskTitulo("exportar")
	tk.ListoCuando = "go test ./..."
	tk.NoTocar = []string{"migrations/**"}
	tk.Riesgos = "archivos grandes"
	cuerpo := generarHandoff(tk, []string{"src/export/writer.go"}, 10, 2)
	for _, want := range []string{
		"## Qué cambió",
		"src/export/writer.go",
		"## Qué no se hizo",
		"migrations/**",
		"archivos grandes",
		"## Cómo verificar",
		"go test ./...",
	} {
		if !strings.Contains(cuerpo, want) {
			t.Errorf("handoff sin %q:\n%s", want, cuerpo)
		}
	}
}
