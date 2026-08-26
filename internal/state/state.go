// Package state tracks the runtime state of each task in
// .devclean/state/<id>.json. State files are machine-managed, so they
// are JSON; the contract stays in yaml for humans (§6.1).
package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Task states. Transitions: pendiente → en_curso → lista | detenida.
const (
	Pendiente = "pendiente"
	EnCurso   = "en_curso"
	Lista     = "lista"
	Detenida  = "detenida"
)

// State is the runtime record of one task.
type State struct {
	ID          string `json:"id"`
	Estado      string `json:"estado"`
	Intentos    int    `json:"intentos"`
	Rama        string `json:"rama,omitempty"`
	Puerto      int    `json:"puerto,omitempty"`
	UltimoError string `json:"ultimo_error,omitempty"`
	Pregunta    string `json:"pregunta,omitempty"`
}

// Dir returns the state directory of the repository at root.
func Dir(root string) string {
	return filepath.Join(root, ".devclean", "state")
}

func path(root, id string) string {
	return filepath.Join(Dir(root), id+".json")
}

// Get reads the state of a task. A task without file is pendiente.
func Get(root, id string) (State, error) {
	data, err := os.ReadFile(path(root, id))
	if errors.Is(err, os.ErrNotExist) {
		return State{ID: id, Estado: Pendiente}, nil
	}
	if err != nil {
		return State{}, err
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return State{}, fmt.Errorf("state/%s.json corrupto · %s · bórralo y reintenta", id, err)
	}
	return s, nil
}

// Save writes the state of a task.
func Save(root string, s State) error {
	if err := os.MkdirAll(Dir(root), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path(root, s.ID), append(data, '\n'), 0o644)
}

// List returns every state file, sorted by ID.
func List(root string) ([]State, error) {
	entries, err := os.ReadDir(Dir(root))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var states []State
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		s, err := Get(root, strings.TrimSuffix(e.Name(), ".json"))
		if err != nil {
			return nil, err
		}
		states = append(states, s)
	}
	sort.Slice(states, func(i, j int) bool { return states[i].ID < states[j].ID })
	return states, nil
}

// Remove deletes the state file of a task.
func Remove(root, id string) error {
	err := os.Remove(path(root, id))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
