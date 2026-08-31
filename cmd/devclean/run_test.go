package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Pastranauwu/devclean/internal/config"
	"github.com/Pastranauwu/devclean/internal/gate"
	"github.com/Pastranauwu/devclean/internal/state"
	"github.com/Pastranauwu/devclean/internal/task"
	"github.com/Pastranauwu/devclean/internal/ui"
)

func tarea(id string, alcance []string) task.Task {
	return task.Task{ID: id, Titulo: id, TocarSolo: alcance}
}

func TestAsignarUnaSola(t *testing.T) {
	aceptadas, rechazos := asignar([]task.Task{tarea("T-001", nil)})
	if len(aceptadas) != 1 || len(rechazos) != 0 {
		t.Fatalf("aceptadas=%v rechazos=%v, quiero una sola aceptada", aceptadas, rechazos)
	}
}

func TestAsignarVacioConMasDeUna(t *testing.T) {
	aceptadas, rechazos := asignar([]task.Task{
		tarea("T-001", []string{"src/export/**"}),
		tarea("T-002", nil),
	})
	if len(aceptadas) != 1 || aceptadas[0].ID != "T-001" {
		t.Fatalf("aceptadas=%v, quiero solo T-001", aceptadas)
	}
	if len(rechazos) != 1 || rechazos[0].ID != "T-002" {
		t.Fatalf("rechazos=%v, quiero T-002 por vacío", rechazos)
	}
}

func TestAsignarCruce(t *testing.T) {
	aceptadas, rechazos := asignar([]task.Task{
		tarea("T-001", []string{"src/**"}),
		tarea("T-002", []string{"src/export/**"}),
		tarea("T-003", []string{"docs/**"}),
	})
	ids := map[string]bool{}
	for _, a := range aceptadas {
		ids[a.ID] = true
	}
	if len(aceptadas) != 2 || !ids["T-001"] || !ids["T-003"] {
		t.Fatalf("aceptadas=%v, quiero T-001 y T-003", aceptadas)
	}
	if len(rechazos) != 1 || rechazos[0].ID != "T-002" {
		t.Fatalf("rechazos=%v, quiero T-002 por cruce", rechazos)
	}
}

func TestDepsVerdes(t *testing.T) {
	verde := map[string]bool{"T-001": true}
	if !depsVerdes(nil, verde) {
		t.Error("sin dependencias debió ser verde")
	}
	if !depsVerdes([]string{"T-001"}, verde) {
		t.Error("dependencia ya verde debió pasar")
	}
	if depsVerdes([]string{"T-002"}, verde) {
		t.Error("dependencia pendiente no debió pasar")
	}
	if got := depsFaltantes([]string{"T-001", "T-003"}, verde); len(got) != 1 || got[0] != "T-003" {
		t.Errorf("depsFaltantes = %v, quiero [T-003]", got)
	}
}

// Una dependencia que quedó verde en una corrida anterior no debe
// bloquear a la que la necesita: antes `integrada` arrancaba vacío en
// cada corrida y la tarea se rechazaba con "no salió verde" para
// siempre.
func TestSembrarVerdesPrevias(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".devclean", "state"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := state.Save(root, state.State{ID: "T-001", Estado: state.Lista}); err != nil {
		t.Fatal(err)
	}
	if err := state.Save(root, state.State{ID: "T-002", Estado: state.Detenida}); err != nil {
		t.Fatal(err)
	}

	nueva := task.Task{ID: "T-009", Titulo: "T-009", DependeDe: []string{"T-001", "T-002"}}
	integrada := map[string]bool{}
	// sin repo git, RamaExiste es falso: la verde cuenta como ya entregada
	sembrarVerdesPrevias(context.Background(), root, []task.Task{nueva}, integrada)

	if !integrada["T-001"] {
		t.Error("T-001 quedó lista en una corrida anterior, debió contar como verde")
	}
	if integrada["T-002"] {
		t.Error("T-002 está detenida, no debió contar como verde")
	}
}

func TestModeloParaTarea(t *testing.T) {
	cfg := config.Config{
		Estrategia: "equilibrada",
		Modelos:    map[string]string{"liviana": "glm-4", "media": "glm-5.2", "pesada": "claude-sonnet"},
		Proveedores: map[string]config.Proveedor{
			"ejecutor": {Modelo: "default-ejecutor"},
		},
	}

	// el flag gana sobre todo
	if got := modeloParaTarea(cfg, "flag-model", task.Task{Peso: "pesada"}); got != "flag-model" {
		t.Errorf("flag = %q, quiero flag-model", got)
	}
	// peso explícito
	if got := modeloParaTarea(cfg, "", task.Task{Peso: "pesada"}); got != "claude-sonnet" {
		t.Errorf("peso pesada = %q, quiero claude-sonnet", got)
	}
	// sin peso, usa la estrategia
	if got := modeloParaTarea(cfg, "", task.Task{}); got != "glm-5.2" {
		t.Errorf("estrategia equilibrada = %q, quiero glm-5.2", got)
	}
	// peso sin modelo mapeado cae al ejecutor
	cfg2 := config.Config{Modelos: map[string]string{"media": "glm-5.2"}, Proveedores: map[string]config.Proveedor{"ejecutor": {Modelo: "default"}}}
	if got := modeloParaTarea(cfg2, "", task.Task{Peso: "pesada"}); got != "default" {
		t.Errorf("peso sin mapeo = %q, quiero default", got)
	}
}

func TestMotivoRechazoUneTodosLosChequeos(t *testing.T) {
	res := gate.Result{
		ID: "T-001",
		Chequeos: []gate.Check{
			{Nombre: "contrato válido", OK: true},
			{Nombre: "ejecutable", OK: false, Motivo: "falta listo_cuando · escribe el comando que dice \"ya está\""},
			{Nombre: "zonas prohibidas", OK: false, Motivo: "tocar_solo incluye go.sum"},
		},
	}
	got := motivoRechazo(res)
	if !strings.Contains(got, "ejecutable") || !strings.Contains(got, "falta listo_cuando") {
		t.Errorf("falta el primer chequeo fallido: %s", got)
	}
	if !strings.Contains(got, "zonas prohibidas") || !strings.Contains(got, "go.sum") {
		t.Errorf("falta el segundo chequeo fallido: %s", got)
	}
	if strings.Contains(got, "contrato válido") {
		t.Errorf("no debe incluir chequeos en verde: %s", got)
	}
}

func TestMotivoRechazoVacioCaeAPrimerMotivo(t *testing.T) {
	res := gate.Result{ID: "T-001"}
	if got := motivoRechazo(res); got != "" {
		t.Errorf("sin chequeos = %q, quiere vacío", got)
	}
}

func TestEmitirResultadosRechazadaImprimeMotivoCompleto(t *testing.T) {
	var b strings.Builder
	out = ui.New(&b, false)
	if err := emitirResultados([]runResult{{
		ID: "T-001", Titulo: "exportar", Estado: "rechazada",
		Motivo: "ejecutable · falta listo_cuando · zonas prohibidas · tocar_solo incluye go.sum",
	}}); err != nil {
		t.Fatal(err)
	}
	got := b.String()
	if !strings.Contains(got, "T-001") || !strings.Contains(got, "falta listo_cuando") || !strings.Contains(got, "go.sum") {
		t.Errorf("plain no imprimió el motivo completo:\n%s", got)
	}
}

func TestRechazarUsaHuerfano(t *testing.T) {
	productora := task.Task{ID: "T-001", Titulo: "wol", Expone: []string{"wol.Send(mac, addr string) error"}}
	buena := task.Task{ID: "T-002", Titulo: "api", Usa: []string{"wol.Send(mac, addr string) error"}}
	huerfana := task.Task{ID: "T-003", Titulo: "cli", Usa: []string{"config.Cargar(p string) error"}}

	todas := []task.Task{productora, buena, huerfana}
	ok, results := rechazarUsaHuerfano([]task.Task{buena, huerfana}, todas, nil)

	if len(ok) != 1 || ok[0].ID != "T-002" {
		t.Fatalf("aprobadas = %v, quiero solo T-002", ok)
	}
	if len(results) != 1 || results[0].ID != "T-003" || results[0].Estado != "rechazada" {
		t.Fatalf("results = %+v, quiero T-003 rechazada", results)
	}
}
