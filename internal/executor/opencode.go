package executor

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"strings"
)

// OpenCode wraps the opencode CLI (https://opencode.ai).
type OpenCode struct{}

func (OpenCode) Name() string { return "opencode" }

func (OpenCode) Available() error {
	bin, err := findBinary("opencode")
	if err != nil {
		return err
	}
	if out, err := exec.Command(bin, "--version").Output(); err != nil || len(strings.TrimSpace(string(out))) == 0 {
		return errors.New("opencode no responde a --version · verifica la instalación")
	}
	return nil
}

// Models devuelve el catálogo real de `opencode models`, en el formato
// provider/model que el CLI exige. Sin esto devclean inventaba ids como
// "glm-5.2" que el servidor rechaza: cada invocación moría en dos
// segundos sin gastar un token y sin dejar rastro.
func (OpenCode) Models(ctx context.Context) ([]string, error) {
	return modelosDeCLI(ctx, "opencode", "models")
}

func (e OpenCode) Run(ctx context.Context, req Request) (Result, error) {
	args := []string{"run", req.Prompt, "--dir", req.RoomPath, "--format", "json", "--auto"}
	if req.Model != "" {
		args = append(args, "--model", req.Model)
	}
	stdout, stderr, code, err := run(ctx, req, "opencode", args...)
	res := Result{Stdout: stdout, Stderr: stderr, ExitCode: code}
	res.FilesChanged, res.Text, res.Tokens = parseOpenCodeEvents(stdout)
	return res, err
}

// parseOpenCodeEvents walks the JSONL event stream best-effort:
// file paths from tool events, tokens from step finish events, and the
// assistant text from text parts.
func parseOpenCodeEvents(stdout string) ([]string, string, Usage) {
	seen := map[string]bool{}
	var files []string
	var usage Usage
	var textos []string

	scanner := bufio.NewScanner(strings.NewReader(stdout))
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || line[0] != '{' {
			continue
		}
		var event map[string]any
		if json.Unmarshal([]byte(line), &event) != nil {
			continue
		}
		collectPaths(event, "", 0, seen, &files)
		collectText(event, &textos)
		collectTokens(event, &usage)
	}
	return files, strings.Join(textos, "\n"), usage
}

// collectTokens suma el gasto de cada evento step_finish. opencode lo
// anida bajo "part", no en la raíz del evento: mirando solo la raíz el
// gasto salía siempre 0, lo que además hacía indistinguible una
// invocación que nunca llegó al modelo de una que sí trabajó.
func collectTokens(m map[string]any, usage *Usage) {
	if tokens, ok := m["tokens"].(map[string]any); ok {
		usage.Input += intValue(tokens, "input")
		usage.Output += intValue(tokens, "output")
		return
	}
	if part, ok := m["part"].(map[string]any); ok {
		collectTokens(part, usage)
	}
}

// collectText finds every "text" string field in the event, best-effort.
func collectText(m map[string]any, out *[]string) {
	if v, ok := m["text"].(string); ok && strings.TrimSpace(v) != "" {
		*out = append(*out, v)
	}
	for _, key := range []string{"part", "message", "state", "input"} {
		if nested, ok := m[key].(map[string]any); ok {
			collectText(nested, out)
		}
	}
}

// collectPaths finds file paths under well-known keys, one level deep.
func collectPaths(m map[string]any, _ string, depth int, seen map[string]bool, files *[]string) {
	if depth > 1 {
		return
	}
	for _, key := range []string{"path", "file", "filePath", "filename"} {
		if v, ok := m[key].(string); ok && looksLikePath(v) && !seen[v] {
			seen[v] = true
			*files = append(*files, v)
		}
	}
	for _, key := range []string{"part", "tool", "state", "input"} {
		if nested, ok := m[key].(map[string]any); ok {
			collectPaths(nested, key, depth+1, seen, files)
		}
	}
}

func looksLikePath(s string) bool {
	return strings.Contains(s, "/") || strings.Contains(s, ".")
}

func intValue(m map[string]any, key string) int {
	switch v := m[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	}
	return 0
}
