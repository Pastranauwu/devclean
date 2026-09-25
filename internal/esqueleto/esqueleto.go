// Package esqueleto es la fase en que el modelo grande deja el proyecto
// listo para rellenar: estructura, ARCHITECTURE.md, tipos e interfaces
// como código real, un stub por módulo con su contrato en comentarios y
// las pruebas que lo juzgan. Después cada tarea es "haz pasar esta
// prueba tocando solo este archivo", trabajo para un modelo barato.
//
// Reemplaza al plan en prosa: las firmas que antes viajaban como texto
// en expone/usa y había que comparar a mano (firmaCanonica, examinador
// ciego, nivel semántico) ahora las valida el compilador, y el oráculo
// lo escribe el modelo caro una vez en vez del barato en cada tarea.
package esqueleto

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/Pastranauwu/devclean/internal/config"
	"github.com/Pastranauwu/devclean/internal/loop"
	"github.com/Pastranauwu/devclean/internal/plan"
	"github.com/Pastranauwu/devclean/internal/task"
)

// Marca es el texto con que lanza todo stub. La verificación no depende
// de él, pero le dice al agente que rellena qué reemplazar.
const Marca = "devclean: sin implementar"

// Documento es el archivo de arquitectura que el esqueleto deja en la
// raíz y que cada cambio siguiente evoluciona.
const Documento = "ARCHITECTURE.md"

// Resultado es lo que devuelve el arquitecto además de lo que escribió
// en disco.
type Resultado struct {
	// Verificar compila o hace typecheck del proyecto entero y hoy pasa.
	Verificar string `json:"verificar"`
	// Integracion corre la prueba de punta a punta; hoy falla por los
	// stubs y pasa cuando todo está rellenado. Es la aceptación global.
	Integracion string `json:"integracion"`
	// Pruebas corre la suite completa del proyecto. En un monorepo
	// devclean no la detecta, y la esclusa de salida la necesita.
	Pruebas      string `json:"pruebas"`
	Arquitectura string `json:"arquitectura"`
	Tareas       []plan.Borrador
}

// Pedido es lo que el humano quiere, ya redactado.
type Pedido struct {
	Texto string // feature, requirements, reglas y aceptación
	// Previo es el ARCHITECTURE.md que ya existe: el esqueleto lo
	// evoluciona en vez de reescribir el proyecto.
	Previo string
	// PrimerID es el id de la primera tarea de relleno.
	PrimerID string
}

// Prompt arma la instrucción del arquitecto. Lo fijo va primero para que
// una corrección (PromptCorregir) comparta el prefijo en caché.
func Prompt(p Pedido, c plan.Contexto) string {
	var b strings.Builder
	b.WriteString("Eres el ARQUITECTO de devclean. No implementas la lógica: dejas el proyecto listo para que muchos agentes baratos lo rellenen en paralelo, cada uno un archivo, sin verse entre sí. Lo que ellos no pueden decidir lo decides tú ahora, en código.\n\n")
	b.WriteString("PEDIDO\n")
	b.WriteString(p.Texto)
	b.WriteString("\n")
	if c.Constitucion != "" {
		b.WriteString("\nCONSTITUCIÓN DEL PROYECTO (respétala)\n" + c.Constitucion + "\n")
	}
	b.WriteString("\nREPOSITORIO\n")
	b.WriteString(contexto(c))
	if strings.TrimSpace(p.Previo) != "" {
		b.WriteString("\nYa existe " + Documento + ". Evoluciónalo: agrega lo nuevo, ajusta lo que cambia y no reescribas lo que ya funciona. Las tareas son solo para lo nuevo o lo que cambia.\n")
	}
	b.WriteString(`
ESCRIBE EN EL REPOSITORIO (tienes herramientas para crear archivos y correr comandos)
1. Estructura y stack: carpetas, manifiestos (package.json, pyproject.toml, go.mod, ...), configuración de build y del runner de pruebas, y .gitignore. Instala las dependencias y deja el build funcionando.
2. ` + Documento + ` en la raíz: estilo de arquitectura (modular, hexagonal o en capas según lo pida el problema, no por moda), cada módulo con su responsabilidad y su archivo, qué módulo puede depender de cuál, flujo de datos, tipos compartidos y cómo correr las pruebas de cada módulo. Es la fuente de verdad que leen todos los agentes y que el próximo cambio evoluciona.
3. Tipos compartidos, interfaces y puertos como código REAL y completo. Eso no se reparte: lo escribes tú.
4. Cada módulo a repartir como STUB: todas sus funciones, clases y métodos públicos con su firma exacta (tipos de entrada y de salida) y un comentario de contrato encima (qué hace, entradas, salida, errores, casos límite y los pasos sugeridos). El cuerpo solo lanza un error con el texto exacto "` + Marca + `".
5. Las pruebas de cada módulo, completas: cubren el contrato de cada stub y los requerimientos que le tocan. Hoy fallan porque el stub lanza; pasan cuando alguien lo implementa bien. Las dependencias de otros módulos entran por parámetro o constructor y en la prueba se reemplazan por fakes, para que cada módulo se implemente y se pruebe solo sin esperar a los demás. Ninguna prueba toca la red ni servicios reales.
6. Una prueba de integración de punta a punta del flujo principal con los módulos reales y el cableado (main, app, rutas) ya escrito por ti.

CÓMO REPARTIR
- Una tarea por archivo de implementación (o por dos o tres archivos muy acoplados). Cuanto más chicas e independientes, más agentes en paralelo y más barato.
- Lo trivial (cableado, configuración, tipos, reexportaciones) lo escribes tú ahora: repartirlo cuesta más que hacerlo.
- El agente que rellena es un modelo barato: no rediseña ni elige librerías. Todo lo que necesite decidir tiene que estar en el comentario del stub.
- El agente no puede tocar pruebas ni archivos fuera de su tarea: las firmas que dejes son definitivas.
- "depende_de" solo cuando la prueba de un módulo necesita de verdad la implementación real de otro; con fakes casi nunca hace falta.

ANTES DE RESPONDER VERIFÍCALO TÚ
- El comando de "verificar" pasa.
- Cada "listo_cuando" corre SOLO la prueba de su módulo y FALLA por "` + Marca + `", no por imports, sintaxis ni archivos que faltan.

RESPONDE AL FINAL SOLO CON ESTE JSON
{
  "verificar": "comando que compila o hace typecheck de todo el proyecto y hoy pasa (ej. \"npx tsc --noEmit\", \"go vet ./...\", \"python -m compileall -q src\")",
  "integracion": "comando que corre la prueba de punta a punta",
  "pruebas": "comando que corre TODA la suite del proyecto (todas las carpetas y lenguajes)",
  "arquitectura": "resumen de 5 a 15 líneas; lo completo vive en ` + Documento + `",
  "tareas": [
    {
      "titulo": "frase corta en minúscula",
      "porque": "qué requerimiento cubre",
      "listo_cuando": "comando que corre solo la prueba de este módulo",
      "tocar_solo": ["archivos de implementación de la tarea, nunca pruebas"],
      "depende_de": [],
      "peso": "liviana | media | pesada",
      "como": "qué implementar y dónde está su contrato"
    }
  ]
}
`)
	if p.PrimerID != "" {
		fmt.Fprintf(&b, "Tus tareas reciben ids correlativos desde %s en el orden del array: úsalos en \"depende_de\".\n", p.PrimerID)
	}
	return b.String()
}

// PromptCorregir repite el pedido original (mismo prefijo, mismo caché)
// y agrega lo que la verificación encontró. El repo conserva el trabajo
// anterior: se corrige, no se empieza de nuevo.
func PromptCorregir(original string, problemas []string) string {
	var b strings.Builder
	b.WriteString(original)
	b.WriteString("\nYA TRABAJASTE SOBRE ESTE REPOSITORIO. Lo que escribiste sigue en disco, pero la verificación de devclean encontró esto:\n")
	for _, p := range problemas {
		b.WriteString("- " + p + "\n")
	}
	b.WriteString("Corrígelo en el repositorio y responde otra vez con el JSON completo.\n")
	return b.String()
}

func contexto(c plan.Contexto) string {
	switch {
	case c.EsVacio && c.Stack != "":
		return "Vacío. El humano eligió el stack: " + c.Stack + ".\n"
	case c.EsVacio:
		return "Vacío. Elige el stack que mejor sirva al pedido y respeta las reglas.\n"
	case c.Lenguaje == "":
		return "Ya tiene código. Inspecciónalo antes de decidir: respeta su stack, su estructura y sus corredores de pruebas.\n"
	default:
		s := "Ya tiene código en " + c.Lenguaje + ". Respeta su stack y su estructura."
		if c.Pruebas != "" {
			s += " Pruebas del proyecto: " + c.Pruebas + "."
		}
		return s + "\n"
	}
}

// Parse lee el JSON final del arquitecto.
func Parse(texto string) (Resultado, error) {
	bs, err := plan.Parse(texto)
	if err != nil {
		return Resultado{}, err
	}
	t := strings.TrimSpace(texto)
	ini := strings.Index(t, "{")
	var r Resultado
	if ini == -1 {
		return Resultado{}, errors.New("el arquitecto no devolvió un objeto JSON")
	}
	if err := json.NewDecoder(strings.NewReader(plan.EscaparControles(t[ini:]))).Decode(&r); err != nil {
		return Resultado{}, fmt.Errorf("el arquitecto devolvió JSON inválido · %s", err)
	}
	r.Tareas = bs
	return r, nil
}

// cargaRE reconoce una prueba que no llegó a ejecutarse porque el código
// no carga: import roto, sintaxis, tipo inexistente. Un esqueleto así
// deja a cada agente barato peleando con un error que no es suyo.
var cargaRE = regexp.MustCompile(`(?i)error collecting|ImportError|ModuleNotFoundError|SyntaxError|IndentationError|Cannot find module|Failed to resolve import|Failed to load url|error TS\d+|\bundefined: |cannot find package|no required module provides|could not import|build failed|\[setup failed\]`)

// Problemas verifica el esqueleto en dir sin ningún modelo. Vacío es
// listo para repartir.
func Problemas(ctx context.Context, dir string, r Resultado, timeout time.Duration, env []string) []string {
	var out []string
	if _, err := os.Stat(filepath.Join(dir, Documento)); err != nil {
		out = append(out, "falta "+Documento+" en la raíz")
	}
	if strings.TrimSpace(r.Verificar) == "" {
		out = append(out, "falta \"verificar\": el comando que compila o hace typecheck del proyecto")
	} else if salida, code := correr(ctx, dir, r.Verificar, timeout, env); code == nil || *code != 0 {
		out = append(out, fmt.Sprintf("\"verificar\" (%s) no pasa: %s", r.Verificar, cola(salida)))
	}
	if len(r.Tareas) == 0 {
		out = append(out, "no hay tareas: si todo está hecho no hacía falta un esqueleto")
	}
	archivos := versionables(ctx, dir)
	for i, t := range r.Tareas {
		nombre := fmt.Sprintf("tarea %d (%s)", i+1, t.Titulo)
		if len(t.TocarSolo) == 0 {
			out = append(out, nombre+": tocar_solo vacío")
		}
		// que la prueba exista lo dice fallaBien: si falta, el runner
		// avisa que no corrió nada
		for _, p := range task.ArchivosDePrueba(t.ListoCuando) {
			p = strings.TrimPrefix(p, "./")
			if config.MatchesAny(t.TocarSolo, p) || algunoEn(t.TocarSolo, p) {
				out = append(out, fmt.Sprintf("%s: tocar_solo incluye su propia prueba %s; las pruebas son tuyas, no del agente", nombre, p))
			}
		}
		for _, f := range t.TocarSolo {
			if !strings.ContainsAny(f, "*?[") && !algunoCoincide([]string{f}, archivos) {
				out = append(out, fmt.Sprintf("%s: %s no existe; el stub lo creas tú", nombre, f))
			}
		}
		out = append(out, fallaBien(ctx, dir, nombre, t.ListoCuando, timeout, env)...)
	}
	if strings.TrimSpace(r.Integracion) != "" {
		out = append(out, fallaBien(ctx, dir, "integracion", r.Integracion, timeout, env)...)
	}
	return out
}

// fallaBien exige que el comando corra la prueba y falle en ella: que
// pase no verifica nada, y que no cargue no es culpa del que rellena.
func fallaBien(ctx context.Context, dir, nombre, cmd string, timeout time.Duration, env []string) []string {
	if strings.TrimSpace(cmd) == "" {
		return []string{nombre + ": sin comando de prueba"}
	}
	salida, code := correr(ctx, dir, cmd, timeout, env)
	switch {
	case code != nil && *code == 0:
		return []string{fmt.Sprintf("%s: %s ya pasa con el stub; la prueba no juzga nada", nombre, cmd)}
	case loop.PruebaNoCorrio(cmd, code, salida):
		return []string{fmt.Sprintf("%s: %s no llega a correr ninguna prueba: %s", nombre, cmd, cola(salida))}
	case cargaRE.MatchString(salida):
		return []string{fmt.Sprintf("%s: %s falla porque el código no carga, no por el stub: %s", nombre, cmd, cola(salida))}
	}
	return nil
}

func correr(ctx context.Context, dir, cmdStr string, timeout time.Duration, env []string) (string, *int) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/c", cmdStr)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", cmdStr)
	}
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		c := 124
		return string(out) + "\nse pasó del tiempo", &c
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		c := exitErr.ExitCode()
		return string(out), &c
	}
	if err != nil {
		return string(out) + err.Error(), nil
	}
	c := 0
	return string(out), &c
}

// versionables lista los archivos del árbol que git vería, sin los
// ignorados ni los excluidos (node_modules y compañía).
func versionables(ctx context.Context, dir string) []string {
	cmd := exec.CommandContext(ctx, "git", "ls-files", "-co", "--exclude-standard")
	cmd.Dir = dir
	out, _ := cmd.Output()
	return strings.Fields(string(out))
}

func algunoCoincide(patrones, archivos []string) bool {
	for _, f := range archivos {
		if config.MatchesAny(patrones, f) {
			return true
		}
	}
	return false
}

func algunoEn(lista []string, p string) bool {
	for _, x := range lista {
		if strings.TrimPrefix(x, "./") == p || strings.HasSuffix(x, "/"+p) || strings.HasSuffix(p, "/"+x) {
			return true
		}
	}
	return false
}

// cola deja las últimas líneas de una salida para el mensaje.
func cola(s string) string {
	lineas := strings.Split(strings.TrimSpace(s), "\n")
	if len(lineas) > 6 {
		lineas = lineas[len(lineas)-6:]
	}
	return strings.Join(lineas, " ⏎ ")
}
