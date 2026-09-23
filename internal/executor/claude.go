package executor

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"strings"
)

// Claude wraps the claude CLI (Claude Code) in print mode.
type Claude struct{}

func (Claude) Name() string { return "claude" }

func (Claude) Available() error {
	bin, err := findBinary("claude")
	if err != nil {
		return err
	}
	if out, err := exec.Command(bin, "--version").Output(); err != nil || len(strings.TrimSpace(string(out))) == 0 {
		return errors.New("claude no responde a --version · verifica la instalación")
	}
	return nil
}

// Models devuelve los modelos que acepta `claude --model`. El CLI no
// expone un subcomando para listarlos, así que van fijos: primero los ids
// de versión —el alias "opus" apunta a Opus 5, no a 5.5— y después los
// alias, que siguen valiendo para los config.yml que ya los tienen.
func (Claude) Models(context.Context) ([]string, error) {
	return []string{"claude-opus-5-5", "claude-fable-5-1", "claude-sonnet-5", "claude-haiku-4-5", "opus", "sonnet", "haiku"}, nil
}

// contextoLimpio deja fuera del agente lo que el usuario configuró para
// su propio Claude Code. Medido en un cuarto real: el contexto base de un
// "responde ok" baja de 23,4k a 12k tokens.
//   - --setting-sources project,local: sin settings de usuario, o sea sin
//     sus hooks (caveman entraba como estilo de prosa), plugins ni skills.
//     El .claude/ y el CLAUDE.md del repo siguen cargando.
//   - --strict-mcp-config: sin MCP, tampoco los conectores de claude.ai.
//   - --exclude-dynamic-system-prompt-sections: el cwd y el git status
//     pasan del prompt de sistema al primer mensaje, así el prompt de
//     sistema es igual en todos los cuartos y el caché se comparte.
//
// --disable-slash-commands no va: también apaga las skills del proyecto.
var contextoLimpio = []string{"--setting-sources", "project,local", "--strict-mcp-config", "--exclude-dynamic-system-prompt-sections"}

func (e Claude) Run(ctx context.Context, req Request) (Result, error) {
	// bypassPermissions: el agente no puede preguntar nada (modo -p) y
	// el contenedor real es el cuarto + la reversión de devclean
	// stream-json y no json: el resultado final trae lo mismo, y los
	// eventos de cada turno son la única forma de saber cuántos turnos
	// hubo y con cuánto contexto arrancó el primero
	args := []string{"-p", req.Prompt, "--output-format", "stream-json", "--verbose", "--permission-mode", "bypassPermissions"}
	args = append(args, contextoLimpio...)
	if req.Model != "" {
		args = append(args, "--model", req.Model)
	}
	if req.Effort != "" {
		args = append(args, "--effort", req.Effort)
	}
	stdout, stderr, code, err := run(ctx, req, "claude", args...)
	res := Result{Stdout: stdout, Stderr: stderr, ExitCode: code}
	res.Text, res.Tokens = parseClaudeStream(stdout)
	return res, err
}

// claudeUsage es el gasto como lo reporta la API: en `usage` de cada
// mensaje y en `usage` del resultado.
type claudeUsage struct {
	Input      int `json:"input_tokens"`
	Output     int `json:"output_tokens"`
	CacheRead  int `json:"cache_read_input_tokens"`
	CacheWrite int `json:"cache_creation_input_tokens"`
}

// claudeEvento es una línea de --output-format stream-json. La salida de
// --output-format json es un solo evento de resultado, así que se lee igual.
type claudeEvento struct {
	Type            string  `json:"type"`
	ParentToolUseID *string `json:"parent_tool_use_id"`
	Message         struct {
		ID    string      `json:"id"`
		Usage claudeUsage `json:"usage"`
	} `json:"message"`
	Result     string      `json:"result"`
	NumTurns   int         `json:"num_turns"`
	CostUSD    float64     `json:"total_cost_usd"`
	Usage      claudeUsage `json:"usage"`
	ModelUsage map[string]struct {
		Input      int `json:"inputTokens"`
		Output     int `json:"outputTokens"`
		CacheRead  int `json:"cacheReadInputTokens"`
		CacheWrite int `json:"cacheCreationInputTokens"`
	} `json:"modelUsage"`
}

// ParseClaudeUsage lee el gasto de una salida de claude ya guardada
// (stream-json o json). Lo usa el benchmark para medir corridas viejas.
func ParseClaudeUsage(stdout string) Usage {
	_, u := parseClaudeStream(stdout)
	return u
}

// parseClaudeStream saca el texto final y el gasto del stream de claude.
//
// Los totales salen de `modelUsage` y no de `usage`: el `usage` del
// resultado se queda corto. Medido en una tarea real con haiku: `usage`
// decía 789k de caché leída y `modelUsage` 4,58M, y solo este último
// cuadra con el `total_cost_usd` que el mismo resultado reporta.
//
// Los turnos se cuentan por id de mensaje del hilo principal: claude
// emite un evento `assistant` por cada bloque de contenido, todos con el
// mismo id y el mismo usage.
func parseClaudeStream(stdout string) (string, Usage) {
	var u Usage
	var texto string
	vistos := map[string]bool{}
	numTurns := 0
	scanner := bufio.NewScanner(strings.NewReader(stdout))
	scanner.Buffer(make([]byte, 1024*1024), 16*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || line[0] != '{' {
			continue
		}
		var ev claudeEvento
		if json.Unmarshal([]byte(line), &ev) != nil {
			continue
		}
		switch ev.Type {
		case "assistant":
			if ev.ParentToolUseID != nil || ev.Message.ID == "" || vistos[ev.Message.ID] {
				continue
			}
			vistos[ev.Message.ID] = true
			if len(vistos) == 1 {
				m := ev.Message.Usage
				u.FirstTurn, u.FirstTurnWrite = m.Input+m.CacheRead+m.CacheWrite, m.CacheWrite
			}
		case "result":
			texto, numTurns, u.CostUSD = ev.Result, ev.NumTurns, ev.CostUSD
			if len(ev.ModelUsage) > 0 {
				for _, m := range ev.ModelUsage {
					u.Input += m.Input
					u.Output += m.Output
					u.CacheRead += m.CacheRead
					u.CacheWrite += m.CacheWrite
				}
			} else {
				u.Input, u.Output, u.CacheRead, u.CacheWrite = ev.Usage.Input, ev.Usage.Output, ev.Usage.CacheRead, ev.Usage.CacheWrite
			}
		}
	}
	u.Turns = len(vistos)
	if u.Turns == 0 {
		u.Turns = numTurns // salida sin eventos por turno
	}
	return texto, u
}
