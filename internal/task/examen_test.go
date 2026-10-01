package task

import "testing"

func TestExamenEsqueletoViajaEnContrato(t *testing.T) {
	in := Task{Version: Version, ID: "T-001", Titulo: "doblar", ListoCuando: "node --test tests/doble.test.js", LimiteIntentos: 3, ExamenEsqueleto: true, ExamenVisible: "tests/doble.test.js"}
	out, err := Parse(in.Marshal())
	if err != nil {
		t.Fatal(err)
	}
	if !out.ExamenEsqueleto || out.ExamenVisible != in.ExamenVisible || len(out.Validate()) != 0 {
		t.Fatalf("examen perdido: %+v, errores %v", out, out.Validate())
	}
}
