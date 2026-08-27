package ship

import (
	"context"
	"errors"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// verificarBisectable corre el comando de pruebas del proyecto sobre el
// commit aplanado (§6.5.6): cada commit debe compilar y pasar pruebas.
// Con un solo commit, es correr la suite una vez y exigir verde.
func verificarBisectable(ctx context.Context, roomPath, pruebas string, timeout time.Duration) (string, bool) {
	if strings.TrimSpace(pruebas) == "" {
		return "sin comando de pruebas · decláralo en config.yml", false
	}
	salida, code := runComando(ctx, roomPath, pruebas, timeout)
	if code != nil && *code == 0 {
		return "verde · " + pruebas, true
	}
	return "falla " + pruebas + " · " + tail(salida), false
}

func runComando(ctx context.Context, dir, cmdStr string, timeout time.Duration) (string, *int) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/c", cmdStr)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", cmdStr)
	}
	cmd.Dir = dir

	out, err := cmd.CombinedOutput()
	code := 0
	if ctx.Err() == context.DeadlineExceeded {
		code = 124
		return string(out), &code
	}
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			code = exitErr.ExitCode()
			return string(out), &code
		}
		return string(out) + err.Error(), nil
	}
	return string(out), &code
}

// tail conserva la última línea útil de una salida para el motivo.
func tail(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "sin salida"
	}
	lines := strings.Split(s, "\n")
	last := strings.TrimSpace(lines[len(lines)-1])
	if last == "" && len(lines) > 1 {
		last = strings.TrimSpace(lines[len(lines)-2])
	}
	const max = 160
	if len(last) > max {
		last = last[:max] + "…"
	}
	return last
}
