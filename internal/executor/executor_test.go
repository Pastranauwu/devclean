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
	if got, _ := parseClaudeStream(stdout); got != `[{"titulo":"x"}]` {
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

func TestClaudePasaEsfuerzoExplicito(t *testing.T) {
	fakeBin(t, "claude", `printf '%s\n' "$@"`)
	req := reqDePrueba()
	req.Effort = "medium"
	res, err := (Claude{}).Run(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Stdout, "--effort\nmedium\n") {
		t.Fatalf("argumentos: %q", res.Stdout)
	}
	req.Effort = ""
	res, err = (Claude{}).Run(context.Background(), req)
	if err != nil || strings.Contains(res.Stdout, "--effort") {
		t.Fatalf("alteró el esfuerzo por defecto: %q, %v", res.Stdout, err)
	}
}

// Con claude casi todo el prompt llega como caché: input_tokens solo
// cuenta lo que quedó fuera de ella. Y el total real está en modelUsage:
// el usage del resultado se queda corto (789k contra 4,58M medidos).
func TestUsoConCache(t *testing.T) {
	stream := `{"type":"system","subtype":"init","tools":["Bash"]}
{"type":"assistant","parent_tool_use_id":null,"message":{"id":"msg_1","usage":{"input_tokens":10,"cache_creation_input_tokens":11106,"cache_read_input_tokens":12306,"output_tokens":1}}}
{"type":"assistant","parent_tool_use_id":null,"message":{"id":"msg_1","usage":{"input_tokens":10,"cache_creation_input_tokens":11106,"cache_read_input_tokens":12306,"output_tokens":1}}}
{"type":"user","parent_tool_use_id":null,"message":{"role":"user"}}
{"type":"assistant","parent_tool_use_id":"toolu_9","message":{"id":"msg_sub","usage":{"input_tokens":3,"cache_creation_input_tokens":900,"cache_read_input_tokens":0,"output_tokens":5}}}
{"type":"assistant","parent_tool_use_id":null,"message":{"id":"msg_2","usage":{"input_tokens":2,"cache_creation_input_tokens":300,"cache_read_input_tokens":23412,"output_tokens":40}}}
{"type":"result","result":"listo","num_turns":2,"total_cost_usd":0.7855,"usage":{"input_tokens":129,"cache_creation_input_tokens":42631,"cache_read_input_tokens":789576,"output_tokens":11848},"modelUsage":{"claude-haiku-4-5":{"inputTokens":554,"outputTokens":28526,"cacheReadInputTokens":4579893,"cacheCreationInputTokens":121937}}}`
	texto, u := parseClaudeStream(stream)
	quiero := Usage{Input: 554, Output: 28526, CacheRead: 4579893, CacheWrite: 121937, Turns: 2, FirstTurn: 23422, FirstTurnWrite: 11106, CostUSD: 0.7855}
	if texto != "listo" || u != quiero {
		t.Errorf("claude: %q %+v\nquiero %+v", texto, u, quiero)
	}
	// la salida de --output-format json (un solo resultado, sin turnos) sigue leyéndose
	if _, viejo := parseClaudeStream(`{"type":"result","num_turns":19,"usage":{"input_tokens":129,"output_tokens":11848}}`); viejo != (Usage{Input: 129, Output: 11848, Turns: 19}) {
		t.Errorf("json: %+v", viejo)
	}

	_, _, oc := parseOpenCodeEvents(`{"type":"step_finish","part":{"cost":0.5,"tokens":{"input":34,"output":67,"cache":{"read":900,"write":50}}}}` + "\n" +
		`{"type":"step_finish","part":{"cost":0.25,"tokens":{"input":6,"output":3,"cache":{"read":100,"write":0}}}}`)
	if oc != (Usage{Input: 40, Output: 70, CacheRead: 1000, CacheWrite: 50, Turns: 2, FirstTurn: 984, FirstTurnWrite: 50, CostUSD: 0.75}) {
		t.Errorf("opencode: %+v", oc)
	}
}

// Cada agente de claude arranca sin lo que el usuario configuró para su
// propio Claude Code (hooks, plugins, MCP), y sin --disable-slash-commands,
// que también apagaría las skills del proyecto.
func TestClaudeArrancaConContextoLimpio(t *testing.T) {
	fakeBin(t, "claude", `printf '%s\n' "$@"`)
	res, err := (Claude{}).Run(context.Background(), reqDePrueba())
	if err != nil {
		t.Fatal(err)
	}
	for _, quiero := range []string{"--setting-sources\nproject,local\n", "--strict-mcp-config\n", "--exclude-dynamic-system-prompt-sections\n", "--output-format\nstream-json\n--verbose\n"} {
		if !strings.Contains(res.Stdout, quiero) {
			t.Errorf("falta %q en %q", quiero, res.Stdout)
		}
	}
	if strings.Contains(res.Stdout, "--disable-slash-commands") {
		t.Error("--disable-slash-commands apaga las skills del proyecto")
	}
}

func TestClaudeHerramientasPorRol(t *testing.T) {
	fakeBin(t, "claude", `printf '%s\n' "$@"`)
	for rol, quiero := range map[Rol]string{
		RolImplementador: "--tools\nBash,Read,Edit,Write\n",
		RolTexto:         "--tools\n\n",
		RolPlanificador:  "--tools\nRead,Bash\n",
	} {
		req := reqDePrueba()
		req.Rol = rol
		res, err := (Claude{}).Run(context.Background(), req)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(res.Stdout, quiero) {
			t.Errorf("rol %q: falta %q en %q", rol, quiero, res.Stdout)
		}
	}
}
