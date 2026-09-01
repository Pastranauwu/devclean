// Package recurse implementa la ejecución recursiva de tareas (§8.3): una
// tarea marcada `recursivo: true` no la resuelve un solo intento de
// agente — se reparte en subtareas reales, cada una con su propio
// contrato y su propio `listo_cuando`, que corren en cuartos anidados
// DENTRO del cuarto de la tarea padre.
//
// Un cuarto ya es un worktree completo del repo: los worktrees no
// comparten archivos sin commitear entre sí, así que un cuarto anidado
// adentro de otro cuarto sale gratis — "git propio" para cada subtarea
// sin reinventar nada, mismo mecanismo de siempre (internal/room).
//
// Agent implementa loop.Agent: para el bucle padre, la descomposición
// entera es "lo que hizo el agente en este intento". El bucle no cambia
// una línea — sigue verificando `listo_cuando` del padre después, con
// código, nunca con el juicio del modelo (manifiesto regla 3).
package recurse

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/Pastranauwu/devclean/internal/config"
	"github.com/Pastranauwu/devclean/internal/gate"
	"github.com/Pastranauwu/devclean/internal/loop"
	"github.com/Pastranauwu/devclean/internal/plan"
	"github.com/Pastranauwu/devclean/internal/room"
	"github.com/Pastranauwu/devclean/internal/task"
)

// Agent descompone una tarea recursiva y ejecuta sus subtareas.
type Agent struct {
	Cfg            config.Config
	Constitucion   string
	Planificador   plan.Generador // rol planificador (modelo caro): decompone
	Ejecutor       loop.Agent     // agente hoja (modelo barato) para subtareas no recursivas
	ModeloEjecutor string
	Task           task.Task // la tarea recursiva que este Agent resuelve
	Profundidad    int       // 0 = raíz

	// Root es la raíz del proyecto (no el cuarto): ahí vive arbol.json,
	// para que sobreviva a que el cuarto de la subtarea se destruya.
	Root string
	// RaizID es la tarea de nivel 0 que arrancó la recursión — todo nodo,
	// a cualquier profundidad, se guarda bajo su arbol.json, así
	// board/standup/TUI leen un solo archivo por tarea raíz.
	RaizID string
}

// Name identifica al agente en attempts.jsonl y en los logs.
func (a Agent) Name() string { return "recursivo" }

// Run descompone la tarea en subtareas, las corre una por una en cuartos
// anidados y las integra a la rama de este cuarto. No abre PR ni decide
// nada por juicio propio: el bucle del padre corre `listo_cuando` después
// de esto, exactamente igual que si un solo agente hubiera escrito el
// código a mano.
func (a Agent) Run(ctx context.Context, req loop.Request) (loop.Result, error) {
	limite := a.Task.LimiteSubtareas
	if limite < 1 {
		limite = task.DefaultLimiteSubtareas
	}

	borradores, err := a.descomponer(ctx, req)
	if err != nil {
		return loop.Result{}, fmt.Errorf("descomposición de %s falló · %s", a.Task.ID, err)
	}
	if len(borradores) > limite {
		return loop.Result{}, fmt.Errorf("descomposición de %s propuso %d subtareas, el límite es %d · bajá el alcance o subí limite_subtareas", a.Task.ID, len(borradores), limite)
	}

	var tokens loop.Tokens
	for i, b := range borradores {
		sub, err := a.tareaDesdeBorrador(b, i+1)
		if err != nil {
			return loop.Result{}, fmt.Errorf("subtarea %d de %s inválida · %s", i+1, a.Task.ID, err)
		}

		gres := gate.Run(ctx, req.RoomPath, a.Cfg, sub, nil, gate.DefaultTimeout)
		if !gres.Aprobada {
			motivo := "rechazada en la esclusa de entrada · " + gres.PrimerMotivo()
			a.registrar(sub, false, 0, motivo, loop.Tokens{})
			return loop.Result{}, fmt.Errorf("subtarea %s %s", sub.ID, motivo)
		}

		r, err := room.Create(ctx, req.RoomPath, sub.ID, "HEAD")
		if err != nil {
			a.registrar(sub, false, 0, "no se pudo crear el cuarto · "+err.Error(), loop.Tokens{})
			return loop.Result{}, fmt.Errorf("no se pudo crear el cuarto de %s · %s", sub.ID, err)
		}

		outcome, agentErr, tk := a.correrSubtarea(ctx, req, r, sub)
		tokens.Entrada += tk.Entrada
		tokens.Salida += tk.Salida
		if agentErr != nil {
			a.registrar(sub, false, outcome.Intentos, agentErr.Error(), tk)
			_ = room.Destroy(ctx, req.RoomPath, sub.ID)
			return loop.Result{Tokens: tokens}, agentErr
		}
		if !outcome.Verde {
			a.registrar(sub, false, outcome.Intentos, outcome.Pregunta, tk)
			_ = room.Destroy(ctx, req.RoomPath, sub.ID)
			return loop.Result{Tokens: tokens}, fmt.Errorf("subtarea %s no llegó a verde · %s", sub.ID, outcome.Pregunta)
		}

		if err := integrar(req.RoomPath, r.Rama, sub.ID); err != nil {
			a.registrar(sub, false, outcome.Intentos, "no se pudo integrar · "+err.Error(), tk)
			_ = room.Destroy(ctx, req.RoomPath, sub.ID)
			return loop.Result{Tokens: tokens}, fmt.Errorf("no se pudo integrar %s a %s · %s", sub.ID, a.Task.ID, err)
		}
		a.registrar(sub, true, outcome.Intentos, "", tk)
		_ = room.Destroy(ctx, req.RoomPath, sub.ID)
	}

	return loop.Result{ExitCode: 0, Tokens: tokens}, nil
}

// registrar guarda el nodo de esta subtarea en arbol.json de la raíz.
// Best-effort: un fallo al escribir el árbol no debe frenar la
// recursión — el árbol es para que humanos lo miren, no parte del
// contrato de verificación.
func (a Agent) registrar(sub task.Task, verde bool, intentos int, motivo string, tk loop.Tokens) {
	if a.Root == "" || a.RaizID == "" {
		return
	}
	_ = AgregarNodo(a.Root, a.RaizID, NodoArbol{
		ID: sub.ID, Titulo: sub.Titulo, Padre: a.Task.ID,
		Profundidad: a.Profundidad + 1, Verde: verde, Intentos: intentos,
		Motivo: motivo, Tokens: tk,
	})
}

// correrSubtarea decide si la subtarea también recursa (si hay
// profundidad disponible) o corre plana con el agente hoja, y la ejecuta.
func (a Agent) correrSubtarea(ctx context.Context, req loop.Request, r room.Room, sub task.Task) (loop.Outcome, error, loop.Tokens) {
	agente := a.Ejecutor
	if sub.Recursivo && a.Profundidad+1 < a.Cfg.RecursionMax {
		agente = Agent{
			Cfg: a.Cfg, Constitucion: a.Constitucion,
			Planificador: a.Planificador,
			Ejecutor:     a.Ejecutor, ModeloEjecutor: a.ModeloEjecutor,
			Task: sub, Profundidad: a.Profundidad + 1,
			Root: a.Root, RaizID: a.RaizID,
		}
	}
	outcome, err := loop.Run(ctx, loop.Options{
		Agent:        agente,
		Root:         req.RoomPath,
		Room:         r,
		Task:         sub,
		Model:        a.ModeloEjecutor,
		Base:         "HEAD",
		Constitucion: a.Constitucion,
	})
	if err != nil {
		return loop.Outcome{}, err, loop.Tokens{}
	}
	tk, _ := ultimoTokens(req.RoomPath, sub.ID)
	return outcome, nil, tk
}

// ultimoTokens suma los tokens de todos los intentos registrados de una
// subtarea, para que el gasto de la recursión se vea reflejado en el
// intento del padre — sin esto, attempts.jsonl del padre subestima el
// costo real de la tarea.
func ultimoTokens(root, id string) (loop.Tokens, error) {
	attempts, err := loop.ReadAttempts(root, id)
	if err != nil {
		return loop.Tokens{}, err
	}
	var tk loop.Tokens
	for _, at := range attempts {
		tk.Entrada += at.Tokens.Entrada
		tk.Salida += at.Tokens.Salida
	}
	return tk, nil
}

// descomponer pide al planificador que parta la tarea en subtareas
// reales, reusando el mismo prompt y parser que `devclean plan` — la
// descomposición no es más que un plan cuyo alcance ya viene acotado al
// `tocar_solo` de la tarea padre.
func (a Agent) descomponer(ctx context.Context, req loop.Request) ([]plan.Borrador, error) {
	frase := a.Task.Titulo
	if a.Task.Porque != "" {
		frase += ". " + a.Task.Porque
	}
	limite := a.Task.LimiteSubtareas
	if limite < 1 {
		limite = task.DefaultLimiteSubtareas
	}
	requisitos := fmt.Sprintf(
		"Es una subdivisión de la tarea %s. SOLO podés declarar \"tocar_solo\" dentro de estas rutas ya asignadas a la tarea (ninguna otra): %s. Máximo %d subtareas.",
		a.Task.ID, strings.Join(a.Task.TocarSolo, ", "), limite,
	)
	ctxPlan := plan.Contexto{
		Lenguaje:     config.DetectLanguage(req.RoomPath),
		Pruebas:      a.Cfg.Pruebas,
		Constitucion: a.Constitucion,
		Requisitos:   requisitos,
		Agentes:      a.Cfg.TodosLosAgentes(),
	}
	borradores, err := plan.Generar(ctx, a.Planificador, ctxPlan, frase)
	if err != nil {
		return nil, err
	}
	restringirAlcance(borradores, a.Task.TocarSolo)
	for i, b := range borradores {
		if len(b.TocarSolo) == 0 {
			return nil, fmt.Errorf("subtarea %d (%q) quedó sin tocar_solo dentro del alcance de %s", i+1, b.Titulo, a.Task.ID)
		}
	}
	return borradores, nil
}

// restringirAlcance recorta de cada borrador los globs que se salen del
// alcance heredado del padre. ponytail: contención por prefijo de
// directorio, no intersección completa de globs — alcanza para el caso
// real (subrutas de tocar_solo) sin escribir un motor de globs nuevo.
func restringirAlcance(bs []plan.Borrador, permitidos []string) {
	if len(permitidos) == 0 {
		return // padre sin alcance declarado: nada que contener
	}
	for i := range bs {
		var dentro []string
		for _, g := range bs[i].TocarSolo {
			if dentroDeAlcance(g, permitidos) {
				dentro = append(dentro, g)
			}
		}
		bs[i].TocarSolo = dentro
	}
}

func dentroDeAlcance(glob string, permitidos []string) bool {
	for _, p := range permitidos {
		if glob == p || strings.HasPrefix(glob, prefijo(p)) {
			return true
		}
	}
	return false
}

// prefijo devuelve la parte literal de un glob antes del primer
// comodín — el directorio que de verdad delimita, "internal/foo/**" →
// "internal/foo/".
func prefijo(glob string) string {
	if i := strings.IndexAny(glob, "*?"); i >= 0 {
		return glob[:i]
	}
	return glob
}

// tareaDesdeBorrador arma el contrato completo de una subtarea a partir
// de lo que propuso el planificador. El id es determinístico y siempre
// válido (`^T-\d{3,}$`): concatena el número de la tarea padre con el
// índice, así que no colisiona ni necesita reservar nada en
// .devclean/tasks/ — estas subtareas son efímeras, nunca se guardan en
// disco.
func (a Agent) tareaDesdeBorrador(b plan.Borrador, indice int) (task.Task, error) {
	sub := task.Task{
		Version:        task.Version,
		ID:             fmt.Sprintf("%s%03d", a.Task.ID, indice),
		Titulo:         b.Titulo,
		Porque:         b.Porque,
		ListoCuando:    b.ListoCuando,
		TocarSolo:      b.TocarSolo,
		NoTocar:        b.NoTocar,
		Riesgos:        b.Riesgos,
		Peso:           b.Peso,
		Agente:         b.Agente,
		LimiteIntentos: task.DefaultLimiteIntentos,
		LimiteLineas:   task.DefaultLimiteLineas,
	}
	if errs := sub.Validate(); len(errs) > 0 {
		return task.Task{}, errs[0]
	}
	return sub, nil
}

// integrar mergea la rama de una subtarea verde a la rama actual del
// cuarto padre (no a una rama de integración aparte: acá no hay oleada
// paralela que encadenar, es una sola rama que va acumulando).
func integrar(roomPath, rama, id string) error {
	cmd := exec.Command("git", "-c", "user.name=devclean", "-c", "user.email=devclean@local",
		"merge", "--no-ff", "-m", "devclean: integra "+id, rama)
	cmd.Dir = roomPath
	out, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	mergeAbort := exec.Command("git", "merge", "--abort")
	mergeAbort.Dir = roomPath
	_, _ = mergeAbort.CombinedOutput()
	return fmt.Errorf("%s", tail(string(out)))
}

func tail(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > 3 {
		lines = lines[len(lines)-3:]
	}
	return strings.Join(lines, " · ")
}
