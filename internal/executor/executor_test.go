package executor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// fakeBin instala un CLI falso en PATH y devuelve su directorio.
func fakeBin(t *testing.T, name, script string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("los binarios falsos son scripts de shell")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func reqDePrueba() Request {
	return Request{
		RoomPath: ".",
		Prompt:   "haz la tarea",
		Timeout:  5 * time.Second,
		Env:      []string{"PORT=4321"},
	}
}

func TestOpenCodeRun(t *testing.T) {
	fakeBin(t, "opencode", `cat <<'EOF'
{"type":"step_start"}
{"type":"tool_use","part":{"path":"src/export/csv.go"}}
{"type":"tool_use","part":{"path":"src/export/csv.go"}}
{"type":"step_finish","tokens":{"input":100,"output":40}}
{"type":"step_finish","tokens":{"input":50,"output":10}}
EOF`)
	e := OpenCode{}
	if err := e.Available(); err != nil {
		t.Fatalf("Available: %v", err)
	}
	res, err := e.Run(context.Background(), reqDePrueba())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.ExitCode != 0 {
		t.Errorf("ExitCode = %d", res.ExitCode)
	}
	if len(res.FilesChanged) != 1 || res.FilesChanged[0] != "src/export/csv.go" {
		t.Errorf("FilesChanged = %v", res.FilesChanged)
	}
	if res.Tokens.Input != 150 || res.Tokens.Output != 50 {
		t.Errorf("Tokens = %+v", res.Tokens)
	}
}

func TestClaudeRun(t *testing.T) {
	fakeBin(t, "claude", `echo '{"type":"result","result":"hecho","usage":{"input_tokens":10,"output_tokens":20}}'`)
	e := Claude{}
	if err := e.Available(); err != nil {
		t.Fatalf("Available: %v", err)
	}
	res, err := e.Run(context.Background(), reqDePrueba())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Tokens.Input != 10 || res.Tokens.Output != 20 {
		t.Errorf("Tokens = %+v", res.Tokens)
	}
}

func TestRunExitCode(t *testing.T) {
	fakeBin(t, "opencode", "exit 3")
	res, err := OpenCode{}.Run(context.Background(), reqDePrueba())
	if err == nil {
		t.Fatal("Run debió devolver error")
	}
	if res.ExitCode != 3 {
		t.Errorf("ExitCode = %d, quiero 3", res.ExitCode)
	}
}

func TestRunTimeout(t *testing.T) {
	fakeBin(t, "opencode", "sleep 10")
	req := reqDePrueba()
	req.Timeout = 100 * time.Millisecond
	res, err := OpenCode{}.Run(context.Background(), req)
	if err == nil {
		t.Fatal("Run con timeout debió devolver error")
	}
	if res.ExitCode != 124 {
		t.Errorf("ExitCode = %d, quiero 124", res.ExitCode)
	}
}

func TestRunPasaEnv(t *testing.T) {
	fakeBin(t, "opencode", "echo \"port=$PORT\"")
	res, err := OpenCode{}.Run(context.Background(), reqDePrueba())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(res.Stdout, "port=4321") {
		t.Errorf("el entorno del cuarto no llegó al agente: %q", res.Stdout)
	}
}

func TestAvailableSinBinario(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	err := OpenCode{}.Available()
	if err == nil || !strings.Contains(err.Error(), "opencode") {
		t.Errorf("Available = %v", err)
	}
	err = Claude{}.Available()
	if err == nil || !strings.Contains(err.Error(), "claude") {
		t.Errorf("Available = %v", err)
	}
}

func TestParseOpenCodeEventsIgnoraBasura(t *testing.T) {
	stdout := "no json\n{\"mal\":true}\n{\"type\":\"x\",\"path\":\"a/b.go\"}\n"
	files, _, _ := parseOpenCodeEvents(stdout)
	if fmt.Sprint(files) != "[a/b.go]" {
		t.Errorf("files = %v", files)
	}
}

func TestOpenCodeExtraeTexto(t *testing.T) {
	stdout := `{"type":"message","part":{"type":"text","text":"hola"}}
{"type":"message","part":{"type":"text","text":"mundo"}}`
	_, text, _ := parseOpenCodeEvents(stdout)
	if text != "hola\nmundo" {
		t.Errorf("text = %q, quiero hola\\nmundo", text)
	}
}

func TestClaudeExtraeTexto(t *testing.T) {
	stdout := `{"type":"result","result":"[{\"titulo\":\"x\"}]","usage":{"input_tokens":1,"output_tokens":2}}`
	if got := parseClaudeText(stdout); got != `[{"titulo":"x"}]` {
		t.Errorf("text = %q", got)
	}
}

// opencode anida el gasto bajo "part": leerlo solo en la raíz daba
// siempre 0 tokens.
func TestParseOpenCodeTokensAnidados(t *testing.T) {
	stream := `{"type":"step_finish","part":{"type":"step-finish","tokens":{"total":21897,"input":34,"output":67}}}
{"type":"step_finish","part":{"type":"step-finish","tokens":{"input":6,"output":4}}}
{"type":"text","part":{"type":"text","text":"listo"}}`
	_, texto, usage := parseOpenCodeEvents(stream)
	if usage.Input != 40 || usage.Output != 71 {
		t.Errorf("usage = %+v, quiero {Input:40 Output:71}", usage)
	}
	if texto != "listo" {
		t.Errorf("texto = %q", texto)
	}
}
