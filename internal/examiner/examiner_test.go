package examiner

import (
	"context"
	"strings"
	"testing"

	"github.com/Pastranauwu/devclean/internal/loop"
	"github.com/Pastranauwu/devclean/internal/task"
)

func TestBuildGoFileImports(t *testing.T) {
	visible := []string{
		"func TestLen(t *testing.T) {\n\tif !bytes.Equal(nil, nil) { t.Fatal(\"x\") }\n}",
	}
	src := buildGoFile("wol", "wakeup/internal/wol", []string{"bytes", "time", "github.com/x/y"}, visible)

	if !strings.Contains(src, `"bytes"`) {
		t.Error("falta import usado: bytes")
	}
	if strings.Contains(src, `"time"`) {
		t.Error("import no usado no debe aparecer: time")
	}
	if strings.Contains(src, "github.com/x/y") {
		t.Error("import externo no debe aparecer")
	}
	if !strings.Contains(src, `"wakeup/internal/wol"`) {
		t.Error("falta el paquete bajo prueba")
	}
	if err := validarSintaxis("go", src); err != nil {
		t.Errorf("el archivo armado no parsea: %v\n%s", err, src)
	}
}

func TestValidarSintaxisGoRechazaBasura(t *testing.T) {
	if validarSintaxis("go", "package x_test\n\nimport (\n\t\"testing\"\n)\n\nfunc TestX(t *testing.T) {\n\tnet.Foo(\n") == nil {
		t.Error("validarSintaxis debió rechazar código con paréntesis sin cerrar")
	}
}

func TestRunSaltaSinExpone(t *testing.T) {
	sellada, err := Run(nil, t.TempDir(), Options{
		Agent: stubAgent{},
		Task:  task.Task{ID: "T-001", Titulo: "init", TocarSolo: []string{"go.mod"}},
	})
	if err != nil || sellada {
		t.Errorf("tarea sin expone no debe examinarse: sellada=%v err=%v", sellada, err)
	}
}

type stubAgent struct{}

func (stubAgent) Name() string { return "stub" }
func (stubAgent) Run(context.Context, loop.Request) (loop.Result, error) {
	return loop.Result{}, nil
}

func TestParseRespTextImports(t *testing.T) {
	v, h, imp, err := parseRespText("```json\n{\"imports\":[\"net\"],\"visible\":[\"func TestA(t *testing.T){}\"],\"hidden\":[]}\n```")
	if err != nil {
		t.Fatal(err)
	}
	if len(v) != 1 || len(h) != 0 || len(imp) != 1 || imp[0] != "net" {
		t.Errorf("parse = v%v h%v imp%v", v, h, imp)
	}
}
