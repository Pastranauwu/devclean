// Package historial archiva cada feature terminado como una nota corta
// tipo ADR: qué se pidió (el spec tal cual), por qué (motivo) y con qué
// resultado. Los commits dicen qué cambió; esto dice con qué intención.
package historial

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/Pastranauwu/devclean/internal/config"
	"github.com/Pastranauwu/devclean/internal/loop"
	"github.com/Pastranauwu/devclean/internal/ship"
	"github.com/Pastranauwu/devclean/internal/spec"
	"github.com/Pastranauwu/devclean/internal/state"
	"github.com/Pastranauwu/devclean/internal/task"
)

// Dir es la carpeta del historial; Index, la tabla con una línea por
// feature archivado.
func Dir(root string) string   { return filepath.Join(root, ".devclean", "historial") }
func Index(root string) string { return filepath.Join(root, ".devclean", "index.md") }

const cabeceraIndex = "# Historial de features\n\n| fecha | id | feature | motivo | estado |\n|---|---|---|---|---|\n"

// Resultado es resultado.yml: lo que quedó probado y cómo se llegó.
type Resultado struct {
	Fecha     string     `yaml:"fecha"`
	Rama      string     `yaml:"rama"`
	Commit    string     `yaml:"commit"`
	Estado    string     `yaml:"estado"`
	Criterios []Criterio `yaml:"criterios"`
	Tareas    []Tarea    `yaml:"tareas"`
}

// Criterio es una aceptación del spec. Las que no traen comando no son
// compuerta: se anotan como "sin comando", no como aprobadas.
type Criterio struct {
	Criterio  string `yaml:"criterio,omitempty"`
	Comando   string `yaml:"comando,omitempty"`
	Resultado string `yaml:"resultado"`
}

// Tarea es una tarea en que se dividió el trabajo. Los intentos son de
// las tareas: un comando de aceptación corre una sola vez.
type Tarea struct {
	ID       string `yaml:"id"`
	Titulo   string `yaml:"titulo"`
	Estado   string `yaml:"estado"`
	Intentos int    `yaml:"intentos"`
}

// Entrada es lo que Archivar dejó escrito.
type Entrada struct {
	Nombre string // NNNN-slug
	Dir    string
}

// Archivar guarda el spec en specPath como la siguiente entrada del
// historial y lo vacía, en un commit sobre la rama actual. Solo archiva
// lo probado: exige que la última entrega conjunta (`ship --todas`) haya
// quedado aprobada con todos sus comandos de aceptación en verde.
func Archivar(root, specPath string) (Entrada, error) {
	crudo, err := os.ReadFile(specPath)
	if err != nil || strings.TrimSpace(string(crudo)) == "" {
		return Entrada{}, errors.New("spec vacío · nada que archivar")
	}
	s, err := spec.Parse(crudo)
	if err != nil {
		return Entrada{}, fmt.Errorf("spec ilegible · %w", err)
	}
	a, hay := ship.LeerAceptacion(root)
	if !hay {
		return Entrada{}, errors.New("sin entrega conjunta registrada · corre devclean ship --todas y archiva cuando pase")
	}
	if !a.Aprobado {
		return Entrada{}, fmt.Errorf("la última entrega no pasó · %s · arregla y corre devclean ship --todas", a.Motivo)
	}
	for _, c := range a.Criterios {
		if !c.Corrio || !c.Paso {
			return Entrada{}, fmt.Errorf("criterio de aceptación sin pasar · %s · arregla y corre devclean ship --todas", c.Comando)
		}
	}

	n, err := siguiente(root)
	if err != nil {
		return Entrada{}, err
	}
	e := Entrada{Nombre: fmt.Sprintf("%04d-%s", n, slug(s.Feature))}
	e.Dir = filepath.Join(Dir(root), e.Nombre)
	if err := os.MkdirAll(e.Dir, 0o755); err != nil {
		return Entrada{}, err
	}

	fecha := a.Fecha.Format("2006-01-02")
	res := Resultado{Fecha: fecha, Rama: a.Rama, Commit: a.Commit, Estado: "probado", Criterios: criterios(s, a), Tareas: tareas(root, a.Tareas)}
	y, err := yaml.Marshal(res)
	if err != nil {
		return Entrada{}, err
	}
	if err := os.WriteFile(filepath.Join(e.Dir, "spec.yml"), crudo, 0o644); err != nil {
		return Entrada{}, err
	}
	if err := os.WriteFile(filepath.Join(e.Dir, "resultado.yml"), y, 0o644); err != nil {
		return Entrada{}, err
	}
	if err := agregarAlIndex(root, fmt.Sprintf("| %s | %04d | %s | %s | probado |\n", fecha, n, celda(s.Feature), celda(s.Motivo))); err != nil {
		return Entrada{}, err
	}

	// el spec queda en blanco para el siguiente feature. La intención y la
	// aceptación guardadas eran de ESTE: si quedaran, el próximo spec se
	// compararía contra el anterior y el arquitecto recibiría los
	// requerimientos viejos como "quitados", con la orden de borrar su
	// código; y una aceptación vieja en verde dejaría archivar sin probar.
	if err := os.WriteFile(specPath, nil, 0o644); err != nil {
		return Entrada{}, err
	}
	rutas := []string{e.Dir, Index(root), specPath}
	if cambio, err := abrirGitignore(root); err != nil {
		return Entrada{}, err
	} else if cambio {
		rutas = append(rutas, filepath.Join(root, ".gitignore"))
	}
	for _, f := range []string{spec.IntencionFile, spec.FeatureStateFile, ship.AceptacionFile} {
		p := filepath.Join(root, ".devclean", f)
		versionado := git(root, "ls-files", "--error-unmatch", p) == nil
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			return Entrada{}, err
		}
		if versionado {
			rutas = append(rutas, p)
		}
	}
	if err := git(root, append([]string{"add", "-A", "--"}, rutas...)...); err != nil {
		return Entrada{}, fmt.Errorf("archivado en %s pero git no pudo agregarlo · %w", e.Dir, err)
	}
	if err := git(root, append([]string{"commit", "-q", "-m", "devclean: archiva " + e.Nombre, "--"}, rutas...)...); err != nil {
		return Entrada{}, fmt.Errorf("archivado en %s pero el commit falló · %w", e.Dir, err)
	}
	return e, nil
}

// abrirGitignore deja pasar el historial en un proyecto que ignora
// `.devclean/` entero (soundlike): con la carpeta ignorada git no mira
// adentro y ninguna excepción sirve, así que la línea pasa a ignorar su
// contenido (`.devclean/*`) menos el historial y el índice. Todo lo demás
// de .devclean sigue ignorado como estaba. Reporta si cambió el archivo.
func abrirGitignore(root string) (bool, error) {
	p := filepath.Join(root, ".gitignore")
	b, err := os.ReadFile(p)
	if err != nil {
		return false, nil // sin .gitignore no hay nada que abrir
	}
	lineas := strings.Split(string(b), "\n")
	cambio := false
	for i, l := range lineas {
		if strings.Trim(strings.TrimSpace(l), "/") == ".devclean" {
			lineas[i] = ".devclean/*\n!.devclean/historial/\n!.devclean/index.md"
			cambio = true
		}
	}
	if !cambio {
		return false, nil
	}
	return true, os.WriteFile(p, []byte(strings.Join(lineas, "\n")), 0o644)
}

// criterios junta los comandos que corrieron en la entrega (los del spec
// y los que agrega el esqueleto) con las aceptaciones textuales del spec.
func criterios(s spec.Spec, a ship.Aceptacion) []Criterio {
	nombre := map[string]string{}
	var out []Criterio
	for _, ac := range s.Acceptance {
		if ac.Command == "" {
			out = append(out, Criterio{Criterio: ac.Criterion, Resultado: "sin comando"})
		} else {
			nombre[ac.Command] = ac.Criterion
		}
	}
	for _, c := range a.Criterios {
		out = append(out, Criterio{Criterio: nombre[c.Comando], Comando: c.Comando, Resultado: "pasó"})
	}
	return out
}

func tareas(root string, ids []string) []Tarea {
	var out []Tarea
	for _, id := range ids {
		t, err := task.Load(config.TasksDir(root), id)
		if err != nil {
			continue
		}
		st, _ := state.Get(root, id)
		intentos, _ := loop.ReadAttempts(root, id)
		out = append(out, Tarea{ID: id, Titulo: t.Titulo, Estado: st.Estado, Intentos: len(intentos)})
	}
	return out
}

// siguiente es el NNNN de la próxima entrada: el mayor que hay más uno.
// Las entradas no se borran, pero si faltara una no se reusa su número.
func siguiente(root string) (int, error) {
	es, err := os.ReadDir(Dir(root))
	if err != nil && !os.IsNotExist(err) {
		return 0, err
	}
	max := 0
	for _, e := range es {
		if num, _, ok := strings.Cut(e.Name(), "-"); ok && e.IsDir() {
			if n, err := strconv.Atoi(num); err == nil && n > max {
				max = n
			}
		}
	}
	if max >= 9999 {
		return 0, errors.New("el historial llegó a 9999 entradas")
	}
	return max + 1, nil
}

var noSlug = regexp.MustCompile(`[^a-z0-9]+`)

// slug deja el nombre del feature apto para una carpeta.
func slug(feature string) string {
	s := strings.NewReplacer("á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ü", "u", "ñ", "n").Replace(strings.ToLower(feature))
	s = strings.Trim(noSlug.ReplaceAllString(s, "-"), "-")
	if len(s) > 50 {
		s = strings.Trim(s[:50], "-")
	}
	if s == "" {
		return "feature"
	}
	return s
}

// celda deja un texto en una sola celda de tabla markdown.
func celda(s string) string {
	return strings.ReplaceAll(strings.Join(strings.Fields(s), " "), "|", "\\|")
}

func agregarAlIndex(root, linea string) error {
	p := Index(root)
	if _, err := os.Stat(p); os.IsNotExist(err) {
		linea = cabeceraIndex + linea
	}
	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(linea)
	return err
}

func git(root string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return nil
}
