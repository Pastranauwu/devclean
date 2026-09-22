package spec

import (
	"fmt"
	"strings"

	"github.com/Pastranauwu/devclean/internal/task"
	"gopkg.in/yaml.v3"
)

// Parse usa YAML 1.2 completo. Los Node personalizados mantienen la forma
// corta histórica de tasks ("- hacer algo") y permiten agrupar requirements
// y acceptance por categorías sin cambiar el modelo interno.
func Parse(data []byte) (Spec, error) {
	var root yaml.Node
	dec := yaml.NewDecoder(strings.NewReader(string(data)))
	if err := dec.Decode(&root); err != nil {
		return Spec{}, err
	}
	if len(root.Content) == 0 {
		return Spec{Version: 1}, nil
	}
	n := root.Content[0]
	if n.Kind != yaml.MappingNode {
		return Spec{}, fmt.Errorf("la especificación debe ser un mapa YAML")
	}
	s := Spec{Version: 1}
	for i := 0; i < len(n.Content); i += 2 {
		key, val := n.Content[i].Value, n.Content[i+1]
		var err error
		switch key {
		case "version":
			err = val.Decode(&s.Version)
		case "feature", "titulo":
			err = val.Decode(&s.Feature)
		case "agente":
			err = val.Decode(&s.Agente)
		case "agentes":
			err = val.Decode(&s.Agentes)
			if err == nil && s.Agentes < 1 {
				err = fmt.Errorf("agentes inválido: %d · mínimo 1", s.Agentes)
			}
		case "ship":
			err = val.Decode(&s.Ship)
		case "reglas", "rules":
			s.Reglas, err = flattenStrings(val)
		case "requirements", "requisitos":
			s.Requirements, err = flattenStrings(val)
		case "acceptance", "aceptacion":
			s.Acceptance, err = decodeAcceptance(val)
		case "constraints", "restricciones":
			err = val.Decode(&s.Constraints)
		case "limites":
			err = val.Decode(&s.Limites)
		case "limite_intentos":
			err = val.Decode(&s.Limites.Intentos)
		case "limite_lineas":
			err = val.Decode(&s.Limites.Lineas)
		case "tasks", "tareas":
			s.Tasks, err = decodeTasks(val)
		case "architecture", "delivery": // reservados para evolución; YAML anidado ya se conserva sintácticamente
		default:
			err = fmt.Errorf("campo desconocido en especificación: %s", key)
		}
		if err != nil {
			return s, fmt.Errorf("%s: %w", key, err)
		}
	}
	for i := range s.Tasks {
		t := &s.Tasks[i]
		if t.Version == 0 {
			t.Version = task.Version
		}
		if t.Agente == "" {
			t.Agente = s.Agente
		}
		if t.LimiteIntentos == 0 {
			t.LimiteIntentos = task.DefaultLimiteIntentos
			if s.Limites.Intentos > 0 {
				t.LimiteIntentos = s.Limites.Intentos
			}
		}
		if t.LimiteLineas == 0 {
			t.LimiteLineas = task.DefaultLimiteLineas
			if s.Limites.Lineas > 0 {
				t.LimiteLineas = s.Limites.Lineas
			}
		}
	}
	return s, nil
}

// flattenStrings aplana requirements y reglas: lista simple, categorías
// anidadas, o una mezcla.
//
// enSecuencia distingue los dos usos de un mapa. Bajo la clave, un mapa
// son categorías (`functional:`) y el nombre de la categoría se descarta.
// Dentro de una LISTA, en cambio, `- texto: más texto` no es una
// categoría: es una frase con dos puntos, y YAML la lee como un mapa de
// un solo entry. Descartar ahí la clave se comía la mitad del requisito
// sin avisar —"la lógica no lee el teclado: eso vive en la terminal"
// llegaba al planificador como "eso vive en la terminal"—, así que se
// vuelve a unir.
func flattenStrings(n *yaml.Node) ([]string, error) {
	var out []string
	var walk func(*yaml.Node, bool) error
	walk = func(x *yaml.Node, enSecuencia bool) error {
		switch x.Kind {
		case yaml.ScalarNode:
			out = append(out, x.Value)
		case yaml.SequenceNode:
			for _, c := range x.Content {
				if err := walk(c, true); err != nil {
					return err
				}
			}
		case yaml.MappingNode:
			for i := 1; i < len(x.Content); i += 2 {
				clave, valor := x.Content[i-1], x.Content[i]
				if enSecuencia && valor.Kind == yaml.ScalarNode {
					texto := clave.Value
					if valor.Value != "" {
						texto += ": " + valor.Value
					}
					out = append(out, texto)
					continue
				}
				if err := walk(valor, false); err != nil {
					return err
				}
			}
		default:
			return fmt.Errorf("se esperaba texto, lista o categorías")
		}
		return nil
	}
	return out, walk(n, false)
}

func decodeAcceptance(n *yaml.Node) ([]Acceptance, error) {
	if n.Kind == yaml.MappingNode {
		var out []Acceptance
		for i := 1; i < len(n.Content); i += 2 {
			a, err := decodeAcceptance(n.Content[i])
			if err != nil {
				return nil, err
			}
			out = append(out, a...)
		}
		return out, nil
	}
	if n.Kind != yaml.SequenceNode {
		return []Acceptance{{Criterion: n.Value}}, nil
	}
	var out []Acceptance
	for _, item := range n.Content {
		if item.Kind == yaml.ScalarNode {
			out = append(out, Acceptance{Criterion: item.Value})
			continue
		}
		var raw struct {
			Criterion string `yaml:"criterion"`
			Criterio  string `yaml:"criterio"`
			Command   string `yaml:"command"`
			Comando   string `yaml:"comando"`
		}
		if err := item.Decode(&raw); err != nil {
			return nil, err
		}
		if raw.Criterion == "" {
			raw.Criterion = raw.Criterio
		}
		if raw.Command == "" {
			raw.Command = raw.Comando
		}
		if raw.Criterion == "" {
			return nil, fmt.Errorf("criterio de aceptación vacío")
		}
		out = append(out, Acceptance{Criterion: raw.Criterion, Command: raw.Command})
	}
	return out, nil
}

func decodeTasks(n *yaml.Node) ([]task.Task, error) {
	if n.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("tasks debe ser una lista")
	}
	var out []task.Task
	for _, item := range n.Content {
		if item.Kind == yaml.ScalarNode {
			out = append(out, task.Task{Version: task.Version, Titulo: item.Value})
			continue
		}
		var raw struct {
			ID       string   `yaml:"id"`
			Titulo   string   `yaml:"titulo"`
			Porque   string   `yaml:"porque"`
			Listo    string   `yaml:"listo_cuando"`
			Tocar    []string `yaml:"tocar_solo"`
			NoTocar  []string `yaml:"no_tocar"`
			Depende  []string `yaml:"depende_de"`
			Expone   []string `yaml:"expone"`
			Usa      []string `yaml:"usa"`
			Riesgos  string   `yaml:"riesgos"`
			Peso     string   `yaml:"peso"`
			Agente   string   `yaml:"agente"`
			Intentos int      `yaml:"limite_intentos"`
			Lineas   int      `yaml:"limite_lineas"`
			Notas    string   `yaml:"notas"`
		}
		if err := item.Decode(&raw); err != nil {
			return nil, err
		}
		out = append(out, task.Task{Version: task.Version, ID: raw.ID, Titulo: raw.Titulo, Porque: raw.Porque, ListoCuando: raw.Listo, TocarSolo: raw.Tocar, NoTocar: raw.NoTocar, DependeDe: raw.Depende, Expone: raw.Expone, Usa: raw.Usa, Riesgos: raw.Riesgos, Peso: raw.Peso, Agente: raw.Agente, LimiteIntentos: raw.Intentos, LimiteLineas: raw.Lineas, Notas: raw.Notas})
	}
	return out, nil
}
