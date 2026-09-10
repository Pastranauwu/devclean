package loop

import (
	"os/exec"
	"strconv"
	"strings"
)

// gitRun ejecuta git en dir y devuelve la salida combinada. Un código de
// salida distinto de cero viene como error; el llamador decide si es lo
// esperado.
func gitRun(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// statusFiles lista los archivos cambiados en el árbol de trabajo
// respecto a HEAD, incluyendo los sin seguimiento. Sirve para la
// reversión, que corre antes de indexar nada.
func statusFiles(dir string) ([]string, error) {
	// --untracked-files=all: sin esto, git colapsa un directorio nuevo
	// entero en "?? dir/" y la reversión intenta borrar el directorio
	out, err := gitRun(dir, "status", "--porcelain", "--untracked-files=all")
	if err != nil {
		return nil, err
	}
	var files []string
	for _, line := range strings.Split(out, "\n") {
		if len(line) < 4 {
			continue
		}
		rest := strings.TrimSpace(line[3:])
		// rename: "R  viejo -> nuevo", nos quedamos con el nuevo
		if i := strings.Index(rest, " -> "); i >= 0 {
			rest = rest[i+4:]
		}
		if rest != "" {
			files = append(files, rest)
		}
	}
	return files, nil
}

// numstatSince devuelve las líneas añadidas y quitadas desde ref. Los
// binarios ("-") no cuentan.
//
// Mide contra una ref y no contra el índice a propósito: `git diff <ref>`
// compara ref con el árbol de trabajo, así que cuenta igual lo que el
// agente dejó sin commitear y lo que commiteó por su cuenta dentro del
// cuarto. Medirlo con `--cached HEAD` daba cero en el segundo caso — un
// agente que commitea (la skill `implement` lo hace) movía HEAD, dejaba
// el índice vacío y el intento se registraba con 0 líneas pese a haber
// trabajo. Corre después de `git add -A`, que es lo que mete los
// archivos nuevos en el árbol comparable.
func numstatSince(dir, ref string) (mas, menos int, err error) {
	out, err := gitRun(dir, "diff", "--numstat", ref)
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

// filesSince lista los archivos cambiados desde ref. Corre después de
// indexar, para que los archivos nuevos cuenten. Sirve para el acumulado
// contra la base de la tarea y para un intento suelto contra el commit
// con que arrancó; ver numstatSince sobre por qué la ref y no el índice.
func filesSince(dir, ref string) ([]string, error) {
	out, err := gitRun(dir, "diff", "--name-only", ref)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, line := range strings.Split(out, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}

// resolveCommit convierte una ref (rama o "HEAD") en hash de commit.
func resolveCommit(dir, ref string) (string, error) {
	out, err := gitRun(dir, "rev-parse", "--verify", "--quiet", ref+"^{commit}")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}
