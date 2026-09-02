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
	"sort"
	"strconv"
	"strings"

	"github.com/Pastranauwu/devclean/internal/config"
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
	Expone      []string `json:"expone"`
	Usa         []string `json:"usa"`
	Riesgos     string   `json:"riesgos"`
	Peso        string   `json:"peso"`
	Agente      string   `json:"agente,omitempty"`
	// LimiteLineas es el presupuesto de líneas añadidas que el
	// planificador estima para esta tarea. Antes era una constante de
	// 200 para todo, que no mira el alcance: una tarea de empaquetado
	// con documentación nunca cabe en 200 líneas, y la esclusa de salida
	// la frenaba después de que el trabajo ya estaba hecho.
	LimiteLineas int `json:"limite_lineas"`

	// Como es el enfoque que sugiere el orquestador: cómo encarar la
	// tarea, qué tocar primero, a qué no meterse. Es orientación para el
	// ejecutor, no contrato vinculante como listo_cuando — se inyecta en
	// el prompt como nota, y en una descomposición recursiva es lo único
	// que le dice al agente chico cómo cumplir su parte sin inventar.
	Como string `json:"como,omitempty"`
}

// UnmarshalJSON tolera que el modelo escriba las dependencias como
// números (`"depende_de": [1]`) o como ids completos (`["T-002"]`):
// `depende_de` es []string en el contrato, y un JSON numérico rompía
// toda la descomposición recursiva.
func (b *Borrador) UnmarshalJSON(data []byte) error {
	type alias Borrador
	var raw struct {
		*alias
		DependeDe json.RawMessage `json:"depende_de"`
	}
	a := alias(*b)
	raw.alias = &a
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*b = Borrador(a)
	if len(raw.DependeDe) == 0 {
		return nil
	}
	var items []json.RawMessage
	if err := json.Unmarshal(raw.DependeDe, &items); err != nil {
		return err
	}
	deps := make([]string, 0, len(items))
	for _, it := range items {
		var s string
		if json.Unmarshal(it, &s) == nil {
			deps = append(deps, s)
			continue
		}
		var n float64
		if json.Unmarshal(it, &n) == nil {
			deps = append(deps, strconv.FormatFloat(n, 'f', -1, 64))
			continue
		}
		deps = append(deps, strings.Trim(string(it), `"`))
	}
	b.DependeDe = deps
	return nil
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
	Lenguaje     string                   // go, node, python, rust, "" si no se detecta
	EsVacio      bool                     // repo sin código fuente todavía
	Pruebas      string                   // comando de pruebas detectado ("" si no hay)
	Stack        string                   // stack elegido por el humano ("" si lo decide el modelo)
	Requisitos   string                   // requisitos extra que dijo el humano, en texto libre
	Constitucion string                   // contenido de .devclean/constitution.md (§6.11), "" si no existe
	Vedadas      []string                 // globs que tocar_solo nunca puede incluir (zonas prohibidas + rutas de prueba)
	Agentes      map[string]config.Agente // agentes disponibles en config.yml (§8.1 / Fase 2)
	// PruebasPropias marca que este stack no tiene examinador ciego, así
	// que las pruebas las escribe la propia tarea y su archivo tiene que
	// entrar en tocar_solo. Sin decirlo, el planificador apunta el
	// listo_cuando a un archivo de prueba que deja fuera de alcance, y
	// nadie puede crearlo: la tarea queda roja para siempre.
	PruebasPropias bool
}

// Prompt arma la instrucción para el planificador: una frase entra, un
// array JSON de contratos sale. El contexto del repo se inyecta para
// que el modelo use el lenguaje y el comando de pruebas reales.
func Prompt(frase string, c Contexto) string {
	var b strings.Builder
	b.WriteString("Eres el planificador de devclean. Parte esta petición en tareas independientes, pequeñas y verificables:\n\n")
	if c.Constitucion != "" {
		b.WriteString("Constitución del proyecto (convenciones establecidas que el plan debe respetar):\n")
		b.WriteString(c.Constitucion)
		b.WriteString("\n\n")
	}
	b.WriteString("\"" + frase + "\"\n\n")
	b.WriteString(contextoPrompt(c))
	b.WriteString("\n\nDevuelve SOLO un array JSON, sin texto alrededor, con estos campos por tarea:\n")
	b.WriteString("- \"titulo\": frase corta en minúscula\n")
	b.WriteString("- \"porque\": por qué importa (una frase)\n")
	b.WriteString("- \"listo_cuando\": un comando ejecutable que HOY FALLE y que pase cuando la tarea esté hecha (obligatorio).\n")
	b.WriteString("  · devclean lo ejecuta ANTES de gastar un solo token y rechaza la tarea si ya pasa: un comando que hoy da verde no verifica nada.\n")
	b.WriteString("  · Por eso no sirve el comando de pruebas del proyecto tal cual si la suite ya está verde (`npm test`, `go test ./...`, `pytest`): acótalo a lo que esta tarea va a crear — el archivo de prueba, el módulo o el paquete que todavía no existe.\n")
	b.WriteString("  · Ejemplos que fallan hoy porque el destino no existe: \"go test ./internal/wol/...\", \"node --test test/validator.test.js\", \"pytest tests/test_wol.py\", \"npm test -- validator\".\n")
	b.WriteString("- \"tocar_solo\": array de globs de archivos que la tarea puede tocar\n")
	if c.PruebasPropias {
		b.WriteString("  · en este proyecto las pruebas las escribe la propia tarea: si el \"listo_cuando\" apunta a un archivo de prueba, ESE archivo tiene que estar también en \"tocar_solo\", o nadie podrá crearlo.\n")
	}
	if len(c.Vedadas) > 0 {
		b.WriteString("  · \"tocar_solo\" NUNCA puede incluir estas rutas (las maneja el proyecto o el examinador ciego, no la tarea): " + strings.Join(c.Vedadas, ", ") + "\n")
	}
	b.WriteString("- \"depende_de\": array de ids (ej. \"T-001\") de tareas que deben estar verdes antes que esta; vacío si no depende de ninguna\n")
	b.WriteString("- \"expone\": array de firmas públicas que esta tarea produce y otra consume (ej. \"wol.Send(mac, addr string) error\", \"POST /wake\"); vacío si no produce ninguna\n")
	b.WriteString("- \"usa\": array de firmas de OTRAS tareas que esta consume, copiadas palabra por palabra del \"expone\" de aquella; vacío si no consume ninguna\n")
	b.WriteString("- \"peso\": \"liviana\", \"media\" o \"pesada\" según la complejidad de la tarea (por defecto \"media\")\n")
	b.WriteString("- \"limite_lineas\": cuántas líneas de código NUEVO crees que necesita esta tarea, con algo de margen. Estímalo por el alcance real: un archivo de configuración son decenas, un módulo con su lógica unos cientos, empaquetado con documentación puede ser más de mil. Es un tope que se verifica al entregar: quedarse corto frena la entrega de trabajo correcto, y pasarse de largo deja de avisar cuando una tarea se desborda. Si no cabe en unas 600 líneas, probablemente son dos tareas: pártela.\n")
	if len(c.Agentes) > 0 {
		var ags []string
		for nombre, a := range c.Agentes {
			desc := nombre
			if len(a.Skills) > 0 {
				desc += fmt.Sprintf(" (habilidades: %s)", strings.Join(a.Skills, ", "))
			}
			ags = append(ags, desc)
		}
		sort.Strings(ags)
		fmt.Fprintf(&b, "- \"agente\": nombre del agente asignado para esta tarea (disponibles: %s); o \"\" para el ejecutor por defecto\n", strings.Join(ags, "; "))
	}
	b.WriteString("- \"riesgos\": riesgos o limitaciones, o \"\" si no hay\n")
	b.WriteString("- \"como\": una línea corta con el enfoque — cómo encarar la tarea, qué tocar primero, a qué no meterse. Es la instrucción que le deja el orquestador al agente que la ejecuta, no el criterio de éxito (eso es listo_cuando)\n\n")
	b.WriteString("Las tareas corren en paralelo y aisladas: no pueden leerse el código entre sí. Si una produce algo que otra necesita, la firma DEBE aparecer igual en el \"expone\" de la que la produce y en el \"usa\" de la que la consume; si no, cada una inventará la suya y no van a encajar.\n\n")
	b.WriteString("Ejemplo:\n[\n")
	b.WriteString("  {\"titulo\": \"enviar magic packet\", \"porque\": \"es la acción central\", \"listo_cuando\": \"go test ./internal/wol/...\", \"tocar_solo\": [\"internal/wol/**\"], \"expone\": [\"wol.Send(mac, addr string) error\"], \"usa\": [], \"peso\": \"media\", \"limite_lineas\": 250, \"riesgos\": \"\"},\n")
	b.WriteString("  {\"titulo\": \"endpoint http que dispara wol\", \"porque\": \"lo invoca la automatización\", \"listo_cuando\": \"go test ./internal/api/...\", \"tocar_solo\": [\"internal/api/**\"], \"expone\": [\"POST /wake\"], \"usa\": [\"wol.Send(mac, addr string) error\"], \"peso\": \"media\", \"limite_lineas\": 180, \"riesgos\": \"\"}\n")
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
	return fmt.Sprintf("El repositorio ya está en desarrollo. Lenguaje detectado: %s. Comando de pruebas del proyecto: %s.\n"+
		"- Mantén ese lenguaje y ese corredor de pruebas; no cambies el stack ni propongas paquetes o rutas que no existen.\n"+
		"- Pero NO copies ese comando tal cual en los \"listo_cuando\" si la suite del proyecto ya está verde: entonces pasaría hoy y devclean rechazaría la tarea. Apunta con el mismo corredor a lo que la tarea va a crear y que todavía no existe.",
		c.Lenguaje, pruebas)
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

// Límites del presupuesto que propone el planificador. Es un modelo:
// puede devolver 0 (no lo estimó), un número absurdo, o uno tan generoso
// que la esclusa deje de avisar de nada.
const (
	LimiteLineasMin = 40
	LimiteLineasMax = 2000
)

// AcotarLimiteLineas devuelve el presupuesto que se escribe en el
// contrato: el propuesto si es razonable, o el más cercano que lo sea.
// Un 0 significa "el modelo no lo estimó" y cae en porDefecto.
func AcotarLimiteLineas(propuesto, porDefecto int) int {
	if propuesto <= 0 {
		return porDefecto
	}
	if propuesto < LimiteLineasMin {
		return LimiteLineasMin
	}
	if propuesto > LimiteLineasMax {
		return LimiteLineasMax
	}
	return propuesto
}
