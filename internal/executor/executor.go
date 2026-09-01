// Package executor wraps agent CLIs as subprocesses behind the
// Executor interface (§8.3, opción A). devclean does not implement its
// own tool loop: it inherits editing, context and tools from the CLI.
package executor

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Executor is the adapter contract of §8.3.
type Executor interface {
	Name() string
	Available() error // verifica binario y versión
	Run(ctx context.Context, req Request) (Result, error)
}

// Request is one agent invocation.
type Request struct {
	RoomPath     string   // cwd del agente, su cuarto
	Prompt       string   // contrato + resultado del intento anterior
	AllowedGlobs []string // tocar_solo
	Model        string
	Timeout      time.Duration
	// Env carries the room's own variables (PORT, ...) — §6.2.
	Env []string
}

// Result is what one invocation produced.
type Result struct {
	FilesChanged []string `json:"files_changed"`
	Tokens       Usage    `json:"tokens"`
	Stdout       string   `json:"stdout"`
	Text         string   `json:"text"` // la respuesta textual del agente, si el adaptador la saca
	ExitCode     int      `json:"exit_code"`
}

// Usage is the token spend of one invocation, best-effort per adapter.
type Usage struct {
	Input  int `json:"input"`
	Output int `json:"output"`
}

// run executes a CLI with timeout and returns its stdout, exit code
// and error. A timeout yields exit code 124.
func run(ctx context.Context, req Request, name string, args ...string) (string, int, error) {
	ctx, cancel := context.WithTimeout(ctx, req.Timeout)
	defer cancel()

	bin, err := findBinary(name)
	if err != nil {
		bin = name
	}

	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = req.RoomPath
	cmd.Env = append(os.Environ(), req.Env...)

	out, err := cmd.Output()
	if ctx.Err() == context.DeadlineExceeded {
		return string(out), 124, err
	}
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return string(out), exitErr.ExitCode(), err
		}
		return string(out), -1, err
	}
	return string(out), 0, nil
}

func init() {
	home, err := os.UserHomeDir()
	if err == nil && home != "" {
		cur := os.Getenv("PATH")
		paths := []string{
			filepath.Join(home, ".local", "bin"),
			filepath.Join(home, ".opencode", "bin"),
			filepath.Join(home, ".npm-global", "bin"),
			filepath.Join(home, "bin"),
		}
		for _, p := range paths {
			if !strings.Contains(cur, p) {
				if info, err := os.Stat(p); err == nil && info.IsDir() {
					cur = p + string(os.PathListSeparator) + cur
				}
			}
		}
		_ = os.Setenv("PATH", cur)
	}
}

// findBinary busca un binario ejecutable en el PATH.
func findBinary(name string) (string, error) {
	bin, err := exec.LookPath(name)
	if err != nil {
		return "", errors.New(name + " no está instalado · instálalo o elige otro ejecutor")
	}
	return bin, nil
}
