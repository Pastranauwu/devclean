package ship

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/Pastranauwu/devclean/internal/room"
)

// abrirPR sube la rama, abre el PR con gh y libera el cuarto (§6.5.8).
// Devuelve la URL del PR. gh es la vía de v0.1; el fallback por API REST
// llega después (§15).
func abrirPR(ctx context.Context, root string, r room.Room, base, titulo, cuerpo string) (string, error) {
	gh, err := exec.LookPath("gh")
	if err != nil {
		return "", errors.New("gh no está instalado · instálalo o usa --dry-run")
	}

	if _, err := gitRun(r.Path, "push", "-u", "origin", r.Rama); err != nil {
		return "", fmt.Errorf("no se pudo subir la rama · %s", tail(err.Error()))
	}

	f, err := os.CreateTemp("", "devclean-handoff-*.md")
	if err != nil {
		return "", err
	}
	defer os.Remove(f.Name())
	if _, err := f.WriteString(cuerpo); err != nil {
		f.Close()
		return "", err
	}
	f.Close()

	cmd := exec.CommandContext(ctx, gh, "pr", "create", "--base", base, "--head", r.Rama, "--title", titulo, "--body-file", f.Name())
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("gh no pudo abrir el PR · %s", tail(string(out)))
	}

	// liberar el cuarto; si falla, el PR ya está abierto, no se rompe
	_ = room.Destroy(ctx, root, r.ID)

	return strings.TrimSpace(string(out)), nil
}
