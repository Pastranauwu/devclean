// Package examiner implements the blind test examiner.
// The examiner sees only the task contract and the public boundary
// (expone signatures, endpoints, CLI). It never sees function bodies.
package examiner

import (
	"context"
	"encoding/json"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Pastranauwu/devclean/internal/loop"
	"github.com/Pastranauwu/devclean/internal/sealed"
	"github.com/Pastranauwu/devclean/internal/task"
)

const (
	VisibleFileName = "devclean_visible_test.go"
	HiddenFileName  = "devclean_hidden_test.go"
	DefaultTimeout  = 3 * time.Minute
)

// Options configures one examiner run.
type Options struct {
	Agent   loop.Agent
	Task    task.Task
	Root    string
	Model   string
	Timeout time.Duration

	// Lenguaje es el stack del proyecto (config.DetectLanguage), y decide
	// cómo se arma y se valida la suite. Vacío se trata como go. Un stack
	// sin examinador (rust, node) salta el examen.
	Lenguaje string
}

// Runner implements loop.Examinador so cmd/run can wire it without a cycle.
type Runner struct{ Options }

func (r Runner) Run(ctx context.Context, roomPath string) (bool, error) {
	return Run(ctx, roomPath, r.Options)
}

// Run invokes the examiner agent, parses the response, writes the visible
// suite to the worktree and seals the hidden suite.
// Returns (true, nil) when a hidden suite was sealed.
// Returns (false, nil) when la tarea no es examinable: no hay examen que
// hacer y no falló nada.
// Returns (false, err) on graceful degradation, con el motivo: el error
// NO debe frenar al implementador, solo queda registrado. Degradar en
// silencio dejaba la tarea sin suite y sin rastro de por qué: la veda de
// rutas de prueba sigue activa, la reversión de alcance le quita al
// implementador las suyas y `listo_cuando` pasa sin ejecutar nada.
func Run(ctx context.Context, roomPath string, o Options) (bool, error) {
	if o.Agent == nil {
		return false, nil
	}
	// examen de caja negra: sin interfaz pública declarada no hay
	// frontera que probar. Tareas de andamiaje (init, wiring) no exponen
	// nada; examinarlas solo produce un test file de relleno que después
	// dispara falsos solapamientos entre ramas.
	if len(o.Task.Expone) == 0 {
		return false, nil
	}
	// un stack sin examinador (rust, node) no se examina: emitir un
	// archivo de otro lenguaje solo rompería la compilación del cuarto.
	lenguaje := lenguajeExamen(o.Lenguaje)
	if lenguaje == "" {
		return false, nil
	}
	if o.Timeout <= 0 {
		o.Timeout = DefaultTimeout
	}

	dir, pkg := inferDirPkg(o.Task.TocarSolo, roomPath)

	// Todo lo que puede invalidar el examen se resuelve ANTES de invocar
	// al modelo: descubrirlo después es pagar una llamada entera para
	// tirarla a la basura.
	var importPath string
	if lenguaje == "go" {
		// El paquete real manda sobre el nombre del directorio. Adivinarlo
		// rompía en el caso más común de Go: `cmd/algo` declara `package
		// main`, no `package algo`, y la suite terminaba con dos paquetes
		// en el mismo directorio — un error que el implementador no puede
		// arreglar, porque las rutas de prueba le están vedadas.
		real := paqueteReal(dir)
		switch real {
		case "":
			// todavía no hay código ahí (tarea de paquete nuevo, el caso
			// más común al arrancar). Renunciar al examen dejaba a esas
			// tareas sin suite: el implementador escribe sus propias
			// pruebas, la reversión de alcance se las quita y
			// `go test ./pkg/...` pasa sin ejecutar nada. El nombre sale
			// del contrato —`expone: ["numeros.Media(...)"]` ya lo
			// declara—, así que el implementador que sigue el contrato
			// coincide con la suite.
			if n := paqueteDeExpone(o.Task.Expone); n != "" {
				pkg = n
			}
		case "main":
			// Go no deja importar un paquete main: no hay examen de caja
			// negra posible sobre un binario desde otro paquete
			return false, nil
		default:
			// el paquete que ya vive en el directorio manda sobre
			// cualquier inferencia
			pkg = real
		}
		// sin nombre no hay suite posible: `package _test` en el mismo
		// directorio rompe la compilación del cuarto y el implementador
		// no puede arreglarlo. Y un `main` inferido cae en la misma
		// pared que el real: `cmd/algo` con `expone: ["main.run(...)"]`
		// daba una suite `package main_test` que importa el binario, y Go
		// responde "imported as main and not used".
		if pkg == "" || pkg == "main" {
			return false, nil
		}

		// El examinador corre ANTES que el implementador, así que en un
		// repo recién nacido todavía no hay go.mod. Sin ruta de import la
		// suite llama a `pkg.Func()` sin importar `pkg`: no compila nunca
		// y nadie puede tocarla. Sin ruta, no hay examen.
		importPath = resolveImportPath(roomPath, dir)
		if importPath == "" {
			return false, nil
		}
	}

	prompt := buildPrompt(o.Task, pkg, lenguaje)
	req := loop.Request{
		RoomPath: roomPath,
		Prompt:   prompt,
		Model:    o.Model,
		Timeout:  o.Timeout,
		Texto:    true,
	}
	res, err := o.Agent.Run(ctx, req)
	if err != nil {
		return false, fmt.Errorf("el examinador no pudo invocar al modelo · %s", err)
	}

	text := res.Text
	if strings.TrimSpace(text) == "" {
		text = res.Stdout
	}
	visible, hidden, imports, err := parseRespText(text)
	if err != nil {
		return false, fmt.Errorf("la respuesta del examinador no se pudo leer · %s", err)
	}
	if len(visible) == 0 {
		return false, fmt.Errorf("el examinador no devolvió ninguna prueba visible")
	}

	visibleRelPath, hiddenRelPath := RutasSuite(o.Task.TocarSolo, lenguaje)

	visibleContent, okVisible := suiteCompleta(lenguaje, roomPath, pkg, importPath, imports, visible)
	if !okVisible {
		return false, fmt.Errorf("no se pudieron resolver los imports de la suite visible")
	}
	// un examinador que emite pruebas que no compilan bloquea al
	// implementador: no puede tocar el archivo y su impl correcta
	// igual da "build failed". Si la suite ni siquiera parsea, se
	// descarta y el implementador corre sin suite ciega.
	if err := validarSintaxis(lenguaje, visibleContent); err != nil {
		return false, fmt.Errorf("la suite visible del examinador no compila · %s", err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return false, fmt.Errorf("no se pudo crear %s · %s", dir, err)
	}
	visiblePath := filepath.Join(roomPath, filepath.FromSlash(visibleRelPath))
	if err := os.WriteFile(visiblePath, []byte(visibleContent), 0o644); err != nil {
		return false, fmt.Errorf("no se pudo escribir la suite visible · %s", err)
	}
	// commit visible tests so the loop's revertFueraDeAlcance does not
	// undo them — git status won't list committed files as "changed".
	commitVisible(roomPath, visiblePath)

	if len(hidden) == 0 {
		return false, fmt.Errorf("el examinador no devolvió suite oculta · el paso suite_oculta se omite")
	}

	hiddenContent, okHidden := suiteCompleta(lenguaje, roomPath, pkg, importPath, imports, hidden)
	if !okHidden {
		return false, fmt.Errorf("no se pudieron resolver los imports de la suite oculta · solo queda la visible")
	}
	if err := validarSintaxis(lenguaje, hiddenContent); err != nil {
		return false, fmt.Errorf("la suite oculta del examinador no compila · solo queda la visible · %s", err)
	}
	s := sealed.SuiteOculta{
		Content: hiddenContent,
		Archivo: hiddenRelPath,
	}
	if err := sealed.Write(o.Root, o.Task.ID, s); err != nil {
		return false, fmt.Errorf("no se pudo sellar la suite oculta · %s", err)
	}
	return true, nil
}

// buildPrompt constructs the examiner instruction. lenguaje va explícito
// en el prompt: sin decirlo, el modelo asume Go y devuelve una suite que
// el proyecto no puede correr.
func buildPrompt(t task.Task, pkg, lenguaje string) string {
	var b strings.Builder
	b.WriteString("Eres el examinador ciego de devclean. Escribes pruebas SIN ver la implementación.\n\n")
	fmt.Fprintf(&b, "Tarea: %s — %s\n", t.ID, t.Titulo)
	if t.Porque != "" {
		fmt.Fprintf(&b, "Por qué: %s\n", t.Porque)
	}
	fmt.Fprintf(&b, "Listo cuando: %s\n", t.ListoCuando)
	if len(t.Expone) > 0 {
		fmt.Fprintf(&b, "Contrato público (firmas que debe exponer): %s\n", strings.Join(t.Expone, "; "))
	}
	if len(t.TocarSolo) > 0 {
		fmt.Fprintf(&b, "Archivos que puede tocar: %s\n", strings.Join(t.TocarSolo, ", "))
	}
	if t.Riesgos != "" {
		fmt.Fprintf(&b, "Riesgos: %s\n", t.Riesgos)
	}
	fmt.Fprintf(&b, "Lenguaje de las pruebas: %s · escribe SOLO en ese lenguaje.\n", lenguaje)
	if lenguaje == "go" {
		fmt.Fprintf(&b, "Package Go de las pruebas: %s_test\n", pkg)
	}
	b.WriteString(`
Reglas:
- Pruebas de CAJA NEGRA: testea solo la interfaz pública declarada en "expone".
- NO escribas código de implementación.
- 70% van en "visible": el implementador las verá como criterio de aceptación.
- 30% van en "hidden": edge cases y casos límite que el implementador NO verá.
- Los tests "hidden" deben fallar si la implementación es superficial o solo optimizada para "visible".
- Cada función de test es independiente.
`)
	b.WriteString(reglasDe(lenguaje))
	return b.String()
}

type suiteJSON struct {
	Imports []string `json:"imports"`
	Visible []string `json:"visible"`
	Hidden  []string `json:"hidden"`
}

func parseRespText(text string) (visible, hidden, imports []string, err error) {
	t := strings.TrimSpace(text)
	t = strings.TrimPrefix(t, "```json")
	t = strings.TrimPrefix(t, "```")
	t = strings.TrimSuffix(t, "```")
	t = strings.TrimSpace(t)

	ini := strings.Index(t, "{")
	fin := strings.LastIndex(t, "}")
	if ini == -1 || fin <= ini {
		return nil, nil, nil, fmt.Errorf("no JSON in response")
	}
	var s suiteJSON
	if err := json.Unmarshal([]byte(t[ini:fin+1]), &s); err != nil {
		return nil, nil, nil, err
	}
	return s.Visible, s.Hidden, s.Imports, nil
}

// buildGoFile assembles a complete Go external test file.
// importPath is the full import path of the package under test (e.g.
// "mymod/calculator"). Empty string means no package import is added.
// extra are stdlib packages declared by the examiner; only those a test
// body actually references are kept, so a package used only by the other
// suite does not turn into an "imported and not used" build error.
func buildGoFile(pkg, importPath string, extra, funcs []string) string {
	body := strings.Join(funcs, "\n")

	var b strings.Builder
	// el encabezado le dice al implementador cómo tiene que llamarse su
	// paquete: la suite es externa (pkg_test) y, si él elige otro nombre,
	// Go responde "found packages X and pkg (…_test.go)" — un error que
	// no puede arreglar, porque esta ruta le está vedada.
	fmt.Fprintf(&b, "// devclean · suite del examinador ciego: criterio de aceptación, no editable.\n")
	fmt.Fprintf(&b, "// El paquete de este directorio tiene que llamarse %q.\n\n", pkg)
	fmt.Fprintf(&b, "package %s_test\n\nimport (\n\t\"testing\"\n", pkg)
	if importPath != "" {
		fmt.Fprintf(&b, "\t%q\n", importPath)
	}
	for _, imp := range dedup(extra) {
		if !importPermitido(imp, importPath) || imp == "testing" {
			continue
		}
		if !strings.Contains(body, selector(imp)+".") {
			continue
		}
		fmt.Fprintf(&b, "\t%q\n", imp)
	}
	b.WriteString(")\n\n")
	for _, f := range funcs {
		b.WriteString(f)
		if !strings.HasSuffix(strings.TrimSpace(f), "}") {
			b.WriteString("\n}")
		}
		b.WriteString("\n\n")
	}
	return b.String()
}

// stdlibImport descarta paquetes externos: el primer segmento de la
// stdlib nunca lleva punto (dominio).
func stdlibImport(path string) bool {
	if path == "" {
		return false
	}
	first := path
	if i := strings.IndexByte(path, '/'); i >= 0 {
		first = path[:i]
	}
	return !strings.Contains(first, ".")
}

// importPermitido acepta la stdlib y los paquetes del mismo módulo que el
// paquete bajo prueba. Filtrar por stdlibImport a secas tiraba el import
// hermano cuando la ruta del módulo lleva punto ("github.com/x/y"), y la
// suite quedaba con `ast.Number` sin importar ast: no compila y el
// implementador no puede tocarla.
func importPermitido(imp, importPath string) bool {
	if stdlibImport(imp) {
		return true
	}
	a, b := primerSegmento(imp), primerSegmento(importPath)
	return a != "" && a == b
}

func primerSegmento(p string) string {
	if i := strings.IndexByte(p, '/'); i >= 0 {
		return p[:i]
	}
	return p
}

// selector devuelve el identificador con que se referencia un import:
// el último segmento de la ruta ("encoding/json" → "json").
func selector(path string) string {
	if i := strings.LastIndexByte(path, '/'); i >= 0 {
		return path[i+1:]
	}
	return path
}

func dedup(xs []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, x := range xs {
		if x == "" || seen[x] {
			continue
		}
		seen[x] = true
		out = append(out, x)
	}
	return out
}

// resolveImportPath reads go.mod to find the module name, then computes
// the import path for the package in dir. Returns "" on any failure.
func resolveImportPath(roomPath, dir string) string {
	data, err := os.ReadFile(filepath.Join(roomPath, "go.mod"))
	if err != nil {
		return ""
	}
	// find "module <name>" line
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "module ") {
			continue
		}
		mod := strings.TrimSpace(strings.TrimPrefix(line, "module"))
		rel, err := filepath.Rel(roomPath, dir)
		if err != nil || rel == "." || rel == "" {
			return mod
		}
		return mod + "/" + filepath.ToSlash(rel)
	}
	return ""
}

// inferDirPkg returns the absolute worktree directory and the Go package
// name inferred from tocar_solo, skipping file-like entries (e.g. go.mod).
// Falls back to roomPath and "main".
func inferDirPkg(tocarSolo []string, roomPath string) (dir, pkg string) {
	absDir, relDir := inferDirFromTocarSolo(tocarSolo, roomPath)
	if relDir == "." {
		return absDir, "main"
	}
	parts := strings.Split(relDir, "/")
	pkg = parts[len(parts)-1]
	if pkg == "" || pkg == "." {
		pkg = "main"
	}
	return absDir, pkg
}

// paqueteDeExpone saca el nombre de paquete del prefijo de las firmas
// prometidas ("numeros.Media(xs []float64) (float64, error)" → numeros).
// Devuelve "" si ninguna firma viene calificada.
func paqueteDeExpone(expone []string) string {
	for _, f := range expone {
		f = strings.TrimSpace(f)
		i := strings.Index(f, ".")
		if i <= 0 {
			continue
		}
		n := f[:i]
		if !esIdentificador(n) {
			continue
		}
		return n
	}
	return ""
}

// esIdentificador filtra prefijos que no son un nombre de paquete Go
// (rutas, firmas sin calificar, ruido del planificador).
func esIdentificador(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r == '_':
		case r >= '0' && r <= '9' && i > 0:
		default:
			return false
		}
	}
	return true
}

// paqueteReal lee el nombre de paquete declarado en los .go que ya viven
// en dir. Devuelve "" si el directorio no existe o todavía no tiene
// código.
//
// Adivinarlo desde el nombre del directorio rompía en el caso más común
// de Go: `cmd/algo` declara `package main`, no `package algo`. La suite
// salía como `package algo_test` en el mismo directorio y `go build`
// respondía "found packages algo (…_test.go) and main (main.go)" — un
// error que el implementador no puede arreglar, porque las rutas de
// prueba le están vedadas. La tarea quedaba roja para siempre.
func paqueteReal(dir string) string {
	entradas, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	fset := token.NewFileSet()
	for _, e := range entradas {
		nombre := e.Name()
		if e.IsDir() || !strings.HasSuffix(nombre, ".go") || strings.HasSuffix(nombre, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, nombre), nil, parser.PackageClauseOnly)
		if err != nil || f.Name == nil {
			continue
		}
		return f.Name.Name
	}
	return ""
}

func toRelPath(p string) string { return filepath.ToSlash(p) }

// commitVisible commits the visible test file to the worktree so the
// implementer loop won't revert it (the revert applies to changes, not
// to already-committed files).
func commitVisible(roomPath, absPath string) {
	rel, err := filepath.Rel(roomPath, absPath)
	if err != nil {
		return
	}
	git := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", roomPath}, args...)...)
		_ = cmd.Run()
	}
	git("-c", "user.name=devclean", "-c", "user.email=devclean@local", "add", rel)
	git("-c", "user.name=devclean", "-c", "user.email=devclean@local", "commit", "-m", "exam: suite visible")
}

// inferDirFromTocarSolo finds the best directory from tocar_solo:
//   - "internal/wol/**" → "internal/wol" (strip glob)
//   - "calculator/calculator.go" → "calculator" (take parent of a specific file)
//   - "go.mod" → skip (top-level dotfile, not a package dir)
//
// Falls back to roomPath when no useful entry is found.
func inferDirFromTocarSolo(tocarSolo []string, roomPath string) (absDir, relDir string) {
	for _, g := range tocarSolo {
		rel := strings.TrimRight(g, "/*")
		if rel == "" || rel == "." {
			return roomPath, "."
		}
		base := filepath.Base(rel)
		if strings.Contains(base, ".") {
			// specific file — use its parent directory
			parent := filepath.Dir(rel)
			if parent == "." || parent == "" {
				// top-level file like "go.mod" — skip to next entry
				continue
			}
			if roomPath != "" {
				return filepath.Join(roomPath, parent), parent
			}
			return parent, parent
		}
		if roomPath != "" {
			return filepath.Join(roomPath, rel), rel
		}
		return rel, rel
	}
	return roomPath, "."
}
