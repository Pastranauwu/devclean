package task

import (
	"reflect"
	"strings"
	"testing"
)

const contractEjemplo = `---
version: 1
id: T-001
titulo: exportar clientes a CSV
porque: soporte pierde 3h/semana copiando a mano
listo_cuando: npm test -- export.spec.ts     # OBLIGATORIO, ejecutable
tocar_solo: ["src/export/**"]
no_tocar: ["src/auth/**", "migrations/**"]
limite_intentos: 3
limite_lineas: 200
riesgos: archivos grandes pueden agotar memoria
---
Notas libres opcionales.
`

func TestParseContratoCompleto(t *testing.T) {
	task, err := Parse([]byte(contractEjemplo))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	want := Task{
		Version:        1,
		ID:             "T-001",
		Titulo:         "exportar clientes a CSV",
		Porque:         "soporte pierde 3h/semana copiando a mano",
		ListoCuando:    "npm test -- export.spec.ts",
		TocarSolo:      []string{"src/export/**"},
		NoTocar:        []string{"src/auth/**", "migrations/**"},
		LimiteIntentos: 3,
		LimiteLineas:   200,
		Riesgos:        "archivos grandes pueden agotar memoria",
		Notas:          "Notas libres opcionales.",
	}
	if !reflect.DeepEqual(task, want) {
		t.Errorf("Parse = %+v\nquiero %+v", task, want)
	}
}

func TestParseErrores(t *testing.T) {
	cases := map[string]string{
		"sin bloque inicial":    "id: T-001\n",
		"sin bloque de cierre":  "---\nid: T-001\n",
		"campo desconocido":     "---\nversion: 1\nid: T-001\nextra: nope\n---\n",
		"entero inválido":       "---\nversion: 1\nlimite_intentos: tres\n---\n",
		"lista mal formada":     "---\nversion: 1\ntocar_solo: src/**\n---\n",
		"elemento sin comillas": "---\nversion: 1\ntocar_solo: [src/**]\n---\n",
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse([]byte(input)); err == nil {
				t.Errorf("Parse debió fallar: %s", name)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	valida := Task{
		Version:        Version,
		ID:             "T-001",
		Titulo:         "algo",
		ListoCuando:    "make test",
		LimiteIntentos: 3,
		LimiteLineas:   200,
	}
	if errs := valida.Validate(); len(errs) != 0 {
		t.Errorf("contrato válido rechazado: %v", errs)
	}

	sinListo := valida
	sinListo.ListoCuando = ""
	errs := sinListo.Validate()
	if len(errs) != 1 || !strings.Contains(errs[0].Error(), "listo_cuando") {
		t.Errorf("Validate sin listo_cuando = %v", errs)
	}

	varios := Task{}
	if errs := varios.Validate(); len(errs) != 6 {
		t.Errorf("contrato vacío: %d errores, quiero 6: %v", len(errs), errs)
	}
}

func TestMarshalRoundtrip(t *testing.T) {
	original, err := Parse([]byte(contractEjemplo))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	reparsed, err := Parse(original.Marshal())
	if err != nil {
		t.Fatalf("reparse: %v", err)
	}
	if !reflect.DeepEqual(original, reparsed) {
		t.Errorf("roundtrip = %+v\nquiero %+v", reparsed, original)
	}
}

func TestMarshalOmiteVacios(t *testing.T) {
	task := Task{Version: Version, ID: "T-001", Titulo: "x", ListoCuando: "true", LimiteIntentos: 3, LimiteLineas: 200}
	out := string(task.Marshal())
	if strings.Contains(out, "porque") || strings.Contains(out, "riesgos") {
		t.Errorf("Marshal incluyó campos vacíos:\n%s", out)
	}
	if !strings.Contains(out, "tocar_solo: []") {
		t.Errorf("Marshal debió escribir listas vacías:\n%s", out)
	}
}

func TestParseSinVersion(t *testing.T) {
	task, err := Parse([]byte("---\nid: T-001\ntitulo: x\n---\n"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	errs := task.Validate()
	if len(errs) == 0 || !strings.Contains(errs[0].Error(), "falta version") {
		t.Errorf("Validate sin version = %v · quiere exigir version", errs)
	}
}

func TestParseVersionInvalida(t *testing.T) {
	for _, v := range []string{"cero", "0", "-1"} {
		if _, err := Parse([]byte("---\nversion: " + v + "\nid: T-001\n---\n")); err == nil {
			t.Errorf("Parse aceptó version: %s", v)
		}
	}
}

func TestParseVersionFuturaTolera(t *testing.T) {
	task, err := Parse([]byte("---\nversion: 2\nid: T-001\ntitulo: x\nlisto_cuando: make test\ncampo_del_futuro: algo\n---\n"))
	if err != nil {
		t.Fatalf("un contrato de versión futura debió parsear: %v", err)
	}
	if task.Aviso != "contrato versión 2, binario soporta 1 · actualiza devclean" {
		t.Errorf("Aviso = %q", task.Aviso)
	}
	if task.Titulo != "x" {
		t.Errorf("los campos conocidos debieron leerse: %+v", task)
	}
}

func TestParseCampoDesconocidoEnVersionActual(t *testing.T) {
	_, err := Parse([]byte("---\nversion: 1\nid: T-001\ncampo_del_futuro: algo\n---\n"))
	if err == nil || !strings.Contains(err.Error(), "campo desconocido") {
		t.Errorf("Parse = %v · un campo desconocido en la versión soportada se rechaza", err)
	}
}

func TestParseDependeDe(t *testing.T) {
	task, err := Parse([]byte("---\nversion: 1\nid: T-002\ntitulo: x\nlisto_cuando: true\ndepende_de: [\"T-001\"]\n---\n"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(task.DependeDe) != 1 || task.DependeDe[0] != "T-001" {
		t.Errorf("DependeDe = %v, quiero [T-001]", task.DependeDe)
	}
	if !strings.Contains(string(task.Marshal()), "depende_de: [\"T-001\"]") {
		t.Errorf("Marshal no escribió depende_de:\n%s", task.Marshal())
	}
}

func TestValidateDependeDeInvalida(t *testing.T) {
	task := Task{Version: Version, ID: "T-001", Titulo: "x", ListoCuando: "true", LimiteIntentos: 3, LimiteLineas: 200, DependeDe: []string{"t-2"}}
	errs := task.Validate()
	if len(errs) != 1 || !strings.Contains(errs[0].Error(), "depende_de inválido") {
		t.Errorf("Validate depende_de mal formada = %v", errs)
	}

	ciclo := task
	ciclo.DependeDe = []string{"T-001"}
	if errs := ciclo.Validate(); len(errs) != 1 || !strings.Contains(errs[0].Error(), "sí misma") {
		t.Errorf("Validate dependencia propia = %v", errs)
	}
}

func TestPeso(t *testing.T) {
	task, err := Parse([]byte("---\nversion: 1\nid: T-001\ntitulo: x\nlisto_cuando: true\npeso: pesada\n---\n"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if task.Peso != "pesada" {
		t.Errorf("Peso = %q, quiero pesada", task.Peso)
	}
	if !strings.Contains(string(task.Marshal()), "peso: pesada") {
		t.Errorf("Marshal no escribió peso:\n%s", task.Marshal())
	}

	inv := Task{Version: Version, ID: "T-001", Titulo: "x", ListoCuando: "true", LimiteIntentos: 3, LimiteLineas: 200, Peso: "gigante"}
	if errs := inv.Validate(); len(errs) != 1 || !strings.Contains(errs[0].Error(), "peso inválido") {
		t.Errorf("Validate peso inválido = %v", errs)
	}
}
