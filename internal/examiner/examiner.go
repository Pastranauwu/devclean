// Package examiner implements the blind test examiner of §6.8.
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
}

// Runner implements loop.Examinador so cmd/run can wire it without a cycle.
type Runner struct{ Options }

func (r Runner) Run(ctx context.Context, roomPath string) (bool, error) {
	return Run(ctx, roomPath, r.Options)
}

// Run invokes the examiner agent, parses the response, writes the visible
// suite to the worktree and seals the hidden suite.
// Returns (true, nil) when a hidden suite was sealed.
// Returns (false, nil) on graceful degradation (no sealed suite written).
// Never returns an error that should stop the implementer.
func Run(ctx context.Context, roomPath string, o Options) (bool, error) {
	if o.Agent == nil {
		return false, nil
	}
	// examen de caja negra: sin interfaz pública declarada no hay
	// frontera que probar. Tareas de andamiaje (init, wiring) no exponen
	// nada; examinarlas solo produce un test file de relleno que después
	// dispara falsos solapamientos entre ramas (§6.8, §6.9).
	if len(o.Task.Expone) == 0 {
		return false, nil
	}
	if o.Timeout <= 0 {
		o.Timeout = DefaultTimeout
	}

	dir, pkg := inferDirPkg(o.Task.TocarSolo, roomPath)

	prompt := buildPrompt(o.Task, pkg)
	req := loop.Request{
		RoomPath: roomPath,
		Prompt:   prompt,
		Model:    o.Model,
		Timeout:  o.Timeout,
	}
	res, err := o.Agent.Run(ctx, req)
	if err != nil {
		return false, nil // graceful degradation
	}

	text := res.Text
	if strings.TrimSpace(text) == "" {
		text = res.Stdout
	}
	visible, hidden, imports, err := parseRespText(text)
	if err != nil || len(visible) == 0 {
		return false, nil
	}

	importPath := resolveImportPath(roomPath, dir)
	visibleContent := buildGoFile(pkg, importPath, imports, visible)
	// un examinador que emite pruebas que no compilan bloquea al
	// implementador: no puede tocar el archivo (A.3) y su impl correcta
	// igual da "build failed". Si la suite ni siquiera parsea, se
	// descarta y el implementador corre sin suite ciega (§6.8).
	if !parsea(visibleContent) {
		return false, nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return false, nil
	}
	visiblePath := filepath.Join(dir, VisibleFileName)
	if err := os.WriteFile(visiblePath, []byte(visibleContent), 0o644); err != nil {
		return false, nil
	}
	// commit visible tests so the loop's revertFueraDeAlcance (A.3) does not
	// undo them — git status won't list committed files as "changed".
	commitVisible(roomPath, visiblePath)

	if len(hidden) == 0 {
		return false, nil
	}

	_, relDir := inferDirFromTocarSolo(o.Task.TocarSolo, roomPath)
	hiddenRelPath := toRelPath(filepath.Join(relDir, HiddenFileName))
	hiddenContent := buildGoFile(pkg, importPath, imports, hidden)
	if !parsea(hiddenContent) {
		return false, nil // solo visible; sin oculta que sellar
	}
	s := sealed.SuiteOculta{
		Content: hiddenContent,
		Archivo: hiddenRelPath,
	}
	if err := sealed.Write(o.Root, o.Task.ID, s); err != nil {
		return false, nil
	}
	return true, nil
}

// buildPrompt constructs the examiner instruction.
func buildPrompt(t task.Task, pkg string) string {
	var b strings.Builder
	b.WriteString("Eres el examinador ciego de devclean (§6.8). Escribes pruebas SIN ver la implementación.\n\n")
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
	fmt.Fprintf(&b, "Package Go de las pruebas: %s_test\n\n", pkg)
	b.WriteString(`Reglas:
- Pruebas de CAJA NEGRA: testea solo la interfaz pública declarada en "expone".
- NO escribas código de implementación.
- 70%% van en "visible": el implementador las verá como criterio de aceptación.
- 30%% van en "hidden": edge cases y casos límite que el implementador NO verá.
- Los tests "hidden" deben fallar si la implementación es superficial o solo optimizada para "visible".
- Usa solo la stdlib de Go. Sin imports externos.
- En "imports" enumera TODO paquete de la stdlib que usen tus tests aparte de "testing" (ej. "net", "time", "bytes", "encoding/json"). Si olvidas uno, la suite no compila y se descarta.
- No pongas el bloque import en el código: solo las funciones. devclean arma el archivo.
- Cada función de test es independiente.

Devuelve SOLO este JSON (sin texto alrededor, sin markdown):
{
  "imports": ["net", "time", "bytes"],
  "visible": ["func TestXxx(t *testing.T) {\n\t// ...\n}", "..."],
  "hidden":  ["func TestYyy(t *testing.T) {\n\t// ...\n}", "..."]
}`)
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
	fmt.Fprintf(&b, "package %s_test\n\nimport (\n\t\"testing\"\n", pkg)
	if importPath != "" {
		fmt.Fprintf(&b, "\t%q\n", importPath)
	}
	for _, imp := range dedup(extra) {
		if !stdlibImport(imp) || imp == "testing" {
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

// parsea reporta si el archivo de pruebas al menos compila a nivel
// sintáctico. No detecta errores de tipos (que la impl aún no exista es
// TDD legítimo), solo que el examinador no devolvió basura.
func parsea(src string) bool {
	_, err := parser.ParseFile(token.NewFileSet(), "devclean_test.go", src, parser.AllErrors)
	return err == nil
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

func toRelDir(tocarSolo []string) string {
	_, relDir := inferDirFromTocarSolo(tocarSolo, "")
	return relDir
}

func toRelPath(p string) string { return filepath.ToSlash(p) }

// commitVisible commits the visible test file to the worktree so the
// implementer loop won't revert it (A.3 applies to changes, not to
// already-committed files).
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
