package spec

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Pastranauwu/devclean/internal/task"
)

// RutaIntegracion es donde vive la prueba de la costura. Fuera del alcance
// de cualquier otra tarea, para que no haya cruce, y en una ruta de prueba:
// la tarea de integración no expone nada, así que no hay examen ciego que
// proteger y la veda de rutas de prueba no le aplica.
const RutaIntegracion = "test/integracion"

// comandoIntegracion es el listo_cuando por lenguaje. Tiene que fallar hoy
// —el directorio no existe— y pasar cuando la prueba esté escrita.
func comandoIntegracion(lenguaje string) string {
	switch lenguaje {
	case "go":
		return "go test ./" + RutaIntegracion + "/..."
	case "node":
		return "node --test " + RutaIntegracion + "/"
	case "python":
		return "pytest " + RutaIntegracion
	}
	return ""
}

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
func TareaDeIntegracion(s Spec, tasks []task.Task, lenguaje string) (task.Task, Acceptance, bool) {
	if len(tasks) < 2 || len(s.AcceptanceCommands()) > 0 {
		return task.Task{}, Acceptance{}, false
	}
	if lenguaje == "" {
		lenguaje = LenguajeDeComandos(tasks)
	}
	comando := comandoIntegracion(lenguaje)
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
		Version:     task.Version,
		Titulo:      "prueba de integración de " + feature,
		Porque:      "una tarea verde no implica un feature correcto: nadie prueba la costura entre tareas",
		ListoCuando: comando,
		TocarSolo:   []string{RutaIntegracion + "/**"},
		DependeDe:   ids,
		Usa:         firmas,
		Peso:        "media",
		Notas:       notasIntegracion(s, tasks),
	}
	return t, Acceptance{Criterion: "la costura entre tareas se prueba de punta a punta", Command: comando}, true
}

// notasIntegracion redacta la instrucción del agente que escribe la prueba.
// Es la derivación que faltaba: lo que cada tarea prometió entra aquí, para
// que la prueba cubra los casos que las proveedoras soportan aunque ninguna
// consumidora los haya pedido.
func notasIntegracion(s Spec, tasks []task.Task) string {
	var b strings.Builder
	b.WriteString("Escribe SOLO pruebas, ninguna implementación: el código ya está y estas pruebas lo juzgan de punta a punta.\n\n")
	b.WriteString("Entra por la frontera pública del feature y recorre la cadena completa, no cada pieza por separado (para eso ya está la suite de cada tarea).\n\n")

	if len(s.Requirements) > 0 {
		b.WriteString("Cada requerimiento necesita al menos un caso:\n")
		for _, r := range s.Requirements {
			b.WriteString("- " + r + "\n")
		}
		b.WriteString("\n")
	}
	if len(s.Acceptance) > 0 {
		b.WriteString("Criterios de aceptación declarados:\n")
		for _, a := range s.Acceptance {
			b.WriteString("- " + a.Criterion + "\n")
		}
		b.WriteString("\n")
	}

	b.WriteString("Lo que cada tarea prometió:\n")
	for _, t := range tasks {
		if len(t.Expone) == 0 {
			continue
		}
		fmt.Fprintf(&b, "- %s · %s: %s\n", t.ID, t.Titulo, strings.Join(t.Expone, ", "))
	}
	b.WriteString("\nCubre también los casos que una tarea soporta y su consumidora nunca le pidió: ahí es donde la cadena se rompe con todas las piezas en verde. ")
	b.WriteString("Si una pieza acepta un operador, un formato o un caso límite, pruébalo entrando por la frontera final, no por la pieza.")
	return b.String()
}
