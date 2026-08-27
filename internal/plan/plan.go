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
	DependeDe   []string `json:"depende_de"`
	Riesgos     string   `json:"riesgos"`
	Peso        string   `json:"peso"`
}

// Generador pide texto a un modelo. El comando lo adapta desde el
// ejecutor; aquí solo importa la interfaz mínima.
type Generador interface {
	Generar(ctx context.Context, prompt string) (string, error)
}

// Contexto es lo que devclean sabe del repositorio al momento de
// planear. Se lo pasa al modelo para que no adivine el stack ni
// invente comandos que no existen en el proyecto (§8.2).
type Contexto struct {
	Lenguaje   string // go, node, python, rust, "" si no se detecta
	EsVacio    bool   // repo sin código fuente todavía
	Pruebas    string // comando de pruebas detectado ("" si no hay)
	Stack      string // stack elegido por el humano ("" si lo decide el modelo)
	Requisitos string // requisitos extra que dijo el humano, en texto libre
}

// Prompt arma la instrucción para el planificador: una frase entra, un
// array JSON de contratos sale. El contexto del repo se inyecta para
// que el modelo use el lenguaje y el comando de pruebas reales.
func Prompt(frase string, c Contexto) string {
	var b strings.Builder
	b.WriteString("Eres el planificador de devclean. Parte esta petición en tareas independientes, pequeñas y verificables:\n\n")
	b.WriteString("\"" + frase + "\"\n\n")
	b.WriteString(contextoPrompt(c))
	b.WriteString("\n\nDevuelve SOLO un array JSON, sin texto alrededor, con estos campos por tarea:\n")
	b.WriteString("- \"titulo\": frase corta en minúscula\n")
	b.WriteString("- \"porque\": por qué importa (una frase)\n")
	b.WriteString("- \"listo_cuando\": un comando ejecutable que diga \"ya está\" (obligatorio)\n")
	b.WriteString("- \"tocar_solo\": array de globs de archivos que la tarea puede tocar\n")
	b.WriteString("- \"depende_de\": array de ids (ej. \"T-001\") de tareas que deben estar verdes antes que esta; vacío si no depende de ninguna\n")
	b.WriteString("- \"peso\": \"liviana\", \"media\" o \"pesada\" según la complejidad de la tarea (por defecto \"media\")\n")
	b.WriteString("- \"riesgos\": riesgos o limitaciones, o \"\" si no hay\n\n")
	b.WriteString("Ejemplo:\n[\n")
	b.WriteString("  {\"titulo\": \"exportar clientes a CSV\", \"porque\": \"soporte pierde horas copiando a mano\", \"listo_cuando\": \"npm test -- export\", \"tocar_solo\": [\"src/export/**\"], \"riesgos\": \"\"}\n")
	b.WriteString("]")
	return b.String()
}

// contextoPrompt redacta la sección de contexto según lo detectado en
// el repo. En un repo vacío activa el modo greenfield: la primera
// tarea inicializa el stack y fija su comando de verificación real.
func contextoPrompt(c Contexto) string {
	var b strings.Builder
	if c.EsVacio {
		b.WriteString("El repositorio está vacío (sin código todavía). Debes arrancar desde cero:\n")
		if c.Stack != "" {
			fmt.Fprintf(&b, "- El humano eligió el stack: %s. Úsalo para todas las tareas.\n", c.Stack)
		} else {
			b.WriteString("- La PRIMERA tarea elige e inicializa UN stack (Go, Node, Python, ...) y deja el proyecto compilando.\n")
		}
		b.WriteString("- La primera tarea deja el proyecto compilando; su \"listo_cuando\" es el comando de build o test real del stack (ej. \"go build ./...\" si Go, \"npm test\" si Node).\n")
		b.WriteString("- Las demás tareas construyen sobre esa base, con \"listo_cuando\" reales de ese mismo stack que hoy fallen, y marcan en \"depende_de\" la tarea que creó la base (y cualquier otra de la que dependan).\n")
		b.WriteString("- Mantén UN solo lenguaje en todas las tareas.\n")
		if c.Requisitos != "" {
			fmt.Fprintf(&b, "- Además de la petición, el humano pidió: %s\n", c.Requisitos)
		}
		return b.String()
	}
	if c.Lenguaje == "" {
		return "El repositorio ya existe pero no se detectó un lenguaje concreto. Inspéctalo antes de elegir comandos y usa el comando de pruebas que ya exista, si lo hay."
	}
	pruebas := c.Pruebas
	if strings.TrimSpace(pruebas) == "" {
		pruebas = "ninguno detectado · propón uno que exista o el estándar del lenguaje"
	}
	return fmt.Sprintf("El repositorio ya está en desarrollo. Lenguaje detectado: %s. Comando de pruebas: %s. Usa ese lenguaje y ese comando en los \"listo_cuando\"; no cambies el stack ni propongas paquetes o rutas que no existen.", c.Lenguaje, pruebas)
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
func Generar(ctx context.Context, g Generador, c Contexto, frase string) ([]Borrador, error) {
	texto, err := g.Generar(ctx, Prompt(frase, c))
	if err != nil {
		return nil, err
	}
	return Parse(texto)
}
