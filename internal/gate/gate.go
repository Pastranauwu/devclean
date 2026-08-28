// Package gate implements the entry gate of §6.3: the four checks a
// task must pass before devclean spends a single token on it.
//
//  0. el contrato es válido (version, id, límites)
//  1. listo_cuando existe y se ejecuta
//  2. el comando falla hoy (si ya pasa, la tarea no tiene sentido)
//  3. tocar_solo se declara si hace falta y no se cruza con el de otra
//  4. tocar_solo no incluye zonas prohibidas globales ni rutas de prueba
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
	Aviso    string  `json:"aviso,omitempty"`
}

// Cruce runs only the scope check between tasks (§6.3.3 y A.4):
// rejects when tocar_solo overlaps another's, or is empty while another
// task runs. cmd/run uses it at assignment time, where the tasks already
// passed the full gate, so no test command runs again.
func Cruce(t task.Task, otras []task.Task) Check {
	return checkSinCruce(t, otras)
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

// Run executes the five checks in order. Check 2 runs listo_cuando for
// real, with timeout, in the repository root. otras carries only the
// tasks already en_curso, which is what decides whether tocar_solo may
// stay empty (adenda A.4).
func Run(ctx context.Context, root string, cfg config.Config, t task.Task, otras []task.Task, timeout time.Duration) Result {
	res := Result{ID: t.ID}
	res.Aviso = t.Aviso

	// el contrato se valida primero: sin version ni id no hay tarea
	// que evaluar (adenda A.1). Los demás chequeos corren igual, para
	// que un solo paso liste todo lo que hay que arreglar.
	res.Chequeos = append(res.Chequeos, checkContrato(t))

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
	res.Chequeos = append(res.Chequeos, checkRutasDePrueba(t, cfg.PatronesPrueba))

	res.Aprobada = true
	for _, c := range res.Chequeos {
		if !c.OK {
			res.Aprobada = false
			break
		}
	}
	return res
}

// checkContrato: the contract itself must be valid before any of the
// runtime checks mean anything.
func checkContrato(t task.Task) Check {
	errs := t.Validate()
	if len(errs) == 0 {
		return Check{"contrato válido", true, ""}
	}
	motivos := make([]string, len(errs))
	for i, e := range errs {
		motivos[i] = e.Error()
	}
	return Check{"contrato válido", false, strings.Join(motivos, " · ")}
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

// checkSinCruce: tocar_solo must not overlap with any other active
// task's. The caller passes only tasks in state en_curso (§6.3).
func checkSinCruce(t task.Task, otras []task.Task) Check {
	// A.4: sin alcance declarado no hay cruce que detectar, así que
	// vacío solo vale mientras esta sea la única tarea en curso
	if len(t.TocarSolo) == 0 {
		for _, o := range otras {
			if o.ID != t.ID {
				return Check{"sin cruce", false, "tocar_solo obligatorio con más de una tarea activa · declara tus rutas"}
			}
		}
		return Check{"sin cruce", true, ""}
	}
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

// AlcanceProhibido reporta si una ruta de tocar_solo cae en una zona
// prohibida global (§6.3: lockfiles, migraciones, CI, changelog) o en
// una ruta de prueba (adenda A.3: las escribe el examinador ciego), y
// devuelve cuál. `plan` la usa para recortar el alcance antes de
// escribir el contrato: el planificador es un modelo y a veces las
// incluye, y un rechazo en la esclusa no tiene arreglo salvo editar a
// mano, que es justo lo que devclean evita (§6.1).
func AlcanceProhibido(p string, zonas, patronesPrueba []string) (string, bool) {
	for _, z := range zonas {
		if globsOverlap(p, z) {
			return z, true
		}
	}
	if len(patronesPrueba) == 0 {
		patronesPrueba = config.DefaultTestPatterns()
	}
	for _, patron := range patronesPrueba {
		if esRutaDePrueba(p, patron) {
			return patron, true
		}
	}
	return "", false
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

// checkRutasDePrueba: tocar_solo must never reach the test files
// (adenda A.3). En v0.2 las escribe un examinador ciego; si el
// implementador puede editarlas, el examen no vale nada.
func checkRutasDePrueba(t task.Task, patrones []string) Check {
	if len(patrones) == 0 {
		patrones = config.DefaultTestPatterns()
	}
	for _, patron := range patrones {
		for _, p := range t.TocarSolo {
			if esRutaDePrueba(p, patron) {
				return Check{"sin rutas de prueba", false, fmt.Sprintf("tocar_solo incluye rutas de prueba (%s casa con %s) · las pruebas no las escribe quien implementa", p, patron)}
			}
		}
	}
	return Check{"sin rutas de prueba", true, ""}
}

// esRutaDePrueba reports whether the tocar_solo entry p aims at test
// files. Deliberately narrower than globsOverlap: `src/export/**`
// contains test files but does not declare them, and rejecting it would
// reject every sane contract. Lo que sí se rechaza es apuntarles:
// `src/export/*_test.go` o cualquier ruta bajo `test/**`.
func esRutaDePrueba(p, patron string) bool {
	if dir := dirPrefix(patron); dir != "" {
		return within(dir, p) || within(dir, path.Base(p))
	}
	if ok, _ := path.Match(patron, path.Base(p)); ok {
		return true
	}
	return false
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
