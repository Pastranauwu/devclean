// Package revisor implementa la revisión de la entrega antes de
// integrarla: un modelo lee el diff completo y puede vetar el merge.
//
// Es el único paso que juzga intención en vez de mecánica. Las esclusas
// verifican que compila, que pasa, que no hay secretos y que cabe en el
// presupuesto; nada de eso dice si el código hace lo que la tarea pedía.
//
// Falla cerrado, al revés que el examinador: si el modelo no responde, o
// responde algo que no se puede leer, la entrega NO se integra. Perder un
// examen ciego cuesta una prueba; integrar sin revisar mete código sin
// mirar en la rama principal.
package revisor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Pastranauwu/devclean/internal/task"
)

// MaxDiff acota lo que se le manda al modelo. Un diff más grande no se
// revisa a medias: se reporta como no revisable y la entrega se frena.
const MaxDiff = 200_000

// DefaultTimeout es lo que se le da al revisor para responder.
const DefaultTimeout = 5 * time.Minute

// Veredicto es la respuesta del revisor.
type Veredicto struct {
	Aprobado  bool     `json:"aprobado"`
	Motivo    string   `json:"motivo"`
	Hallazgos []string `json:"hallazgos,omitempty"`
}

// Resumen arma una línea para el log y para el comentario del PR.
func (v Veredicto) Resumen() string {
	estado := "veta la integración"
	if v.Aprobado {
		estado = "aprueba"
	}
	s := "revisor " + estado
	if v.Motivo != "" {
		s += " · " + v.Motivo
	}
	for _, h := range v.Hallazgos {
		s += "\n  · " + h
	}
	return s
}

// Generador pide texto a un modelo. La interfaz mínima, como en plan.
type Generador interface {
	Generar(ctx context.Context, prompt string) (string, error)
}

// Opciones lleva lo que hace falta para revisar una entrega.
type Opciones struct {
	Generador Generador
	Tareas    []task.Task
	Diff      string
	Timeout   time.Duration
}

// Revisar pide el veredicto y lo parsea. Un error significa "no se pudo
// revisar", y quien llama no debe integrar.
func Revisar(ctx context.Context, o Opciones) (Veredicto, error) {
	if o.Generador == nil {
		return Veredicto{}, errors.New("sin revisor · declara el modelo del rol revisor en config.yml")
	}
	if strings.TrimSpace(o.Diff) == "" {
		return Veredicto{}, errors.New("no hay diff que revisar")
	}
	if len(o.Diff) > MaxDiff {
		return Veredicto{}, fmt.Errorf(
			"el diff son %d caracteres y el revisor solo cubre %d · revísalo a mano o parte la entrega",
			len(o.Diff), MaxDiff)
	}
	if o.Timeout <= 0 {
		o.Timeout = DefaultTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, o.Timeout)
	defer cancel()

	texto, err := o.Generador.Generar(ctx, Prompt(o.Tareas, o.Diff))
	if err != nil {
		return Veredicto{}, fmt.Errorf("el revisor no respondió · %s", err)
	}
	return Parse(texto)
}

// Prompt arma la instrucción del revisor: qué se pidió y qué se escribió.
func Prompt(tareas []task.Task, diff string) string {
	var b strings.Builder
	b.WriteString("Eres el revisor de devclean. Decides si esta entrega entra en la rama principal.\n\n")
	b.WriteString("Ya se verificó por código, no lo repitas: la rama rebasea limpio, cada tarea es un commit, ")
	b.WriteString("la suite del proyecto pasa sobre el conjunto, no hay secretos ni prints de debug, y cada tarea ")
	b.WriteString("cabe en su presupuesto de líneas.\n\n")
	b.WriteString("Lo que TÚ juzgas es lo que ninguna comprobación automática ve: si el código hace lo que la tarea pedía.\n\n")

	b.WriteString("Lo que se pidió:\n")
	for _, t := range tareas {
		fmt.Fprintf(&b, "- %s: %s\n", t.ID, t.Titulo)
		if t.Porque != "" {
			fmt.Fprintf(&b, "  por qué: %s\n", t.Porque)
		}
		fmt.Fprintf(&b, "  se da por hecha cuando pasa: %s\n", t.ListoCuando)
		if len(t.Expone) > 0 {
			fmt.Fprintf(&b, "  debía exponer: %s\n", strings.Join(t.Expone, "; "))
		}
	}

	b.WriteString("\nEl diff completo:\n```diff\n")
	b.WriteString(diff)
	b.WriteString("\n```\n\n")

	b.WriteString("Veta la integración SOLO por defectos concretos que puedas señalar en el diff:\n")
	b.WriteString("- el código no hace lo que su tarea decía, o lo hace solo para el caso que prueba el test\n")
	b.WriteString("- un fallo real: caso límite roto, error sin manejar que pierde datos, condición de carrera\n")
	b.WriteString("- un problema de seguridad: entrada sin validar en una frontera de confianza, secreto derivado, permiso de más\n")
	b.WriteString("- las piezas de dos tareas no encajan entre sí\n\n")
	b.WriteString("NO vetes por estilo, nombres, formato, gustos de arquitectura, ni por falta de pruebas o documentación ")
	b.WriteString("que nadie pidió. Si dudas y no puedes señalar la línea concreta, aprueba.\n\n")
	b.WriteString("Devuelve SOLO este JSON, sin texto alrededor:\n")
	b.WriteString(`{"aprobado": true|false, "motivo": "una frase", "hallazgos": ["archivo:línea — qué está mal"]}`)
	return b.String()
}

// Parse extrae el veredicto, tolerando vallas markdown y texto alrededor.
// Lo que no se puede leer no se aprueba: es un merge lo que está en juego.
func Parse(texto string) (Veredicto, error) {
	t := strings.TrimSpace(texto)
	t = strings.TrimPrefix(t, "```json")
	t = strings.TrimPrefix(t, "```")
	t = strings.TrimSuffix(t, "```")

	ini := strings.Index(t, "{")
	fin := strings.LastIndex(t, "}")
	if ini == -1 || fin <= ini {
		return Veredicto{}, errors.New("el revisor no devolvió un veredicto legible · no se integra a ciegas")
	}
	var v Veredicto
	if err := json.Unmarshal([]byte(t[ini:fin+1]), &v); err != nil {
		return Veredicto{}, fmt.Errorf("el veredicto del revisor no es JSON válido · %s", err)
	}
	if !v.Aprobado && strings.TrimSpace(v.Motivo) == "" && len(v.Hallazgos) == 0 {
		return Veredicto{}, errors.New("el revisor vetó sin decir por qué · trátalo como revisión fallida")
	}
	return v, nil
}
