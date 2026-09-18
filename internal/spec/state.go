package spec

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const FeatureStateFile = "feature.json"

// SaveFeatureState conserva la intención humana separada del IR de tareas.
func SaveFeatureState(root string, s Spec) error {
	dir := filepath.Join(root, ".devclean")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, FeatureStateFile), append(b, '\n'), 0o644)
}
func LoadFeatureState(root string) (Spec, error) {
	b, err := os.ReadFile(filepath.Join(root, ".devclean", FeatureStateFile))
	if err != nil {
		return Spec{}, err
	}
	var s Spec
	err = json.Unmarshal(b, &s)
	return s, err
}
func (s Spec) AcceptanceCommands() []string {
	var out []string
	for _, a := range s.Acceptance {
		if a.Command != "" {
			out = append(out, a.Command)
		}
	}
	return out
}
