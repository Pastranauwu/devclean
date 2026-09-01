// Package room manages the isolated workrooms of §6.2: one git
// worktree per active task under .devclean/rooms/<id>/, on branch
// devclean/<id>, with its own dependencies and port. Rooms are
// destroyed when the task ends.
package room

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Pastranauwu/devclean/internal/state"
)

// Room is an isolated worktree for one task.
type Room struct {
	ID     string `json:"id"`
	Path   string `json:"path"`
	Rama   string `json:"rama"`
	Puerto int    `json:"puerto"`
	// Commit es el commit desde el que arrancó el cuarto, ya resuelto a
	// hash. Marca dónde empieza el trabajo propio de la tarea.
	Commit string `json:"commit,omitempty"`
}

// Dir returns the rooms directory of the repository at root.
func Dir(root string) string {
	return filepath.Join(root, ".devclean", "rooms")
}

// Branch returns the branch name of a task's room.
func Branch(id string) string { return "devclean/" + id }

// RamaExiste reporta si la rama de un cuarto sigue en el repo. El
// trabajo verde de una corrida anterior vive ahí hasta que `ship` lo
// entrega, así que una corrida nueva necesita saber si puede apoyarse
// en él.
func RamaExiste(ctx context.Context, root, id string) bool {
	_, err := git(ctx, root, "rev-parse", "--verify", "--quiet", Branch(id)+"^{commit}")
	return err == nil
}

// IntegrationBranch es la rama temporal donde se encadenan las oleadas:
// el trabajo verde de una oleada se mergea aquí y la siguiente oleada
// crea sus cuartos desde esta rama (Fase 2).
const IntegrationBranch = "devclean/_integra"

// ResetIntegration borra la rama y el worktree de integración previos y
// los recrea desde base. Corre al inicio de una corrida por oleadas.
func ResetIntegration(ctx context.Context, root, base string) error {
	_ = Destroy(ctx, root, "_integra")
	if base == "" {
		base = "HEAD"
	}
	if _, err := git(ctx, root, "rev-parse", "--verify", "--quiet", base+"^{commit}"); err != nil {
		return fmt.Errorf("no hay commits en %s · haz un commit inicial y reintenta", base)
	}
	path := filepath.Join(Dir(root), "_integra")
	if out, err := git(ctx, root, "worktree", "add", path, "-b", IntegrationBranch, base); err != nil {
		return fmt.Errorf("no se pudo crear la rama de integración · %s", strings.TrimSpace(out))
	}
	return nil
}

// Integrar mergea la rama de una tarea verde en la rama de integración.
// Devuelve la salida del conflicto (vacía si el merge fue limpio) y un
// error si hubo conflicto u otro fallo.
func Integrar(ctx context.Context, root, id string) (string, error) {
	path := filepath.Join(Dir(root), "_integra")
	if _, err := os.Stat(path); err != nil {
		return "", errors.New("no hay rama de integración · vuelve a correr devclean run")
	}
	msg := "devclean: integra " + id
	cmd := exec.CommandContext(ctx, "git", "-c", "user.name=devclean", "-c", "user.email=devclean@local", "merge", "--no-ff", "-m", msg, Branch(id))
	cmd.Dir = path
	out, err := cmd.CombinedOutput()
	if err == nil {
		return "", nil
	}
	_, _ = git(ctx, path, "merge", "--abort")
	return strings.TrimSpace(string(out)), err
}

// Create sets up the room for a task: a worktree on a new branch from
// base, dependencies installed per manifest, and a free port assigned.
// On any failure the half-created room is destroyed.
func Create(ctx context.Context, root, id, base string) (Room, error) {
	r := Room{ID: id, Path: filepath.Join(Dir(root), id), Rama: Branch(id)}

	if _, err := os.Stat(r.Path); err == nil {
		return Room{}, fmt.Errorf("ya existe el cuarto %s · destrúyelo antes de reintentar", id)
	}
	// la carpeta del cuarto no está: limpia cualquier worktree o rama
	// que dejó una corrida anterior con rooms/ borrado, para que el
	// `worktree add` de abajo no choque. Create solo se llama sobre
	// tareas pendientes, así que una rama devclean/<id> aquí es basura.
	if err := Destroy(ctx, root, id); err != nil {
		return Room{}, err
	}
	if base == "" {
		base = "HEAD"
	}
	// verificar la base antes: el mensaje de git varía con el idioma
	if _, err := git(ctx, root, "rev-parse", "--verify", "--quiet", base+"^{commit}"); err != nil {
		return Room{}, fmt.Errorf("no hay commits en %s · haz un commit inicial y reintenta", base)
	}
	if out, err := git(ctx, root, "worktree", "add", r.Path, "-b", r.Rama, base); err != nil {
		return Room{}, fmt.Errorf("no se pudo crear el cuarto %s · %s", id, strings.TrimSpace(out))
	}
	if hash, err := git(ctx, root, "rev-parse", base+"^{commit}"); err == nil {
		r.Commit = strings.TrimSpace(hash)
	}

	fail := func(err error) (Room, error) {
		_ = Destroy(context.Background(), root, id)
		return Room{}, err
	}

	if err := installDeps(ctx, r.Path); err != nil {
		return fail(err)
	}
	puerto, err := freePort()
	if err != nil {
		return fail(err)
	}
	r.Puerto = puerto
	return r, nil
}

// Ensure devuelve el cuarto de una tarea, reusándolo si ya existe. Es lo
// que permite reintentar una tarea detenida sin tirar el trabajo parcial
// que quedó en su rama: el agente arranca viendo lo que ya escribió, en
// vez de rehacerlo desde cero y volver a pagar los mismos tokens.
func Ensure(ctx context.Context, root, id, base string) (Room, error) {
	r := Room{ID: id, Path: filepath.Join(Dir(root), id), Rama: Branch(id)}
	if _, err := os.Stat(r.Path); err != nil {
		return Create(ctx, root, id, base)
	}
	// la carpeta está, pero solo sirve si git la reconoce como worktree
	if _, err := git(ctx, r.Path, "rev-parse", "--git-dir"); err != nil {
		if err := Destroy(ctx, root, id); err != nil {
			return Room{}, err
		}
		return Create(ctx, root, id, base)
	}
	// el cuarto se reusa, así que su punto de partida es el que ya
	// tenía: recalcularlo desde `base` contaría como propio el trabajo
	// que hereda de una oleada anterior
	if st, err := state.Get(root, id); err == nil && st.Commit != "" {
		r.Commit = st.Commit
	}
	puerto, err := freePort()
	if err != nil {
		return Room{}, err
	}
	r.Puerto = puerto
	return r, nil
}

// Destroy removes the worktree and its branch. Missing pieces are
// skipped: Destroy always leaves a clean state, even when the room
// directory was wiped out of band. rooms/ is gitignored, so a `git
// clean -fdx` o un rm a mano dejan la registración del worktree y su
// rama huérfanas; sin limpiarlas, el próximo `worktree add` choca con
// "fatal: una rama llamada 'devclean/<id>' ya existe".
func Destroy(ctx context.Context, root, id string) error {
	path := filepath.Join(Dir(root), id)
	// quita el worktree si su carpeta sigue ahí, luego poda las
	// registraciones que quedaron cuando la carpeta ya no está.
	_, _ = git(ctx, root, "worktree", "remove", "--force", path)
	_, _ = git(ctx, root, "worktree", "prune")
	// rev-parse no depende del idioma de git; `branch -D` sí.
	if _, err := git(ctx, root, "rev-parse", "--verify", "--quiet", "refs/heads/"+Branch(id)); err != nil {
		return nil
	}
	if out, err := git(ctx, root, "branch", "-D", Branch(id)); err != nil {
		return fmt.Errorf("no se pudo borrar la rama %s · %s", Branch(id), strings.TrimSpace(out))
	}
	return nil
}

// freePort returns a port that was free a moment ago. §6.2: ports are
// assigned, never fixed.
func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// installDeps installs dependencies per manifest: npm install for
// package.json, go mod download for go.mod.
func installDeps(ctx context.Context, path string) error {
	if exists(filepath.Join(path, "package.json")) {
		if out, err := run(ctx, path, "npm", "install"); err != nil {
			return fmt.Errorf("npm install falló en el cuarto · %s", tail(out))
		}
	}
	if exists(filepath.Join(path, "go.mod")) {
		if out, err := run(ctx, path, "go", "mod", "download"); err != nil {
			return fmt.Errorf("go mod download falló en el cuarto · %s", tail(out))
		}
	}
	return nil
}

func exists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func git(ctx context.Context, dir string, args ...string) (string, error) {
	return run(ctx, dir, "git", args...)
}

func run(ctx context.Context, dir, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// tail keeps the last lines of a command output for error messages.
func tail(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > 3 {
		lines = lines[len(lines)-3:]
	}
	return strings.Join(lines, " · ")
}
