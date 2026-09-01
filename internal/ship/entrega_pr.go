package ship

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Pastranauwu/devclean/internal/config"
	"github.com/Pastranauwu/devclean/internal/room"
	"github.com/Pastranauwu/devclean/internal/task"
)

// RamaEntrega es la rama donde se juntan los commits de todas las tareas
// verdes antes de abrir un solo PR.
const RamaEntrega = "devclean/_entrega"

// OpcionesEntrega lleva las dependencias de la entrega conjunta.
type OpcionesEntrega struct {
	Root    string
	Config  config.Config
	Base    string
	Tareas  []task.Task       // todas en estado lista
	Modelos map[string]string // id de tarea -> modelo del último intento
	// Commits lleva, por tarea, el commit desde el que se creó su cuarto.
	// Es lo que separa su trabajo del que heredó de una oleada anterior,
	// y a diferencia de los mensajes `wip:` sobrevive a que la esclusa
	// aplane la rama: sin él, una segunda pasada medía el cuarto entero.
	Commits  map[string]string
	Titulo   string // título del PR; vacío usa el de la primera tarea
	Timeout  time.Duration
	DryRun   bool
	Progreso func(Paso)
}

// Entrega es el desenlace de entregar varias tareas en un solo PR.
type Entrega struct {
	Tareas   []Resultado `json:"tareas"`
	Pasos    []Paso      `json:"pasos"`
	Aprobado bool        `json:"aprobado"`
	PR       string      `json:"pr,omitempty"`
	Rama     string      `json:"rama,omitempty"`
}

// PrimerMotivo devuelve el motivo del primer paso o tarea que frenó.
func (e Entrega) PrimerMotivo() string {
	for _, r := range e.Tareas {
		if !r.Aprobado {
			return r.ID + " · " + r.PrimerMotivo()
		}
	}
	for _, p := range e.Pasos {
		if !p.OK {
			return p.Detalle
		}
	}
	return ""
}

// EntregarTodas pasa cada tarea por su esclusa de salida y, si todas la
// superan, junta sus commits en una sola rama lineal y abre UN PR.
//
// Es la diferencia entre N pull requests que se pisan entre sí y uno
// limpio: cada tarea aporta exactamente un commit (la esclusa individual
// ya aplanó sus wip), en orden de dependencia, sin merges de por medio.
// La suite del proyecto corre una vez más sobre el conjunto ya integrado,
// que es lo único capaz de detectar que dos tareas verdes por separado se
// rompen juntas.
func EntregarTodas(ctx context.Context, o OpcionesEntrega) Entrega {
	var e Entrega
	if o.Base == "" {
		o.Base = "HEAD"
	}
	if o.Timeout <= 0 {
		o.Timeout = DefaultTimeout
	}
	apuntar := func(p Paso) {
		e.Pasos = append(e.Pasos, p)
		if o.Progreso != nil {
			o.Progreso(p)
		}
	}

	ordenadas, err := ordenTopologico(o.Tareas)
	if err != nil {
		apuntar(Paso{"orden", false, err.Error()})
		return e
	}

	// 1. la esclusa de salida completa de cada tarea, en seco: ninguna
	//    entra al PR si no pasa sola su propio control de calidad.
	for _, t := range ordenadas {
		roomPath := roomPathDe(o.Root, t.ID)
		// cada tarea se aplana contra su propio punto de partida, no
		// contra la rama base: así su commit lleva solo lo suyo
		baseTarea := o.Commits[t.ID]
		if baseTarea == "" {
			baseTarea = baseDelta(roomPath, t.ID, o.Base) // cuartos anteriores a que se anotara
		}
		if baseTarea == "" {
			baseTarea = o.Base
		}
		r := Run(ctx, Opciones{
			Root:    o.Root,
			Room:    room.Room{ID: t.ID, Path: roomPath, Rama: room.Branch(t.ID)},
			Task:    t,
			Config:  o.Config,
			Modelo:  o.Modelos[t.ID],
			Base:    baseTarea,
			Timeout: o.Timeout,
			DryRun:  true, // el PR lo abre la entrega, no cada tarea
		})
		e.Tareas = append(e.Tareas, r)
		if !r.Aprobado {
			apuntar(Paso{"esclusa " + t.ID, false, r.PrimerMotivo()})
			return e
		}
		apuntar(Paso{"esclusa " + t.ID, true, "lista para integrar"})
	}

	// 2. una rama de entrega limpia desde la base
	target := o.Base
	if _, err := gitRun(o.Root, "rev-parse", "--verify", "--quiet", "origin/"+o.Base); err == nil {
		target = "origin/" + o.Base
	}
	path := roomPathDe(o.Root, "_entrega")
	if err := limpiarEntrega(o.Root, path); err != nil {
		apuntar(Paso{"rama de entrega", false, err.Error()})
		return e
	}
	if out, err := gitRun(o.Root, "worktree", "add", path, "-b", RamaEntrega, target); err != nil {
		apuntar(Paso{"rama de entrega", false, tail(out)})
		return e
	}
	defer func() {
		if !e.Aprobado || o.DryRun {
			return
		}
		_ = limpiarEntrega(o.Root, path)
	}()
	e.Rama = RamaEntrega
	apuntar(Paso{"rama de entrega", true, RamaEntrega + " desde " + target})

	// 3. un commit por tarea, en orden de dependencia
	for _, t := range ordenadas {
		// cherry-pick crea un commit, así que necesita identidad: sin
		// ella git muere con "empty ident name" en cualquier maquina que
		// no tenga user.name configurado (un runner de CI, por ejemplo)
		args := append(identity(path), "cherry-pick", room.Branch(t.ID))
		if out, err := gitRun(path, args...); err != nil {
			// un cherry-pick falla por conflicto, pero tambien por
			// cosas que no tienen nada que ver con el codigo. Culpar
			// siempre al solapamiento manda al usuario a revisar unos
			// tocar_solo que estan bien.
			conflictos := unmerged(path)
			_, _ = gitRun(path, "cherry-pick", "--abort")
			detalle := "no se pudo integrar · " + tail(out)
			if len(conflictos) > 0 {
				detalle = "choca con el trabajo de una tarea anterior en " + unir(conflictos) +
					" · sus alcances (tocar_solo) se solapan"
			}
			apuntar(Paso{"integrar " + t.ID, false, detalle})
			return e
		}
	}
	apuntar(Paso{"integrar", true, fmt.Sprintf("%d commits, uno por tarea", len(ordenadas))})

	// 4. la suite completa sobre el conjunto ya integrado
	pruebas := strings.TrimSpace(o.Config.Pruebas)
	if pruebas == "" {
		apuntar(Paso{"integradas", true, "sin comando de pruebas en config.yml · no se verificó el conjunto"})
	} else if salida, ok := correrPruebas(ctx, path, pruebas, o.Timeout); !ok {
		apuntar(Paso{"integradas", false,
			"las tareas pasan por separado pero el conjunto falla · " + salida})
		return e
	} else {
		apuntar(Paso{"integradas", true, pruebas})
	}

	// 5. un solo PR
	cuerpo := cuerpoEntrega(ordenadas, e.Tareas)
	titulo := o.Titulo
	if titulo == "" && len(ordenadas) > 0 {
		titulo = ordenadas[0].Titulo
	}
	if o.DryRun {
		apuntar(Paso{"pr", true, "dry-run · sin PR · rama " + RamaEntrega})
		e.Aprobado = true
		return e
	}
	url, err := abrirPREntrega(ctx, o.Root, path, o.Base, titulo, cuerpo)
	if err != nil {
		apuntar(Paso{"pr", false, err.Error()})
		return e
	}
	apuntar(Paso{"pr", true, url})
	e.PR, e.Aprobado = url, true

	// los cuartos ya entregaron: liberarlos deja el repo limpio
	for _, t := range ordenadas {
		_ = room.Destroy(ctx, o.Root, t.ID)
	}
	_ = room.Destroy(ctx, o.Root, "_integra")
	return e
}

// baseDelta devuelve el commit desde el que empieza el trabajo propio de
// una tarea: el padre del primer commit que escribió devclean en ese
// cuarto (la suite del examinador, o el primer `wip:`).
//
// Hace falta porque un cuarto de la segunda oleada nace desde la rama de
// integración y por tanto YA contiene el trabajo de la oleada anterior.
// Aplanarlo contra la rama base metería ese trabajo ajeno dentro de su
// propio commit, y al juntar los commits en la rama de entrega el segundo
// volvería a añadir los archivos del primero: conflicto garantizado.
// Cadena vacía si no se puede determinar; el llamador cae en la base.
func baseDelta(roomPath, id, base string) string {
	out, err := gitRun(roomPath, "log", "--reverse", "--format=%H%x00%s", "HEAD")
	if err != nil {
		return ""
	}
	prefijoWip := "wip: " + id + " "
	for _, linea := range strings.Split(out, "\n") {
		hash, subject, ok := strings.Cut(strings.TrimSpace(linea), "\x00")
		if !ok {
			continue
		}
		if !strings.HasPrefix(subject, "exam:") && !strings.HasPrefix(subject, prefijoWip) {
			continue
		}
		padre, err := gitRun(roomPath, "rev-parse", "--verify", "--quiet", hash+"^")
		if err != nil {
			return "" // el trabajo arranca en el commit raíz
		}
		return strings.TrimSpace(padre)
	}

	// Sin marcadores, la esclusa ya paso por aqui y aplano la rama en un
	// solo commit: el trabajo propio de la tarea es justo ese commit. Sin
	// este caso, una segunda pasada media el cuarto entero contra la rama
	// base y frenaba por un presupuesto que nadie habia excedido.
	if unSoloCommit(roomPath, base) {
		if padre, err := gitRun(roomPath, "rev-parse", "--verify", "--quiet", "HEAD^"); err == nil {
			return strings.TrimSpace(padre)
		}
	}
	return ""
}

// unSoloCommit reporta si la rama tiene exactamente un commit por encima
// de base, que es la forma que deja `aplanar`.
func unSoloCommit(roomPath, base string) bool {
	out, err := gitRun(roomPath, "rev-list", "--count", base+"..HEAD")
	if err != nil {
		return false
	}
	return strings.TrimSpace(out) == "1"
}

// roomPathDe devuelve la ruta del cuarto de un id.
func roomPathDe(root, id string) string { return filepath.Join(room.Dir(root), id) }

// limpiarEntrega borra el worktree y la rama de entrega de una corrida
// anterior, para que `worktree add` no choque.
func limpiarEntrega(root, path string) error {
	_, _ = gitRun(root, "worktree", "remove", "--force", path)
	_, _ = gitRun(root, "worktree", "prune")
	if _, err := gitRun(root, "rev-parse", "--verify", "--quiet", "refs/heads/"+RamaEntrega); err != nil {
		return nil
	}
	if out, err := gitRun(root, "branch", "-D", RamaEntrega); err != nil {
		return fmt.Errorf("no se pudo borrar la rama de entrega · %s", tail(out))
	}
	return nil
}

// correrPruebas corre el comando de pruebas del proyecto en dir.
func correrPruebas(ctx context.Context, dir, cmdStr string, timeout time.Duration) (string, bool) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "sh", "-c", cmdStr)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Sprintf("las pruebas tardaron más de %s", timeout), false
	}
	if err != nil {
		return tail(string(out)), false
	}
	return "", true
}

// ordenTopologico ordena las tareas para que ninguna llegue antes que
// aquella de la que depende. Un ciclo se reporta en vez de colgarse.
func ordenTopologico(tareas []task.Task) ([]task.Task, error) {
	porID := make(map[string]task.Task, len(tareas))
	for _, t := range tareas {
		porID[t.ID] = t
	}
	ids := make([]string, 0, len(tareas))
	for _, t := range tareas {
		ids = append(ids, t.ID)
	}
	sort.Strings(ids) // orden estable entre tareas sin dependencia mutua

	listo := map[string]bool{}
	var out []task.Task
	for len(out) < len(tareas) {
		avanzo := false
		for _, id := range ids {
			if listo[id] {
				continue
			}
			puede := true
			for _, d := range porID[id].DependeDe {
				// una dependencia fuera del lote ya está en la base
				if _, enLote := porID[d]; enLote && !listo[d] {
					puede = false
					break
				}
			}
			if !puede {
				continue
			}
			listo[id] = true
			out = append(out, porID[id])
			avanzo = true
		}
		if !avanzo {
			var faltan []string
			for _, id := range ids {
				if !listo[id] {
					faltan = append(faltan, id)
				}
			}
			return nil, fmt.Errorf("ciclo de dependencias entre %s · revisa depende_de", strings.Join(faltan, ", "))
		}
	}
	return out, nil
}

// cuerpoEntrega arma el handoff del PR conjunto: una sección por tarea,
// con lo mismo que llevaría su PR individual.
func cuerpoEntrega(tareas []task.Task, resultados []Resultado) string {
	porID := make(map[string]Resultado, len(resultados))
	for _, r := range resultados {
		porID[r.ID] = r
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%d tareas entregadas, un commit cada una.\n\n", len(tareas))
	for _, t := range tareas {
		r := porID[t.ID]
		fmt.Fprintf(&b, "---\n\n### %s · %s\n\n", t.ID, t.Titulo)
		if t.Porque != "" {
			fmt.Fprintf(&b, "%s\n\n", t.Porque)
		}
		fmt.Fprintf(&b, "%d líneas añadidas, %d quitadas.\n\n", r.LineasMas, r.LineasMenos)
		if t.Riesgos != "" {
			fmt.Fprintf(&b, "Riesgos: %s\n\n", t.Riesgos)
		}
		fmt.Fprintf(&b, "Verificar:\n```\n%s\n```\n\n", t.ListoCuando)
	}
	return b.String()
}

// abrirPREntrega sube la rama de entrega y abre el PR único.
func abrirPREntrega(ctx context.Context, root, path, base, titulo, cuerpo string) (string, error) {
	gh, err := exec.LookPath("gh")
	if err != nil {
		return "", fmt.Errorf("gh no está instalado · instálalo o usa --dry-run")
	}
	if _, err := gitRun(root, "remote", "get-url", "origin"); err != nil {
		return "", fmt.Errorf("sin remoto origin · agrégalo con git remote add origin <url> o usa --dry-run")
	}
	if out, err := gitRun(path, "push", "--force", "-u", "origin", RamaEntrega); err != nil {
		return "", fmt.Errorf("no se pudo subir la rama de entrega · %s", tail(out))
	}
	if url := prExistente(ctx, root, gh, RamaEntrega); url != "" {
		return url, nil
	}

	f, err := os.CreateTemp("", "devclean-entrega-*.md")
	if err != nil {
		return "", err
	}
	defer os.Remove(f.Name())
	if _, err := f.WriteString(cuerpo); err != nil {
		f.Close()
		return "", err
	}
	f.Close()

	cmd := exec.CommandContext(ctx, gh, "pr", "create", "--base", base, "--head", RamaEntrega, "--title", titulo, "--body-file", f.Name())
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		if url := prExistente(ctx, root, gh, RamaEntrega); url != "" {
			return url, nil
		}
		return "", fmt.Errorf("gh no pudo abrir el PR · %s", tail(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}
