package spec

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Pastranauwu/devclean/internal/task"
)

// RutaIntegracion es donde vive la prueba de la costura, en una ruta de
// prueba: la tarea de integración no expone nada, así que no hay examen
// ciego que proteger y la veda de rutas de prueba no le aplica.
const RutaIntegracion = "test/integracion"

// DirIntegracion es el directorio de la tarea de integración id. Uno por
// tarea y no la raíz: en un proyecto en marcha test/integracion ya tiene
// las pruebas de features anteriores —el comando pasaría hoy y la esclusa
// la rechazaría— y el planificador escribe ahí sus propias pruebas, que
// cruzaban con un test/integracion/** entero.
func DirIntegracion(id string) string {
	return RutaIntegracion + "/" + strings.ToLower(id)
}

// comandoIntegracion es el listo_cuando por lenguaje. Tiene que fallar hoy
// —el directorio no existe— y pasar cuando la prueba esté escrita.
func comandoIntegracion(lenguaje, dir string) string {
	switch lenguaje {
	case "go":
		return "go test ./" + dir + "/..."
	case "node":
		return "node --test " + dir + "/"
	case "python":
		return "pytest " + dir
	}
	return ""
}

// archivoIntegracion es el único archivo que la tarea de integración puede
// escribir. Uno solo a propósito: con el directorio entero, la del snake
// escribió 8 archivos y 1576 líneas que repetían la suite de cada tarea
// (29 turnos, 22% del costo de la corrida).
func archivoIntegracion(lenguaje, dir string) string {
	switch lenguaje {
	case "go":
		return dir + "/costura_test.go"
	case "node":
		return dir + "/costura.test.js"
	case "python":
		return dir + "/test_costura.py"
	}
	return ""
}

// LimiteLineasIntegracion acota la prueba de costura: un caso por costura
// más los límites que la consumidora no pidió caben holgados. La del snake
// original, verde con 12 tareas, tenía 288 líneas.
const LimiteLineasIntegracion = 300

// LenguajeDeComandos deduce el stack de los listo_cuando que escribió el
// planificador. Hace falta porque en un repo vacío no hay lenguaje que
// detectar todavía: el stack lo elige el plan, y sus comandos son la única
// evidencia de cuál eligió.
func LenguajeDeComandos(tasks []task.Task) string {
	for _, t := range tasks {
		c := strings.ToLower(strings.TrimSpace(t.ListoCuando))
		switch {
		case strings.HasPrefix(c, "go test"), strings.HasPrefix(c, "go build"):
			return "go"
		case strings.Contains(c, "pytest"):
			return "python"
		case strings.HasPrefix(c, "npm "), strings.HasPrefix(c, "npx "), strings.HasPrefix(c, "node "), strings.HasPrefix(c, "yarn "), strings.HasPrefix(c, "pnpm "):
			return "node"
		}
	}
	return ""
}

// TareaDeIntegracion deriva la tarea que prueba la costura entre tareas, y
// el criterio de aceptación que la vuelve a correr sobre el conjunto
// integrado.
//
// Existe porque una tarea verde no implica un feature correcto y el plan no
// cierra esa costura solo: cada tarea prueba lo que su propio contrato
// pide, y una puede prometer en su alcance un caso que su consumidora nunca
// le pidió. Verificado con cinco agentes construyendo una calculadora
// (lexer, parser, evaluador, cli): las cinco verdes, integración verde, y
// `-2^2` devolvía "unknown binary operator: ^" porque el lexer y el parser
// tenían el operador y el evaluador nunca lo pidió. Ningún listo_cuando lo
// ve, y el nivel funcional del solapamiento tampoco: corre esos mismos
// comandos.
//
// Lo que cierra el hueco es una prueba que entre por la frontera final y
// recorra la cadena entera. Se deriva del plan, no de un modelo: depende de
// todas las tareas, consume todo lo que el plan promete —así el agente que
// la implemente recibe la superficie completa en su prompt— y no expone
// nada.
//
// Devuelve false y no inventa nada cuando no hay costura que probar (una
// sola tarea, o ninguna relación entre ellas), cuando el humano ya declaró
// un comando de aceptación —ahí la costura es suya— o cuando el stack no
// tiene un comando conocido que pueda fallar hoy.
func TareaDeIntegracion(s Spec, tasks []task.Task, lenguaje, id string) (task.Task, Acceptance, bool) {
	if len(tasks) < 2 || len(s.AcceptanceCommands()) > 0 || id == "" {
		return task.Task{}, Acceptance{}, false
	}
	if lenguaje == "" {
		lenguaje = LenguajeDeComandos(tasks)
	}
	dir := DirIntegracion(id)
	comando := comandoIntegracion(lenguaje, dir)
	if comando == "" {
		return task.Task{}, Acceptance{}, false
	}

	var ids, firmas []string
	hayCostura := false
	for _, t := range tasks {
		if t.ID == "" {
			// sin ids no se puede declarar depende_de, y ordenar la
			// tarea de integración al final es todo su sentido
			return task.Task{}, Acceptance{}, false
		}
		ids = append(ids, t.ID)
		firmas = append(firmas, t.Expone...)
		if len(t.DependeDe) > 0 || len(t.Usa) > 0 {
			hayCostura = true
		}
		for _, g := range t.TocarSolo {
			if globsOverlap(g, dir+"/**") {
				// el plan ya reclamó la zona: la costura es de esa tarea
				return task.Task{}, Acceptance{}, false
			}
		}
	}
	if !hayCostura || len(firmas) == 0 {
		return task.Task{}, Acceptance{}, false
	}
	sort.Strings(ids)

	feature := strings.TrimSpace(s.Feature)
	if feature == "" {
		feature = "el feature"
	}
	t := task.Task{
		Version:      task.Version,
		ID:           id,
		Titulo:       "prueba de integración de " + feature,
		Porque:       "una tarea verde no implica un feature correcto: nadie prueba la costura entre tareas",
		ListoCuando:  comando,
		TocarSolo:    []string{archivoIntegracion(lenguaje, dir)},
		DependeDe:    ids,
		Usa:          firmas,
		Peso:         "media",
		LimiteLineas: LimiteLineasIntegracion,
		Notas:        notasIntegracion(s, tasks, archivoIntegracion(lenguaje, dir)),
	}
	return t, Acceptance{Criterion: "la costura entre tareas se prueba de punta a punta", Command: comando}, true
}

// notasIntegracion redacta la instrucción del agente que escribe la prueba.
// Es la derivación que faltaba: cada costura del plan (quién consume qué
// de quién) entra aquí, para que la prueba recorra esas y solo esas, con
// los casos que las proveedoras soportan aunque ninguna consumidora los
// haya pedido. Requerimientos y aceptación van como contexto, no como
// lista de casos: cubrirlos es trabajo de cada tarea.
func notasIntegracion(s Spec, tasks []task.Task, archivo string) string {
	var b strings.Builder
	b.WriteString("Escribe SOLO pruebas, ninguna implementación: el código ya está y estas pruebas lo juzgan de punta a punta.\n\n")
	fmt.Fprintf(&b, "Todo va en UN archivo, %s, de no más de %d líneas. La suite de cada tarea ya prueba cada pieza por separado: no repitas esos casos. Aquí solo va lo que cruza de una tarea a otra, entrando por la frontera pública del feature.\n\n", archivo, LimiteLineasIntegracion)

	b.WriteString("Costuras del plan (quién consume qué de quién), un caso por costura como mínimo:\n")
	for _, c := range costuras(tasks) {
		b.WriteString("- " + c + "\n")
	}
	b.WriteString("\nCubre también los casos que una tarea soporta y su consumidora nunca le pidió: ahí es donde la cadena se rompe con todas las piezas en verde. ")
	b.WriteString("Si una pieza acepta un operador, un formato o un caso límite, pruébalo entrando por la frontera final, no por la pieza.\n\n")

	b.WriteString("Lo que cada tarea prometió:\n")
	for _, t := range tasks {
		if len(t.Expone) == 0 {
			continue
		}
		fmt.Fprintf(&b, "- %s · %s: %s\n", t.ID, t.Titulo, strings.Join(t.Expone, ", "))
	}
	if len(s.Requirements) > 0 || len(s.Acceptance) > 0 {
		b.WriteString("\nContexto del feature (no es una lista de casos a cubrir):\n")
		for _, r := range s.Requirements {
			b.WriteString("- " + r + "\n")
		}
		for _, a := range s.Acceptance {
			b.WriteString("- aceptación: " + a.Criterion + "\n")
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

// costuras lista, por par consumidora → proveedora, lo que una usa de la
// otra; un depende_de sin usa también es costura. Se compara por nombre de
// firma, igual que ValidatePlan.
func costuras(tasks []task.Task) []string {
	dueño := map[string]string{}
	for _, t := range tasks {
		for _, f := range t.Expone {
			dueño[task.NombreDeFirma(f)] = t.ID
		}
	}
	var out []string
	for _, t := range tasks {
		usados := map[string][]string{}
		var orden []string
		for _, f := range t.Usa {
			n := task.NombreDeFirma(f)
			p, ok := dueño[n]
			if !ok || p == t.ID {
				continue
			}
			if usados[p] == nil {
				orden = append(orden, p)
			}
			usados[p] = append(usados[p], n)
		}
		for _, d := range t.DependeDe {
			if usados[d] == nil {
				orden = append(orden, d)
				usados[d] = []string{}
			}
		}
		for _, p := range orden {
			if len(usados[p]) == 0 {
				out = append(out, fmt.Sprintf("%s depende de %s", t.ID, p))
				continue
			}
			out = append(out, fmt.Sprintf("%s usa de %s: %s", t.ID, p, strings.Join(usados[p], ", ")))
		}
	}
	return out
}
