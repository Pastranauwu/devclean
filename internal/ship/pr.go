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

	// sin remoto el push falla con un "exit status 128" que no dice nada;
	// un repo local es un caso normal, no un error del usuario
	if _, err := gitRun(root, "remote", "get-url", "origin"); err != nil {
		return "", errors.New("sin remoto origin · agrégalo con git remote add origin <url> o usa --dry-run")
	}

	// el error de git trae el motivo en su salida, no en err.Error()
	// (que es solo "exit status N")
	if out, err := gitRun(r.Path, "push", "-u", "origin", r.Rama); err != nil {
		return "", fmt.Errorf("no se pudo subir la rama · %s", tail(out))
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
