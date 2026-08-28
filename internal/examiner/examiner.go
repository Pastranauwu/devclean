// Package examiner implements the blind test examiner of §6.8.
// The examiner sees only the task contract and the public boundary
// (expone signatures, endpoints, CLI). It never sees function bodies.
package examiner

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
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

	visible, hidden, err := parseRespText(res.Stdout)
	if err != nil || len(visible) == 0 {
		return false, nil
	}

	visibleContent := buildGoFile(pkg, visible)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return false, nil
	}
	if err := os.WriteFile(filepath.Join(dir, VisibleFileName), []byte(visibleContent), 0o644); err != nil {
		return false, nil
	}

	if len(hidden) == 0 {
		return false, nil
	}

	hiddenRelPath := toRelPath(filepath.Join(toRelDir(o.Task.TocarSolo), HiddenFileName))
	s := sealed.SuiteOculta{
		Content: buildGoFile(pkg, hidden),
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
- Usa solo la stdlib de Go (testing, fmt, errors, etc.). Sin imports externos.
- Cada función de test es independiente.

Devuelve SOLO este JSON (sin texto alrededor, sin markdown):
{
  "visible": ["func TestXxx(t *testing.T) {\n\t// ...\n}", "..."],
  "hidden":  ["func TestYyy(t *testing.T) {\n\t// ...\n}", "..."]
}`)
	return b.String()
}

type suiteJSON struct {
	Visible []string `json:"visible"`
	Hidden  []string `json:"hidden"`
}

func parseRespText(text string) (visible, hidden []string, err error) {
	t := strings.TrimSpace(text)
	t = strings.TrimPrefix(t, "```json")
	t = strings.TrimPrefix(t, "```")
	t = strings.TrimSuffix(t, "```")
	t = strings.TrimSpace(t)

	ini := strings.Index(t, "{")
	fin := strings.LastIndex(t, "}")
	if ini == -1 || fin <= ini {
		return nil, nil, fmt.Errorf("no JSON in response")
	}
	var s suiteJSON
	if err := json.Unmarshal([]byte(t[ini:fin+1]), &s); err != nil {
		return nil, nil, err
	}
	return s.Visible, s.Hidden, nil
}

func buildGoFile(pkg string, funcs []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "package %s_test\n\nimport \"testing\"\n\n", pkg)
	for _, f := range funcs {
		b.WriteString(f)
		if !strings.HasSuffix(strings.TrimSpace(f), "}") {
			b.WriteString("\n}")
		}
		b.WriteString("\n\n")
	}
	return b.String()
}

// inferDirPkg returns the absolute worktree directory and the Go package
// name inferred from tocar_solo[0]. Falls back to roomPath and "main".
func inferDirPkg(tocarSolo []string, roomPath string) (dir, pkg string) {
	if len(tocarSolo) == 0 {
		return roomPath, "main"
	}
	rel := strings.TrimRight(tocarSolo[0], "/*")
	if rel == "" {
		return roomPath, "main"
	}
	parts := strings.Split(rel, "/")
	pkg = parts[len(parts)-1]
	if pkg == "" || pkg == "." {
		pkg = "main"
	}
	return filepath.Join(roomPath, rel), pkg
}

func toRelDir(tocarSolo []string) string {
	if len(tocarSolo) == 0 {
		return "."
	}
	rel := strings.TrimRight(tocarSolo[0], "/*")
	if rel == "" {
		return "."
	}
	return rel
}

func toRelPath(p string) string { return filepath.ToSlash(p) }
