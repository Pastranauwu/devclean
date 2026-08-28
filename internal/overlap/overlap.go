// Package overlap implements the active overlap detection of §6.9:
// textual (git merge-tree) and semantic (shared exported symbols from
// attempts.jsonl). The functional level is future work.
package overlap

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"github.com/Pastranauwu/devclean/internal/loop"
	"github.com/Pastranauwu/devclean/internal/room"
)

// Resultado is the overlap check result between two tasks.
type Resultado struct {
	TareaA     string   `json:"tarea_a"`
	TareaB     string   `json:"tarea_b"`
	Textual    bool     `json:"textual"`
	Semantico  bool     `json:"semantico"`
	Comunes    []string `json:"comunes,omitempty"`
	Conflictos []string `json:"conflictos,omitempty"`
}

// Alerta returns a human-readable alert if any overlap was detected.
// Returns "" if clean.
func (r Resultado) Alerta() string {
	if !r.Textual && !r.Semantico {
		return ""
	}
	var partes []string
	if r.Textual {
		partes = append(partes, "conflicto de texto en: "+strings.Join(r.Conflictos, ", "))
	}
	if r.Semantico {
		partes = append(partes, "símbolo(s) exportado(s) en común: "+strings.Join(r.Comunes, ", "))
	}
	return fmt.Sprintf("%s ↔ %s · %s", r.TareaA, r.TareaB, strings.Join(partes, "; "))
}

// CheckPar runs textual and semantic checks between two tasks.
// root is the repo root; attemptsA/B are the recorded attempts for each.
func CheckPar(root, idA, idB string, attemptsA, attemptsB []loop.Attempt) Resultado {
	res := Resultado{TareaA: idA, TareaB: idB}

	// semantic: shared exported symbols from last attempt
	res.Comunes = simbolosComunes(attemptsA, attemptsB)
	res.Semantico = len(res.Comunes) > 0

	// textual: git merge-tree between the two branches
	ramaA := room.Branch(idA)
	ramaB := room.Branch(idB)
	conflictos, _ := mergeTree(root, ramaA, ramaB)
	res.Conflictos = conflictos
	res.Textual = len(conflictos) > 0

	return res
}

// mergeTree runs git merge-tree and returns conflicting file paths.
// Returns nil, nil on clean merge or when branches don't exist yet.
func mergeTree(root, ramaA, ramaB string) ([]string, error) {
	cmd := exec.Command("git", "-C", root, "merge-tree", "--write-tree", "--no-messages", ramaA, ramaB)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	err := cmd.Run()
	if err == nil {
		return nil, nil // clean merge
	}
	// exit code 1 = conflicts; parse stdout for filenames
	// if the command itself doesn't exist or fails for another reason, skip gracefully
	var exitErr *exec.ExitError
	if !isExitError(err, &exitErr) {
		return nil, nil
	}
	var conflictos []string
	seen := map[string]bool{}
	for _, line := range strings.Split(stdout.String(), "\n") {
		// git merge-tree --write-tree prints "CONFLICT (content): ..." lines
		if !strings.HasPrefix(line, "CONFLICT") {
			continue
		}
		// last token is usually the file path
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		f := fields[len(fields)-1]
		if !seen[f] {
			conflictos = append(conflictos, f)
			seen[f] = true
		}
	}
	if len(conflictos) == 0 {
		conflictos = []string{fmt.Sprintf("%s ↔ %s", ramaA, ramaB)}
	}
	return conflictos, nil
}

func isExitError(err error, target **exec.ExitError) bool {
	if ee, ok := err.(*exec.ExitError); ok {
		*target = ee
		return true
	}
	return false
}

func simbolosComunes(attemptsA, attemptsB []loop.Attempt) []string {
	symA := ultimosSimbolos(attemptsA)
	symB := ultimosSimbolos(attemptsB)
	if len(symA) == 0 || len(symB) == 0 {
		return nil
	}
	setA := make(map[string]bool, len(symA))
	for _, s := range symA {
		setA[s] = true
	}
	seen := map[string]bool{}
	var out []string
	for _, s := range symB {
		if setA[s] && !seen[s] {
			out = append(out, s)
			seen[s] = true
		}
	}
	return out
}

func ultimosSimbolos(as []loop.Attempt) []string {
	for i := len(as) - 1; i >= 0; i-- {
		if as[i].SimbolosExportados != nil {
			return *as[i].SimbolosExportados
		}
	}
	return nil
}
