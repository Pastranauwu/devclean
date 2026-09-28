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

// IntencionFile guarda lo que el humano declaró la última vez que se
// planeó el spec. Sin esto, cada `up` sobre un spec de requirements
// volvía a pagar al arquitecto y duplicaba las tareas; con esto, un spec
// igual no se replanea y uno editado solo manda lo que cambió.
const IntencionFile = "intencion.json"

// Intencion es la parte del spec que decide qué construir. Agentes, ship
// y límites no cambian el plan.
type Intencion struct {
	Feature      string       `json:"feature"`
	Requirements []string     `json:"requirements,omitempty"`
	Reglas       []string     `json:"reglas,omitempty"`
	Acceptance   []Acceptance `json:"acceptance,omitempty"`
	Constraints  Constraints  `json:"constraints,omitempty"`
}

func IntencionDe(s Spec) Intencion {
	return Intencion{Feature: s.Feature, Requirements: s.Requirements, Reglas: s.Reglas, Acceptance: s.Acceptance, Constraints: s.Constraints}
}

// Igual compara dos intenciones por contenido.
func (i Intencion) Igual(o Intencion) bool {
	a, _ := json.Marshal(i)
	b, _ := json.Marshal(o)
	return string(a) == string(b)
}

func SaveIntencion(root string, i Intencion) error {
	b, err := json.MarshalIndent(i, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(root, ".devclean", IntencionFile), append(b, '\n'), 0o644)
}

// LoadIntencion devuelve la última intención planeada; false si nunca se
// planeó un spec en este repo.
func LoadIntencion(root string) (Intencion, bool) {
	b, err := os.ReadFile(filepath.Join(root, ".devclean", IntencionFile))
	if err != nil {
		return Intencion{}, false
	}
	var i Intencion
	if json.Unmarshal(b, &i) != nil {
		return Intencion{}, false
	}
	return i, true
}
