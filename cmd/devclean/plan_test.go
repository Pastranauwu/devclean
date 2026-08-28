package main

import (
	"os"
	"strings"
	"testing"

	"github.com/Pastranauwu/devclean/internal/config"
	"github.com/Pastranauwu/devclean/internal/plan"
	"github.com/Pastranauwu/devclean/internal/task"
)

func TestSanearAlcance(t *testing.T) {
	zonas, patrones := zonasYPatrones(config.Config{})
	bs := []plan.Borrador{
		{Titulo: "init go", TocarSolo: []string{"go.mod", "go.sum", "Makefile"}},
		{Titulo: "wol", TocarSolo: []string{"internal/wol/**"}},
	}
	sanearAlcance(bs, zonas, patrones)

	if got := strings.Join(bs[0].TocarSolo, ","); got != "go.mod,Makefile" {
		t.Errorf("tocar_solo[0] = %q, quiero go.mod,Makefile", got)
	}
	if got := strings.Join(bs[1].TocarSolo, ","); got != "internal/wol/**" {
		t.Errorf("tocar_solo[1] = %q, sin cambios", got)
	}
}

func TestIdsCorrelativos(t *testing.T) {
	root := t.TempDir()
	dir := config.TasksDir(root)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := task.Save(dir, task.Task{Version: task.Version, ID: "T-001", Titulo: "x", ListoCuando: "true", LimiteIntentos: 3, LimiteLineas: 200}); err != nil {
		t.Fatal(err)
	}
	ids, err := idsCorrelativos(dir, 3)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"T-002", "T-003", "T-004"}
	for i, w := range want {
		if ids[i] != w {
			t.Errorf("ids[%d] = %s, quiero %s", i, ids[i], w)
		}
	}
}

func TestIdsCorrelativosVacio(t *testing.T) {
	dir := config.TasksDir(t.TempDir())
	ids, err := idsCorrelativos(dir, 1)
	if err != nil {
		t.Fatal(err)
	}
	if ids[0] != "T-001" {
		t.Errorf("primer id = %s, quiero T-001", ids[0])
	}
}

func TestConfirmar(t *testing.T) {
	if !confirmar(strings.NewReader("s\n")) {
		t.Error("s debió confirmar")
	}
	if confirmar(strings.NewReader("n\n")) {
		t.Error("n no debió confirmar")
	}
	if confirmar(strings.NewReader("q\n")) {
		t.Error("respuesta suelta no debió confirmar")
	}
}
