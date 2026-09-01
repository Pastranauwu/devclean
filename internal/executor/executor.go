// Package executor wraps agent CLIs as subprocesses behind the
// Executor interface (§8.3, opción A). devclean does not implement its
// own tool loop: it inherits editing, context and tools from the CLI.
package executor

import (
	"bytes"
	"context"
	"errors"
	"fmt"
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
	// Models lista los ids de modelo que este CLI acepta de verdad.
	// devclean nunca inventa ids: los pide al CLI y valida contra esta
	// lista antes de gastar un token.
	Models(ctx context.Context) ([]string, error)
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
	Stderr       string   `json:"stderr"` // diagnóstico del CLI: sin esto un fallo de infra es invisible
	Text         string   `json:"text"`   // la respuesta textual del agente, si el adaptador la saca
	ExitCode     int      `json:"exit_code"`
}

// Usage is the token spend of one invocation, best-effort per adapter.
type Usage struct {
	Input  int `json:"input"`
	Output int `json:"output"`
}

// run executes a CLI with timeout and returns its stdout, stderr, exit
// code and error. A timeout yields exit code 124.
//
// stderr se captura aparte a propósito: es donde los CLIs de agente
// escriben el motivo real de un fallo (modelo inexistente, key ausente,
// rate limit). Descartarlo dejaba al usuario con "exit status 1" y nada
// más, que es indistinguible de "el agente trabajó y las pruebas
// fallaron".
func run(ctx context.Context, req Request, name string, args ...string) (string, string, int, error) {
	ctx, cancel := context.WithTimeout(ctx, req.Timeout)
	defer cancel()

	bin, err := findBinary(name)
	if err != nil {
		bin = name
	}

	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = req.RoomPath
	cmd.Env = append(os.Environ(), req.Env...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	// Sin WaitDelay, matar el proceso por timeout no basta: si el CLI
	// dejó un nieto vivo con el pipe abierto, cmd.Wait se queda esperando
	// para siempre y `timeout_agente` deja de ser un límite. Con esto se
	// cierran los pipes y la invocación vuelve, colgada o no.
	cmd.WaitDelay = 10 * time.Second

	err = cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return stdout.String(), stderr.String(), 124, err
	}
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return stdout.String(), stderr.String(), exitErr.ExitCode(), err
		}
		return stdout.String(), stderr.String(), -1, err
	}
	return stdout.String(), stderr.String(), 0, nil
}

// modelosDeCLI corre un subcomando que lista modelos, una línea cada
// uno, y devuelve los ids tal cual los acepta el CLI.
func modelosDeCLI(ctx context.Context, name string, args ...string) ([]string, error) {
	bin, err := findBinary(name)
	if err != nil {
		return nil, err
	}
	out, err := exec.CommandContext(ctx, bin, args...).Output()
	if err != nil {
		return nil, fmt.Errorf("%s no pudo listar modelos · %s", name, err)
	}
	var ids []string
	for _, l := range strings.Split(string(out), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			ids = append(ids, l)
		}
	}
	return ids, nil
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
