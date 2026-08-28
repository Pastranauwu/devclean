// Package sealed manages the hidden test suite storage for §6.8.
// The sealed dir lives in the main repo (.devclean/sealed/<id>/), NOT
// in the worktree — the worktree is the implementer's domain.
package sealed

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const fileName = "suite_oculta.json"

// SuiteOculta is what gets stored for a task.
type SuiteOculta struct {
	Hash    string `json:"hash"`    // sha256 of Content for integrity check
	Content string `json:"content"` // the hidden test file content
	Archivo string `json:"archivo"` // relative path within the room to write this file
}

// Dir returns .devclean/sealed/<id>/ in root.
func Dir(root, id string) string {
	return filepath.Join(root, ".devclean", "sealed", id)
}

// Write seals the hidden suite for a task. Overwrites any prior sealed suite.
func Write(root, id string, s SuiteOculta) error {
	s.Hash = fmt.Sprintf("%x", sha256.Sum256([]byte(s.Content)))
	if err := os.MkdirAll(Dir(root, id), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(Dir(root, id), fileName), append(data, '\n'), 0o644)
}

// Read loads the sealed suite. Returns os.ErrNotExist if never sealed.
// Verifies the hash to detect accidental corruption.
func Read(root, id string) (SuiteOculta, error) {
	data, err := os.ReadFile(filepath.Join(Dir(root, id), fileName))
	if errors.Is(err, os.ErrNotExist) {
		return SuiteOculta{}, os.ErrNotExist
	}
	if err != nil {
		return SuiteOculta{}, err
	}
	var s SuiteOculta
	if err := json.Unmarshal(data, &s); err != nil {
		return SuiteOculta{}, fmt.Errorf("suite sellada corrupta · %s", err)
	}
	got := fmt.Sprintf("%x", sha256.Sum256([]byte(s.Content)))
	if got != s.Hash {
		return SuiteOculta{}, fmt.Errorf("suite sellada corrupta · hash no coincide")
	}
	return s, nil
}

// Burn removes the sealed directory. Called by ship after the hidden run.
func Burn(root, id string) error {
	return os.RemoveAll(Dir(root, id))
}

// Exists reports whether a sealed suite exists for id.
func Exists(root, id string) bool {
	_, err := os.Stat(filepath.Join(Dir(root, id), fileName))
	return err == nil
}
