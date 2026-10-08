package ship

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Una entrega cortada (Ctrl+C, kill) no corre su defer: la rama queda
// aplanada y la corrida siguiente la devuelve a su punta. Si después del
// corte hubo trabajo nuevo, no se toca.
func TestRestaurarPuntasTrasUnaEntregaCortada(t *testing.T) {
	root := repoConCommit(t)
	r := cuartoConWip(t, root)
	if err := os.MkdirAll(filepath.Dir(archivoPuntas(root)), 0o755); err != nil {
		t.Fatal(err)
	}
	punta := strings.TrimSpace(gitCmd(t, r.Path, "rev-parse", "HEAD"))
	aplanar := func() {
		t.Helper()
		gitCmd(t, r.Path, "reset", "--soft", "main")
		gitCmd(t, r.Path, "-c", "user.email=d@d", "-c", "user.name=d", "commit", "-m", "feat: aplanado")
	}

	guardarPuntas(root, map[string]string{r.Path: punta})
	aplanar()
	hechas, dudosas := RestaurarPuntas(root)
	if len(hechas) != 1 || len(dudosas) != 0 {
		t.Fatalf("restauradas=%v dudosas=%v, quiero la rama restaurada", hechas, dudosas)
	}
	if h := strings.TrimSpace(gitCmd(t, r.Path, "rev-parse", "HEAD")); h != punta {
		t.Fatalf("HEAD = %s, quiero la punta %s", h, punta)
	}
	if _, err := os.Stat(archivoPuntas(root)); err == nil {
		t.Error("el archivo de puntas debía borrarse")
	}

	// trabajo posterior al corte: el árbol ya no es el de la punta
	guardarPuntas(root, map[string]string{r.Path: punta})
	aplanar()
	escribir(t, r.Path, "nuevo.go", "package a\n")
	gitCmd(t, r.Path, "add", "-A")
	gitCmd(t, r.Path, "-c", "user.email=d@d", "-c", "user.name=d", "commit", "-m", "wip: después")
	despues := strings.TrimSpace(gitCmd(t, r.Path, "rev-parse", "HEAD"))
	hechas, dudosas = RestaurarPuntas(root)
	if len(hechas) != 0 || len(dudosas) != 1 {
		t.Fatalf("restauradas=%v dudosas=%v, quiero que no la toque", hechas, dudosas)
	}
	if h := strings.TrimSpace(gitCmd(t, r.Path, "rev-parse", "HEAD")); h != despues {
		t.Fatalf("perdió el trabajo posterior: HEAD = %s", h)
	}
}
