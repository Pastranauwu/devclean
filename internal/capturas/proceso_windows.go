package capturas

import "os/exec"

func enGrupo(*exec.Cmd) {}

// ponytail: en Windows solo muere el proceso de cmd; los hijos que haya
// lanzado "levantar" pueden quedar vivos. taskkill /T lo cubriría.
func apagar(c *exec.Cmd) {
	if c.Process != nil {
		_ = c.Process.Kill()
		_, _ = c.Process.Wait()
	}
}
