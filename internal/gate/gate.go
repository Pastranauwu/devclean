// Package gate implements the entry gate of §6.3: the four checks a
// task must pass before devclean spends a single token on it.
//
//  1. listo_cuando existe y se ejecuta
//  2. el comando falla hoy (si ya pasa, la tarea no tiene sentido)
//  3. tocar_solo no se cruza con el de otra tarea
//  4. tocar_solo no incluye zonas prohibidas globales
//
// All checks are pure code: no model is involved in verification.
package gate

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/Pastranauwu/devclean/internal/config"
	"github.com/Pastranauwu/devclean/internal/task"
)

// DefaultTimeout is the maximum time check 2 gives listo_cuando to run.
const DefaultTimeout = 5 * time.Minute

// Check is the result of one gate check.
type Check struct {
	Nombre string `json:"nombre"`
	OK     bool   `json:"ok"`
	Motivo string `json:"motivo,omitempty"`
}

// Result is the outcome of running the gate on one task.
type Result struct {
	ID       string  `json:"id"`
	Aprobada bool    `json:"aprobada"`
	Chequeos []Check `json:"chequeos"`
}

// PrimerMotivo returns the reason of the first failed check.
func (r Result) PrimerMotivo() string {
	for _, c := range r.Chequeos {
		if !c.OK {
			return c.Motivo
		}
	}
	return ""
}

// Run executes the four checks in order. Check 2 runs listo_cuando for
// real, with timeout, in the repository root. An empty tocar_solo
// declares no scope, so checks 3 and 4 have nothing to evaluate.
func Run(ctx context.Context, root string, cfg config.Config, t task.Task, otras []task.Task, timeout time.Duration) Result {
	res := Result{ID: t.ID}

	ejecutable := checkEjecutable(root, t)
	res.Chequeos = append(res.Chequeos, ejecutable)
	if ejecutable.OK {
		res.Chequeos = append(res.Chequeos, checkFallaHoy(ctx, root, t.ListoCuando, timeout))
	} else {
		res.Chequeos = append(res.Chequeos, Check{
			Nombre: "falla hoy",
			OK:     false,
			Motivo: "no evaluado · listo_cuando no es ejecutable",
		})
	}
	res.Chequeos = append(res.Chequeos, checkSinCruce(t, otras))
	res.Chequeos = append(res.Chequeos, checkZonasProhibidas(t, cfg.ZonasProhibidas))

	res.Aprobada = true
	for _, c := range res.Chequeos {
		if !c.OK {
			res.Aprobada = false
			break
		}
	}
	return res
}

// checkEjecutable: listo_cuando exists and names something runnable.
func checkEjecutable(root string, t task.Task) Check {
	cmdStr := strings.TrimSpace(t.ListoCuando)
	if cmdStr == "" {
		return Check{"ejecutable", false, "falta listo_cuando · escribe el comando que dice \"ya está\""}
	}
	bin := strings.Fields(cmdStr)[0]
	if strings.ContainsRune(bin, '/') {
		abs := bin
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(root, bin)
		}
		info, err := os.Stat(abs)
		if err != nil || info.IsDir() || info.Mode()&0o111 == 0 {
			return Check{"ejecutable", false, fmt.Sprintf("comando no encontrado: %s · instálalo o corrige listo_cuando", bin)}
		}
		return Check{"ejecutable", true, ""}
	}
	if _, err := exec.LookPath(bin); err != nil {
		return Check{"ejecutable", false, fmt.Sprintf("comando no encontrado: %s · instálalo o corrige listo_cuando", bin)}
	}
	return Check{"ejecutable", true, ""}
}

// checkFallaHoy: the command must fail today. It runs for real with a
// timeout; a zero exit rejects the task as pointless.
func checkFallaHoy(ctx context.Context, root, listoCuando string, timeout time.Duration) Check {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/c", listoCuando)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", listoCuando)
	}
	cmd.Dir = root

	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return Check{"falla hoy", false, fmt.Sprintf("listo_cuando tardó más de %s · acota el comando", timeout)}
	}
	if err == nil {
		return Check{"falla hoy", false, "listo_cuando ya pasa · la tarea no tiene sentido"}
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return Check{"falla hoy", true, ""}
	}
	return Check{"falla hoy", false, fmt.Sprintf("no se pudo ejecutar listo_cuando: %v", err)}
}

// checkSinCruce: tocar_solo must not overlap with any other task's.
// In this phase every task on disk counts; rooms and states arrive
// with the execution engine.
func checkSinCruce(t task.Task, otras []task.Task) Check {
	for _, o := range otras {
		if o.ID == t.ID {
			continue
		}
		for _, a := range t.TocarSolo {
			for _, b := range o.TocarSolo {
				if globsOverlap(a, b) {
					return Check{"sin cruce", false, fmt.Sprintf("tocar_solo se cruza con %s en %s · ajusta los alcances", o.ID, a)}
				}
			}
		}
	}
	return Check{"sin cruce", true, ""}
}

// checkZonasProhibidas: tocar_solo must not reach the global forbidden
// zones (§6.3: lockfiles, migrations, CI, changelog).
func checkZonasProhibidas(t task.Task, zonas []string) Check {
	for _, z := range zonas {
		for _, p := range t.TocarSolo {
			if globsOverlap(p, z) {
				return Check{"zonas prohibidas", false, fmt.Sprintf("tocar_solo incluye la zona prohibida %s · quita %s o ajusta el alcance", z, p)}
			}
		}
	}
	return Check{"zonas prohibidas", true, ""}
}

// globsOverlap reports whether two patterns can match the same file.
// Conservative: when in doubt, they overlap.
func globsOverlap(a, b string) bool {
	if a == b {
		return true
	}
	if dir := dirPrefix(a); dir != "" && within(dir, b) {
		return true
	}
	if dir := dirPrefix(b); dir != "" && within(dir, a) {
		return true
	}
	if ok, _ := path.Match(a, b); ok {
		return true
	}
	if ok, _ := path.Match(b, a); ok {
		return true
	}
	ha, hb := literalHead(a), literalHead(b)
	return strings.HasPrefix(ha, hb) || strings.HasPrefix(hb, ha)
}

// dirPrefix returns the directory a pattern covers when it ends in
// /** or /*, or empty otherwise.
func dirPrefix(p string) string {
	if strings.HasSuffix(p, "/**") {
		return strings.TrimSuffix(p, "/**")
	}
	if strings.HasSuffix(p, "/*") {
		return strings.TrimSuffix(p, "/*")
	}
	return ""
}

// within reports whether pattern p points inside directory dir.
func within(dir, p string) bool {
	return p == dir || strings.HasPrefix(p, dir+"/")
}

// literalHead returns the pattern up to its first wildcard.
func literalHead(p string) string {
	i := strings.IndexAny(p, "*?[")
	if i == -1 {
		return p
	}
	return p[:i]
}
