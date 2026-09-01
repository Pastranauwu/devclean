package loop

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/Pastranauwu/devclean/internal/config"
	"github.com/Pastranauwu/devclean/internal/room"
	"github.com/Pastranauwu/devclean/internal/sealed"
	"github.com/Pastranauwu/devclean/internal/task"
)

// DefaultTimeout is the fallback for PruebaTimeout when Options no lo fija.
const DefaultTimeout = 5 * time.Minute

// DefaultAgentTimeout is the fallback for AgentTimeout. Más largo que
// DefaultTimeout a propósito: una invocación real de agente (edición
// multi-archivo, no solo correr pruebas) tarda más que una corrida de
// pruebas, y cortarla a los 5 minutos quemaba el intento antes de que el
// agente terminara — causa habitual de "se agotaron los intentos" sin PR.
const DefaultAgentTimeout = 20 * time.Minute

// Agent es lo mínimo que el bucle pide al ejecutor: una invocación por
// intento y el gasto de tokens que produjo. La interfaz se declara aquí,
// del lado del consumidor; los adaptadores de internal/executor la
// satisfacen con un shim en cmd/run, sin que el bucle dependa de un
// paquete en obra.
type Agent interface {
	Name() string
	Run(ctx context.Context, req Request) (Result, error)
}

// Examinador genera la suite de pruebas ciega antes de que el
// implementador empiece (§6.8). La interfaz vive en loop para evitar
// un ciclo de importación con el paquete examiner.
type Examinador interface {
	Run(ctx context.Context, roomPath string) (bool, error)
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
	Stderr   string // diagnóstico del CLI: por qué no llegó al modelo
	Text     string // respuesta textual del agente (campo "result" del JSON de claude)
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

	// Interfaces son las firmas congeladas de las tareas hermanas que
	// esta consume (§6.10), ya resueltas por el que llama. Sin esto el
	// agente inventa la firma: las tareas de una misma oleada corren en
	// cuartos separados y no pueden leerse entre sí.
	Interfaces []string

	// Constitucion es el contenido de .devclean/constitution.md (§6.11),
	// ya cargado por quien llama. Si está vacío, no se inyecta nada.
	Constitucion string

	// Skills son las habilidades del rol asignado (§8.1 / Fase 1), solo
	// nombres — etiquetas descriptivas en el prompt.
	Skills []string

	// SkillsContenido es el cuerpo completo de los paquetes de skill real
	// (internal/skills) resueltos para este rol, ya cargado por quien
	// llama. Vacío si no hay ninguno instalado.
	SkillsContenido string

	// OnIntento, si no es nil, avisa cuándo empieza y termina cada
	// intento. Es la única señal de vida en modo plano: una invocación
	// de agente puede tardar veinte minutos y hasta ahora no imprimía
	// nada hasta el final de la corrida entera.
	OnIntento func(intento int, fase string)

	// Examinador, si no es nil, se invoca una vez antes del bucle del
	// implementador para escribir la suite visible y sellar la oculta
	// (§6.8). Si falla, el bucle continúa sin pruebas ciegas.
	Examinador Examinador
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
		o.AgentTimeout = DefaultAgentTimeout
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

	// el latido es el único estado en vivo: attempts.jsonl no se escribe
	// hasta que el intento termina, y un intento puede durar veinte
	// minutos. Se borra al salir, pase lo que pase.
	var acumulado Tokens
	avisar := func(intento int, fase string) {
		EscribirLatido(o.Root, Latido{
			ID: o.Task.ID, Intento: intento, Limite: limite, Fase: fase,
			Modelo: o.Model, DesdeFase: time.Now().UTC(), Tokens: acumulado,
		})
		if o.OnIntento != nil {
			o.OnIntento(intento, fase)
		}
	}
	defer BorrarLatido(o.Root, o.Task.ID)

	// una suite sellada a mano (devclean task seal) manda sobre el
	// examinador automático: el usuario ya pagó esas pruebas y volver a
	// generarlas las pisaría.
	if sealed.Exists(o.Root, o.Task.ID) {
		suiteManualEnCuarto(o.Root, o.Task.ID, o.Room.Path)
	} else if o.Examinador != nil {
		avisar(1, FaseExamen)
		_, _ = o.Examinador.Run(ctx, o.Room.Path) // graceful degradation: never blocks
	}

	var prevErr string
	for intento := 1; intento <= limite; intento++ {
		inicio := time.Now().UTC()
		avisar(intento, FaseAgente)

		req := Request{
			RoomPath:     o.Room.Path,
			Prompt:       promptPara(o.Task, o.Interfaces, o.Constitucion, o.Skills, o.SkillsContenido, prevErr),
			AllowedGlobs: o.Task.TocarSolo,
			Model:        o.Model,
			Timeout:      o.AgentTimeout,
			Env:          o.Env,
		}
		res, agentErr := o.Agent.Run(ctx, req)
		logRel := guardarLog(o.Root, o.Task.ID, intento, req.Prompt, res, agentErr)

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

		avisar(intento, FaseVerificando)
		salida, code := runPrueba(ctx, o.Room.Path, o.Task.ListoCuando, o.PruebaTimeout)
		pasaron, fallaron := ParseTestCounts(salida)
		fin := time.Now().UTC()

		// los campos de lista son arrays en el contrato, no null: un
		// intento sin cambios ni reversiones es [], no nada
		if archivos == nil {
			archivos = []string{}
		}
		if revertidos == nil {
			revertidos = []string{}
		}

		acumulado.Entrada += res.Tokens.Entrada
		acumulado.Salida += res.Tokens.Salida

		codigoAgente := res.ExitCode
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
			AgenteSalidaCodigo:       &codigoAgente,
			Log:                      logRel,
		}
		if agentErr != nil {
			a.ErrorAgente = diagnostico(res, agentErr)
		}
		if err := s.Append(a); err != nil {
			return Outcome{}, err
		}

		if code != nil && *code == 0 {
			return Outcome{Verde: true, Intentos: intento}, nil
		}

		// el agente no llegó al modelo: reintentar no cambia nada, y
		// gastar los intentos restantes solo retrasa el diagnóstico
		if falloDeInfra(res, agentErr, archivos) {
			motivo := fmt.Sprintf("el agente no pudo ejecutarse (%s) · revisa el modelo y la key con devclean doctor", a.ErrorAgente)
			if logRel != "" {
				motivo += " · detalle en " + logRel
			}
			return Outcome{Verde: false, Intentos: intento, UltimoError: a.ErrorAgente, Pregunta: motivo}, nil
		}

		prevErr = resumenFallo(o.Task.ListoCuando, code, salida)
		if agentErr != nil {
			prevErr = strings.TrimSpace(a.ErrorAgente + " · " + prevErr)
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

// suiteManualEnCuarto copia al cuarto la suite visible que el usuario
// selló con `devclean task seal`. Aterriza en la misma ruta que usaría el
// examinador automático, así que de acá para abajo nadie distingue el
// origen. Degrada igual que el examinador: si algo falla, el implementador
// corre sin suite visible, pero nunca se lo frena (§6.8).
func suiteManualEnCuarto(root, id, roomPath string) {
	s, err := sealed.Read(root, id)
	if err != nil || s.Visible == "" || s.ArchivoVisible == "" {
		return
	}
	abs := filepath.Join(roomPath, filepath.FromSlash(s.ArchivoVisible))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return
	}
	if err := os.WriteFile(abs, []byte(s.Visible), 0o644); err != nil {
		return
	}
	// commitear o revertFueraDeAlcance (A.3) la borra en el primer
	// intento: git status lista lo que cambió, no lo ya commiteado.
	_, _ = gitRun(roomPath, "add", s.ArchivoVisible)
	_, _ = gitRun(roomPath, "-c", "user.name=devclean", "-c", "user.email=devclean@local",
		"commit", "-m", "exam: suite visible sellada a mano")
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
func promptPara(t task.Task, interfaces []string, constitucion string, skills []string, skillsContenido string, prevErr string) string {
	var b strings.Builder
	if constitucion != "" {
		fmt.Fprintf(&b, "Constitución del proyecto (convenciones que todos los agentes deben seguir):\n%s\n\n", constitucion)
	}
	if len(skills) > 0 {
		fmt.Fprintf(&b, "Habilidades de este rol: %s\n\n", strings.Join(skills, ", "))
	}
	if skillsContenido != "" {
		fmt.Fprintf(&b, "Skills instaladas para este rol — sigue sus instrucciones:\n%s\n\n", skillsContenido)
	}
	fmt.Fprintf(&b, "Tarea %s: %s\n", t.ID, t.Titulo)
	if t.Porque != "" {
		fmt.Fprintf(&b, "Por qué: %s\n", t.Porque)
	}
	fmt.Fprintf(&b, "Listo cuando: %s\n", t.ListoCuando)
	if len(t.TocarSolo) > 0 {
		fmt.Fprintf(&b, "Solo puedes tocar: %s\n", strings.Join(t.TocarSolo, ", "))
	}
	// §6.10: lo que esta tarea debe exponer y lo que otras ya le
	// garantizan. Son contrato, no sugerencia: la esclusa de salida
	// verifica que `expone` aparezca en el diff.
	if len(t.Expone) > 0 {
		fmt.Fprintf(&b, "Debes exponer estas firmas exactas (otras tareas ya las consumen): %s\n", strings.Join(t.Expone, "; "))
	}
	if len(interfaces) > 0 {
		fmt.Fprintf(&b, "Otras tareas exponen esto; úsalo tal cual, no lo redefinas: %s\n", strings.Join(interfaces, "; "))
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

// resumenFallo describe por qué falló listo_cuando, para el agente y para
// el usuario.
//
// Un comando que no imprime nada es normal y frecuente: el planificador
// escribe cosas como `go run . --help | grep -q -- --mac`, y `grep -q`
// calla por diseño. Antes eso producía literalmente "sin salida", que no
// le dice nada a nadie: el agente recibía ese texto como único dato del
// intento anterior y se quedaba sin saber qué arreglar. Diciendo el
// comando y su código de salida, al menos hay de dónde agarrarse.
func resumenFallo(listoCuando string, code *int, salida string) string {
	if cuerpo := ultimasLineas(salida, 6, 600); cuerpo != "" {
		return cuerpo
	}
	estado := "falló"
	if code != nil {
		estado = fmt.Sprintf("salió con código %d", *code)
	}
	msg := fmt.Sprintf("listo_cuando %s sin imprimir nada · comando: %s", estado, strings.TrimSpace(listoCuando))
	if silencioso(listoCuando) {
		msg += " · el comando silencia su salida, así que no hay pista del fallo: córrelo a mano sin -q para ver qué falta"
	}
	return msg
}

// silencioso reporta si el comando esconde su propia salida.
func silencioso(cmd string) bool {
	for _, patron := range []string{"grep -q", "-q --", "--quiet", "> /dev/null", ">/dev/null", "&>/dev/null"} {
		if strings.Contains(cmd, patron) {
			return true
		}
	}
	return false
}

// ultimasLineas devuelve las últimas n líneas no vacías de s, acotadas a
// max caracteres. Cadena vacía si no hay nada que mostrar.
func ultimasLineas(s string, n, max int) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	var utiles []string
	for _, l := range strings.Split(s, "\n") {
		if strings.TrimSpace(l) != "" {
			utiles = append(utiles, strings.TrimRight(l, " \t"))
		}
	}
	if len(utiles) == 0 {
		return ""
	}
	if len(utiles) > n {
		utiles = utiles[len(utiles)-n:]
	}
	out := strings.Join(utiles, "\n")
	if len(out) > max {
		out = "…" + out[len(out)-max:]
	}
	return out
}
