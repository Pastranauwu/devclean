package ui

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// Los dos modos son excluyentes: --json no debe traer las líneas de texto
// mezcladas con el documento, o la salida deja de ser parseable.
func TestPlainEscribeLineasYNoDocumento(t *testing.T) {
	var b bytes.Buffer
	p := New(&b, false)
	if p.JSON() {
		t.Error("JSON() dijo true en modo plano")
	}
	p.Line("tarea %s · %s", "T-001", "lista")
	if err := p.Data(map[string]string{"id": "T-001"}); err != nil {
		t.Fatalf("Data: %v", err)
	}
	got := b.String()
	if got != "tarea T-001 · lista\n" {
		t.Errorf("salida plana = %q", got)
	}
}

func TestJSONEscribeDocumentoYCallaLasLineas(t *testing.T) {
	var b bytes.Buffer
	p := New(&b, true)
	if !p.JSON() {
		t.Error("JSON() dijo false en modo json")
	}
	p.Line("esto no debe salir")
	if err := p.Data(map[string]string{"id": "T-001"}); err != nil {
		t.Fatalf("Data: %v", err)
	}
	got := b.String()
	if strings.Contains(got, "esto no debe salir") {
		t.Errorf("una línea de texto se coló en la salida json: %q", got)
	}
	var v map[string]string
	if err := json.Unmarshal([]byte(got), &v); err != nil {
		t.Fatalf("la salida json no parsea (%v): %q", err, got)
	}
	if v["id"] != "T-001" {
		t.Errorf("Data = %v", v)
	}
}
