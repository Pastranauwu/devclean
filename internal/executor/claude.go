package executor

import (
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

// Models devuelve los alias de modelo que acepta `claude --model`. El
// CLI no expone un subcomando para listarlos, así que van fijos: son
// alias estables, no ids de versión.
func (Claude) Models(context.Context) ([]string, error) {
	return []string{"opus", "sonnet", "haiku"}, nil
}

func (e Claude) Run(ctx context.Context, req Request) (Result, error) {
	// bypassPermissions: el agente no puede preguntar nada (modo -p) y
	// el contenedor real es el cuarto + la reversión de devclean (§11)
	args := []string{"-p", req.Prompt, "--output-format", "json", "--permission-mode", "bypassPermissions"}
	if req.Model != "" {
		args = append(args, "--model", req.Model)
	}
	stdout, stderr, code, err := run(ctx, req, "claude", args...)
	res := Result{Stdout: stdout, Stderr: stderr, ExitCode: code}
	res.Text = parseClaudeText(stdout)
	res.Tokens = parseClaudeUsage(stdout)
	return res, err
}

// parseClaudeText reads the "result" field of --output-format json.
func parseClaudeText(stdout string) string {
	var parsed struct {
		Result string `json:"result"`
	}
	if json.Unmarshal([]byte(strings.TrimSpace(stdout)), &parsed) != nil {
		return ""
	}
	return parsed.Result
}

// parseClaudeUsage reads the final JSON object of --output-format json.
func parseClaudeUsage(stdout string) Usage {
	var parsed struct {
		Usage struct {
			Input  int `json:"input_tokens"`
			Output int `json:"output_tokens"`
		} `json:"usage"`
	}
	if json.Unmarshal([]byte(strings.TrimSpace(stdout)), &parsed) != nil {
		return Usage{}
	}
	return Usage{Input: parsed.Usage.Input, Output: parsed.Usage.Output}
}
