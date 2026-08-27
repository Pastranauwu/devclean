package loop

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/Pastranauwu/devclean/internal/config"
	"github.com/Pastranauwu/devclean/internal/room"
	"github.com/Pastranauwu/devclean/internal/task"
)

// DefaultTimeout is the fallback when Options no fija un timeout.
const DefaultTimeout = 5 * time.Minute

// Agent es lo mínimo que el bucle pide al ejecutor: una invocación por
// intento y el gasto de tokens que produjo. La interfaz se declara aquí,
// del lado del consumidor; los adaptadores de internal/executor la
// satisfacen con un shim en cmd/run, sin que el bucle dependa de un
// paquete en obra.
type Agent interface {
	Name() string
	Run(ctx context.Context, req Request) (Result, error)
}

// Request es una invocación del agente dentro de su cuarto.
type Request struct {
	RoomPath     string
	Prompt       string
	AllowedGlobs []string
	Model        string
	Timeout      time.Duration
	Env          []string
}

// Result es lo que devolvió una invocación.
type Result struct {
	Stdout   string
	ExitCode int
	Tokens   Tokens
}

// Options lleva las dependencias del bucle.
type Options struct {
	Agent          Agent
	Root           string
	Room           room.Room
	Task           task.Task
	Model          string
	Base           string        // rama base para el diff de símbolos
	PatronesPrueba []string      // rutas de prueba que nunca se editan
	AgentTimeout   time.Duration // timeout de cada invocación del agente
	PruebaTimeout  time.Duration // timeout de cada listo_cuando
	Env            []string      // variables propias del cuarto (PORT, ...)
}

// Outcome es el resultado de correr el bucle sobre una tarea.
type Outcome struct {
	Verde       bool   `json:"verde"`
	Intentos    int    `json:"intentos"`
	UltimoError string `json:"ultimo_error,omitempty"`
	Pregunta    string `json:"pregunta,omitempty"`
}

// Run dirige el bucle de §6.4 e instrumenta cada intento (adenda A.2).
// Devuelve el Outcome; solo error si el mecanismo falla (no si la tarea
// se agotó: detenerse es un resultado válido, no un fallo).
func Run(ctx context.Context, o Options) (Outcome, error) {
	if o.Agent == nil {
		return Outcome{}, errors.New("bucle sin ejecutor · elige uno con devclean doctor")
	}
	if o.Room.Path == "" {
		return Outcome{}, errors.New("bucle sin cuarto · crea el worktree antes de correr")
	}
	if o.AgentTimeout <= 0 {
		o.AgentTimeout = DefaultTimeout
	}
	if o.PruebaTimeout <= 0 {
		o.PruebaTimeout = DefaultTimeout
	}
	if o.PatronesPrueba == nil {
		o.PatronesPrueba = config.DefaultTestPatterns()
	}
	limite := o.Task.LimiteIntentos
	if limite < 1 {
		limite = task.DefaultLimiteIntentos
	}

	base := o.Base
	if base == "" {
		base = "HEAD"
	}
	if hash, err := resolveCommit(o.Room.Path, base); err == nil {
		base = hash
	}

	s, err := openStore(o.Root, o.Task.ID)
	if err != nil {
		return Outcome{}, err
	}

	var prevErr string
	for intento := 1; intento <= limite; intento++ {
		inicio := time.Now().UTC()

		req := Request{
			RoomPath:     o.Room.Path,
			Prompt:       promptPara(o.Task, prevErr),
			AllowedGlobs: o.Task.TocarSolo,
			Model:        o.Model,
			Timeout:      o.AgentTimeout,
			Env:          o.Env,
		}
		res, agentErr := o.Agent.Run(ctx, req)

		revertidos, err := revertFueraDeAlcance(o.Room.Path, o.Task.TocarSolo, o.PatronesPrueba)
		if err != nil {
			return Outcome{}, err
		}
		// indexar lo que queda (todo dentro de alcance) para que los
		// archivos nuevos del intento cuenten en el diff
		if _, err := gitRun(o.Room.Path, "add", "-A"); err != nil {
			return Outcome{}, err
		}
		archivos, err := stagedFiles(o.Room.Path)
		if err != nil {
			return Outcome{}, err
		}
		mas, menos, err := stagedNumstat(o.Room.Path)
		if err != nil {
			return Outcome{}, err
		}
		simbolos, err := simbolosExportados(o.Room.Path, base)
		if err != nil {
			return Outcome{}, err
		}

		// el punto de restauración se guarda antes de verificar, para
		// que el trabajo verde quede commiteado y ship pueda aplanarlo
		if err := commitWip(o.Room.Path, o.Task.ID, intento); err != nil {
			return Outcome{}, err
		}

		salida, code := runPrueba(ctx, o.Room.Path, o.Task.ListoCuando, o.PruebaTimeout)
		pasaron, fallaron := parseTestCounts(salida)
		fin := time.Now().UTC()

		// los campos de lista son arrays en el contrato, no null: un
		// intento sin cambios ni reversiones es [], no nada
		if archivos == nil {
			archivos = []string{}
		}
		if revertidos == nil {
			revertidos = []string{}
		}

		a := Attempt{
			Intento:                  intento,
			Inicio:                   inicio,
			Fin:                      fin,
			SalidaCodigo:             code,
			TestsPasaron:             pasaron,
			TestsFallaron:            fallaron,
			ArchivosTocados:          archivos,
			SimbolosExportados:       simbolos,
			LineasMas:                mas,
			LineasMenos:              menos,
			RevertidosFueraDeAlcance: revertidos,
			Tokens:                   res.Tokens,
			Modelo:                   o.Model,
		}
		if err := s.Append(a); err != nil {
			return Outcome{}, err
		}

		if code != nil && *code == 0 {
			return Outcome{Verde: true, Intentos: intento}, nil
		}

		prevErr = tailSalida(salida)
		if agentErr != nil {
			prevErr = strings.TrimSpace(agentErr.Error() + " · " + prevErr)
		}
	}

	pregunta := fmt.Sprintf("agotó %d intentos · falla: %s", limite, prevErr)
	return Outcome{
		Verde:       false,
		Intentos:    limite,
		UltimoError: prevErr,
		Pregunta:    pregunta,
	}, nil
}

// commitWip guarda el punto de restauración interno: un commit `wip:` en
// la rama del cuarto. Sin cambios que guardar, no hace nada. Los wip son
// basura intencional que ship aplana; nunca llegan al PR (§6.4).
func commitWip(roomPath, id string, intento int) error {
	if _, err := gitRun(roomPath, "diff", "--cached", "--quiet"); err == nil {
		return nil // nada que guardar
	}
	msg := fmt.Sprintf("wip: %s intento %d", id, intento)
	if _, err := gitRun(roomPath, "-c", "user.name=devclean", "-c", "user.email=devclean@local", "commit", "-m", msg); err != nil {
		return fmt.Errorf("no se pudo guardar el punto de restauración · %s", err)
	}
	return nil
}

// runPrueba ejecuta listo_cuando dentro del cuarto y devuelve la salida
// combinada y su código de salida (nil si ni siquiera arrancó).
func runPrueba(ctx context.Context, dir, cmdStr string, timeout time.Duration) (string, *int) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/c", cmdStr)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", cmdStr)
	}
	cmd.Dir = dir

	out, err := cmd.CombinedOutput()
	code := 0
	if ctx.Err() == context.DeadlineExceeded {
		code = 124
		return string(out), &code
	}
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			code = exitErr.ExitCode()
			return string(out), &code
		}
		return string(out) + err.Error(), nil
	}
	return string(out), &code
}

// promptPara arma el prompt de un intento: el contrato y, si lo hay, la
// salida del intento anterior. El agente nunca decide si terminó.
func promptPara(t task.Task, prevErr string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Tarea %s: %s\n", t.ID, t.Titulo)
	if t.Porque != "" {
		fmt.Fprintf(&b, "Por qué: %s\n", t.Porque)
	}
	fmt.Fprintf(&b, "Listo cuando: %s\n", t.ListoCuando)
	if len(t.TocarSolo) > 0 {
		fmt.Fprintf(&b, "Solo puedes tocar: %s\n", strings.Join(t.TocarSolo, ", "))
	}
	if t.Riesgos != "" {
		fmt.Fprintf(&b, "Riesgos: %s\n", t.Riesgos)
	}
	if t.Notas != "" {
		fmt.Fprintf(&b, "Notas:\n%s\n", t.Notas)
	}
	if prevErr != "" {
		fmt.Fprintf(&b, "\nEl intento anterior falló:\n%s\n", prevErr)
	}
	return b.String()
}

// tailSalida reduce la salida de listo_cuando a la última línea útil,
// para devolvérsela al agente y para la pregunta concreta final.
func tailSalida(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "sin salida"
	}
	lines := strings.Split(s, "\n")
	last := strings.TrimSpace(lines[len(lines)-1])
	if last == "" && len(lines) > 1 {
		last = strings.TrimSpace(lines[len(lines)-2])
	}
	if last == "" {
		return "sin salida"
	}
	const max = 160
	if len(last) > max {
		last = last[:max] + "…"
	}
	return last
}
