// Package plan implementa al planificador (§5, §8.2): convierte una
// petición en lenguaje natural en contratos de tarea. El contrato lo
// redacta un modelo; devclean solo parsea, muestra y aprueba. Nunca lo
// escribe a mano el usuario (§6.1).
package plan

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Borrador es una tarea propuesta por el planificador, antes de asignarle
// id y version.
type Borrador struct {
	Titulo      string   `json:"titulo"`
	Porque      string   `json:"porque"`
	ListoCuando string   `json:"listo_cuando"`
	TocarSolo   []string `json:"tocar_solo"`
	NoTocar     []string `json:"no_tocar"`
	Riesgos     string   `json:"riesgos"`
}

// Generador pide texto a un modelo. El comando lo adapta desde el
// ejecutor; aquí solo importa la interfaz mínima.
type Generador interface {
	Generar(ctx context.Context, prompt string) (string, error)
}

// Prompt arma la instrucción para el planificador: una frase entra, un
// array JSON de contratos sale.
func Prompt(frase string) string {
	return `Eres el planificador de devclean. Parte esta petición en tareas independientes, pequeñas y verificables:

"` + frase + `"

Devuelve SOLO un array JSON, sin texto alrededor, con estos campos por tarea:
- "titulo": frase corta en minúscula
- "porque": por qué importa (una frase)
- "listo_cuando": un comando ejecutable que diga "ya está" (obligatorio)
- "tocar_solo": array de globs de archivos que la tarea puede tocar
- "riesgos": riesgos o limitaciones, o "" si no hay

Ejemplo:
[
  {"titulo": "exportar clientes a CSV", "porque": "soporte pierde horas copiando a mano", "listo_cuando": "npm test -- export", "tocar_solo": ["src/export/**"], "riesgos": ""}
]`
}

// Parse extrae la lista de borradores de la respuesta del modelo, tolerando
// vallas markdown y texto alrededor. Un borrador sin titulo ni listo_cuando
// se rechaza: un plan sin criterio de "listo" no es un plan.
func Parse(texto string) ([]Borrador, error) {
	t := strings.TrimSpace(texto)
	t = strings.TrimPrefix(t, "```json")
	t = strings.TrimPrefix(t, "```")
	t = strings.TrimSuffix(t, "```")
	t = strings.TrimSpace(t)

	ini := strings.Index(t, "[")
	fin := strings.LastIndex(t, "]")
	if ini == -1 || fin <= ini {
		return nil, errors.New("el modelo no devolvió un array JSON · vuelve a intentarlo")
	}

	var bs []Borrador
	if err := json.Unmarshal([]byte(t[ini:fin+1]), &bs); err != nil {
		return nil, fmt.Errorf("el modelo devolvió JSON inválido · %s", err)
	}
	if len(bs) == 0 {
		return nil, errors.New("el modelo no propuso ninguna tarea")
	}
	for i, b := range bs {
		if strings.TrimSpace(b.Titulo) == "" || strings.TrimSpace(b.ListoCuando) == "" {
			return nil, fmt.Errorf("la tarea %d del plan no trae titulo ni listo_cuando · el modelo se desvió", i+1)
		}
	}
	return bs, nil
}

// Generar pide el plan y lo parsea.
func Generar(ctx context.Context, g Generador, frase string) ([]Borrador, error) {
	texto, err := g.Generar(ctx, Prompt(frase))
	if err != nil {
		return nil, err
	}
	return Parse(texto)
}
