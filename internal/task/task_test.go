package task

import (
	"reflect"
	"strings"
	"testing"
)

const contractEjemplo = `---
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
		"campo desconocido":     "---\nid: T-001\nextra: nope\n---\n",
		"entero inválido":       "---\nlimite_intentos: tres\n---\n",
		"lista mal formada":     "---\ntocar_solo: src/**\n---\n",
		"elemento sin comillas": "---\ntocar_solo: [src/**]\n---\n",
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
	if errs := varios.Validate(); len(errs) != 5 {
		t.Errorf("contrato vacío: %d errores, quiero 5: %v", len(errs), errs)
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
	task := Task{ID: "T-001", Titulo: "x", ListoCuando: "true", LimiteIntentos: 3, LimiteLineas: 200}
	out := string(task.Marshal())
	if strings.Contains(out, "porque") || strings.Contains(out, "riesgos") {
		t.Errorf("Marshal incluyó campos vacíos:\n%s", out)
	}
	if !strings.Contains(out, "tocar_solo: []") {
		t.Errorf("Marshal debió escribir listas vacías:\n%s", out)
	}
}
