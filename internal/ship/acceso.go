package ship

import (
	"context"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

var repoGitHub = regexp.MustCompile(`github\.com[:/]([^/\s]+)/([^/\s]+?)(?:\.git)?/?$`)

// repoDeOrigin devuelve "dueño/repo" si url es de GitHub; "" si no.
func repoDeOrigin(url string) string {
	m := repoGitHub.FindStringSubmatch(strings.TrimSpace(url))
	if m == nil {
		return ""
	}
	return m[1] + "/" + m[2]
}

// AccesoRemoto dice por qué la cuenta activa de gh no va a poder abrir el
// PR en origin, o "" si puede o no hay cómo saberlo (sin origin, origin
// fuera de GitHub, sin gh, sin red). Con varias cuentas en la máquina la
// activa suele ser otra, y sin este aviso se descubre al final de la
// entrega. Pregunta a la API con tope de tiempo y nunca frena a nadie.
func AccesoRemoto(root string) string {
	url, err := gitRun(root, "remote", "get-url", "origin")
	repo := repoDeOrigin(url)
	gh, errGh := exec.LookPath("gh")
	if err != nil || repo == "" || errGh != nil {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	api := func(args ...string) (string, error) {
		out, err := exec.CommandContext(ctx, gh, append([]string{"api"}, args...)...).CombinedOutput()
		return strings.TrimSpace(string(out)), err
	}
	permiso, err := api("repos/"+repo, "--jq", ".permissions.push")
	if err == nil && permiso == "true" {
		return ""
	}
	// un repo privado al que la cuenta no entra responde 404; cualquier
	// otro fallo (red, gh sin sesión) no dice nada del acceso
	if err != nil && !strings.Contains(permiso, "404") {
		return ""
	}
	cuenta := "la cuenta activa de gh"
	if login, err := api("user", "--jq", ".login"); err == nil && login != "" {
		cuenta += " (" + login + ")"
	}
	return cuenta + " no puede subir a " + repo + " · cámbiala con gh auth switch o entrega con --local"
}
