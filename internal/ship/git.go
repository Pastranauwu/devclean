package ship

import (
	"context"
	"os/exec"
	"strconv"
	"strings"
)

// gitRun ejecuta git en dir y devuelve la salida combinada.
func gitRun(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// rebase trae la base y rebasea la rama del cuarto sobre ella (§6.5.1).
// Devuelve la ref destino y, si hubo conflicto, los archivos implicados.
func rebase(ctx context.Context, root, roomPath, base, rama string) (target string, conflictos []string, err error) {
	// sin remoto no hay qué traer: el fetch falla y se ignora
	_, _ = gitRun(root, "fetch", "--quiet")

	target = base
	if _, err := gitRun(root, "rev-parse", "--verify", "--quiet", "origin/"+base); err == nil {
		target = "origin/" + base
	}

	if _, err := gitRun(roomPath, "rebase", target); err != nil {
		conflictos = unmerged(roomPath)
		_, _ = gitRun(roomPath, "rebase", "--abort")
		return target, conflictos, err
	}
	return target, nil, nil
}

// unmerged lista los archivos en conflicto (diff-filter U).
func unmerged(dir string) []string {
	out, _ := gitRun(dir, "diff", "--name-only", "--diff-filter=U")
	var files []string
	for _, l := range strings.Split(out, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			files = append(files, l)
		}
	}
	return files
}

// aplanar colapsa los commits wip de la rama en uno solo con mensaje
// Conventional Commits y trailer Agent: (§6.5.2). Devuelve cuántos wip
// había y el hash del commit resultante.
func aplanar(ctx context.Context, roomPath, target, titulo, tipo, modelo string) (int, string, error) {
	out, err := gitRun(roomPath, "rev-list", "--count", target+"..HEAD")
	if err != nil {
		return 0, "", err
	}
	cuenta, _ := strconv.Atoi(strings.TrimSpace(out))

	if _, err := gitRun(roomPath, "reset", "--soft", target); err != nil {
		return 0, "", err
	}
	if _, err := gitRun(roomPath, "diff", "--cached", "--quiet"); err == nil {
		return 0, "", &noEntregaError{}
	}

	args := append(identity(roomPath), "commit", "-m", tipo+": "+titulo)
	if modelo != "" {
		args = append(args, "-m", "Agent: "+modelo)
	}
	if _, err := gitRun(roomPath, args...); err != nil {
		return 0, "", err
	}
	hash, err := gitRun(roomPath, "rev-parse", "HEAD")
	return cuenta, strings.TrimSpace(hash), err
}

// noEntregaError es el caso "nada que entregar": no hay cambios.
type noEntregaError struct{}

func (noEntregaError) Error() string { return "nada que entregar · la rama no tiene cambios" }

// identity devuelve los -c de identidad cuando el repo no tiene una
// configurada. Hacen falta LAS DOS: git deduce el correo del sistema
// cuando falta, pero no el nombre, y entonces cualquier commit muere con
// "empty ident name (for <user@host>) not allowed". Es lo que pasa en un
// runner de CI limpio, donde nadie corrió `git config user.name`.
func identity(roomPath string) []string {
	nombre, _ := gitRun(roomPath, "config", "user.name")
	email, _ := gitRun(roomPath, "config", "user.email")
	if strings.TrimSpace(nombre) == "" || strings.TrimSpace(email) == "" {
		return []string{"-c", "user.name=devclean", "-c", "user.email=devclean@local"}
	}
	return nil
}

// diffAplanado devuelve el texto, los archivos y las líneas del commit
// aplanado frente a la base. Alimenta a los escáneres.
func diffAplanado(roomPath, target string) (texto string, archivos []string, mas, menos int, err error) {
	texto, err = gitRun(roomPath, "diff", target+"..HEAD")
	if err != nil {
		return "", nil, 0, 0, err
	}
	archivos, err = diffArchivos(roomPath, target)
	if err != nil {
		return "", nil, 0, 0, err
	}
	mas, menos, err = diffNumstat(roomPath, target)
	if err != nil {
		return "", nil, 0, 0, err
	}
	return texto, archivos, mas, menos, nil
}

func diffArchivos(roomPath, target string) ([]string, error) {
	out, err := gitRun(roomPath, "diff", "--name-only", target+"..HEAD")
	if err != nil {
		return nil, err
	}
	var files []string
	for _, l := range strings.Split(out, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			files = append(files, l)
		}
	}
	return files, nil
}

func diffNumstat(roomPath, target string) (mas, menos int, err error) {
	out, err := gitRun(roomPath, "diff", "--numstat", target+"..HEAD")
	if err != nil {
		return 0, 0, err
	}
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		a, errA := strconv.Atoi(fields[0])
		d, errD := strconv.Atoi(fields[1])
		if errA != nil || errD != nil {
			continue
		}
		mas += a
		menos += d
	}
	return mas, menos, nil
}

// tipoCommit infiere el tipo Conventional Commits del título de la tarea.
func tipoCommit(titulo string) string {
	lower := strings.ToLower(titulo)
	for _, w := range []string{"arregl", "fix", "bug", "falla", "corrig", "repar", "error", "romp"} {
		if strings.Contains(lower, w) {
			return "fix"
		}
	}
	return "feat"
}

// unir junta una lista en un texto legible.
func unir(items []string) string {
	return strings.Join(items, ", ")
}

// itoa convierte un entero sin strconv en cada llamada.
func itoa(n int) string { return strconv.Itoa(n) }
