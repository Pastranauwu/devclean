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

// mergeTree runs git merge-tree and returns the conflicting file paths.
//
// Con --write-tree --no-messages la salida es el OID del árbol escrito y,
// cuando hay conflicto, una línea por etapa sin fusionar en formato
// "<modo> <oid> <etapa>\t<ruta>". Esas líneas son estables y no dependen
// del idioma de la terminal — los mensajes "CONFLICT (content)" sí (en
// español salen "CONFLICTO (contenido)"), y el parseo por prefijo no los
// encontraba: falso negativo que escondía el archivo en conflicto.
//
// Devuelve nil, nil tanto en fusión limpia como cuando no se pudo
// comparar (rama inexistente, git viejo): una rama que aún no existe no
// es un conflicto, y reportarlo como tal era el falso positivo
// "devclean/T-001 ↔ devclean/T-002" que se veía antes del primer wip:.
func mergeTree(root, ramaA, ramaB string) ([]string, error) {
	cmd := exec.Command("git", "-C", root, "merge-tree", "--write-tree", "--no-messages", ramaA, ramaB)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	err := cmd.Run()
	if err == nil {
		return nil, nil // fusión limpia
	}
	var exitErr *exec.ExitError
	if !isExitError(err, &exitErr) || exitErr.ExitCode() != 1 {
		return nil, nil // no es un conflicto: no se pudo comparar
	}
	// exit 1 puede ser conflicto real o "no se pudo fusionar" (rama
	// inexistente). Lo distingue la salida: un conflicto trae líneas de
	// etapa; un error trae un mensaje y nada más.
	var conflictos []string
	seen := map[string]bool{}
	for _, line := range strings.Split(stdout.String(), "\n") {
		ruta, ok := parseLineaEtapa(line)
		if ok && !seen[ruta] {
			conflictos = append(conflictos, ruta)
			seen[ruta] = true
		}
	}
	return conflictos, nil
}

// parseLineaEtapa extrae la ruta de una línea de etapa sin fusionar de
// merge-tree: "<modo> <oid> <etapa>\t<ruta>". La etapa (1 base, 2 la
// nuestra, 3 la suya) es lo que la distingue del resto de la salida.
func parseLineaEtapa(line string) (string, bool) {
	meta, ruta, ok := strings.Cut(line, "\t")
	if !ok || ruta == "" {
		return "", false
	}
	campos := strings.Fields(meta)
	if len(campos) != 3 {
		return "", false
	}
	switch campos[2] {
	case "1", "2", "3":
		return ruta, true
	}
	return "", false
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
