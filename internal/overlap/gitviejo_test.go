package overlap

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// En git < 2.38 el flag --write-tree no existe: git sale con 129 y la
// deteccion textual quedaba apagada reportando "limpio".
func TestGitViejoNoSeReportaComoLimpio(t *testing.T) {
	casos := []struct {
		code   int
		stderr string
		quiero bool
	}{
		{129, "error: unknown option `write-tree'\nusage: git merge-tree", true},
		{129, "error: opción desconocida: `write-tree'\nuso: git merge-tree", true},
		{1, "CONFLICT (content)", false},
		{0, "", false},
		{128, "fatal: not a git repository", false},
	}
	for _, c := range casos {
		if got := esGitSinWriteTree(c.code, c.stderr); got != c.quiero {
			t.Errorf("code=%d stderr=%q → %v, quiero %v", c.code, c.stderr, got, c.quiero)
		}
	}
}

// Lo que el hallazgo pedía: que "no pude comparar" no se confunda con
// "no hay conflicto" en lo que ve el usuario.
func TestAlertaDistingueIndeterminadoDeLimpio(t *testing.T) {
	limpio := Resultado{TareaA: "T-001", TareaB: "T-002"}
	if a := limpio.Alerta(); a != "" {
		t.Errorf("sin cruce debe callar, dijo %q", a)
	}
	ciego := Resultado{TareaA: "T-001", TareaB: "T-002", Indeterminado: errGitViejo.Error()}
	a := ciego.Alerta()
	if a == "" {
		t.Fatal("no poder comparar se reportaba como limpio")
	}
	if !strings.Contains(a, "2.38") {
		t.Errorf("la alerta no dice qué hacer: %q", a)
	}
}

// Una rama que aún no existe no es un conflicto ni un fallo: sigue
// siendo silencio (era el falso positivo que el código ya había curado).
func TestRamaInexistenteSigueEnSilencio(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("sin git")
	}
	root := t.TempDir()
	for _, args := range [][]string{
		{"init", "-b", "main", "-q"},
		{"-c", "user.email=t@t", "-c", "user.name=t", "commit", "--allow-empty", "-m", "init", "-q"},
	} {
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	_ = os.WriteFile(filepath.Join(root, "x"), []byte("x"), 0o644)

	_, conflictos, err := mergeTree(root, "no-existe-a", "no-existe-b")
	if len(conflictos) != 0 {
		t.Errorf("rama inexistente reportada como conflicto: %v", conflictos)
	}
	if err != nil && strings.Contains(err.Error(), "2.38") {
		t.Errorf("rama inexistente confundida con git viejo: %v", err)
	}
}
