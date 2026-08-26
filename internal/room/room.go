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
)

// Room is an isolated worktree for one task.
type Room struct {
	ID     string `json:"id"`
	Path   string `json:"path"`
	Rama   string `json:"rama"`
	Puerto int    `json:"puerto"`
}

// Dir returns the rooms directory of the repository at root.
func Dir(root string) string {
	return filepath.Join(root, ".devclean", "rooms")
}

// Branch returns the branch name of a task's room.
func Branch(id string) string { return "devclean/" + id }

// Create sets up the room for a task: a worktree on a new branch from
// base, dependencies installed per manifest, and a free port assigned.
// On any failure the half-created room is destroyed.
func Create(ctx context.Context, root, id, base string) (Room, error) {
	r := Room{ID: id, Path: filepath.Join(Dir(root), id), Rama: Branch(id)}

	if _, err := os.Stat(r.Path); err == nil {
		return Room{}, fmt.Errorf("ya existe el cuarto %s · destrúyelo antes de reintentar", id)
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

// Destroy removes the worktree and its branch. Missing pieces are
// skipped: Destroy always leaves a clean state.
func Destroy(ctx context.Context, root, id string) error {
	path := filepath.Join(Dir(root), id)
	if _, err := os.Stat(path); err == nil {
		if out, err := git(ctx, root, "worktree", "remove", "--force", path); err != nil {
			return fmt.Errorf("no se pudo destruir el cuarto %s · %s", id, strings.TrimSpace(out))
		}
	}
	if _, err := git(ctx, root, "branch", "-D", Branch(id)); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			// la rama no existía: nada que borrar
			return nil
		}
		return err
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
