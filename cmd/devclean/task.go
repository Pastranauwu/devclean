package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Pastranauwu/devclean/internal/config"
	"github.com/Pastranauwu/devclean/internal/examiner"
	"github.com/Pastranauwu/devclean/internal/sealed"
	"github.com/Pastranauwu/devclean/internal/state"
	"github.com/Pastranauwu/devclean/internal/task"
)

func newTaskCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "task",
		Short: "manejo manual de tareas",
	}
	cmd.AddCommand(
		newTaskAddCmd(),
		newTaskEditCmd(),
		newTaskRmCmd(),
		newTaskListCmd(),
		newTaskCheckCmd(),
		newTaskSealCmd(),
	)
	return cmd
}

// projectRoot resuelve la raíz del repo y exige que .devclean exista.
func projectRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	root, err := config.RepoRoot(cwd)
	if err != nil {
		return "", err
	}
	if !config.Exists(root) {
		return "", errors.New("sin configuración · corre devclean init primero")
	}
	return root, nil
}

func validTaskID(id string) error {
	if !task.ValidID(id) {
		return fmt.Errorf("id inválido: %s · usa el formato T-001", id)
	}
	return nil
}

func newTaskAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   `add "<título>"`,
		Short: "crea una tarea nueva con id correlativo",
		Example: `  devclean task add "exportar clientes a CSV"
  devclean task add "login acepta tildes con soporte de ñ"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			titulo := strings.TrimSpace(args[0])
			if titulo == "" {
				return errors.New("falta el título · devclean task add \"lo que necesitas\"")
			}
			root, err := projectRoot()
			if err != nil {
				return err
			}
			return runTaskAdd(root, titulo)
		},
	}
}

func runTaskAdd(root, titulo string) error {
	dir := config.TasksDir(root)
	id, err := task.NextID(dir)
	if err != nil {
		return err
	}
	stack := config.DetectLanguage(root)
	t := task.Task{
		Version:        task.Version,
		ID:             id,
		Titulo:         titulo,
		TocarSolo:      []string{},
		NoTocar:        []string{},
		LimiteIntentos: task.DefaultLimiteIntentos,
		LimiteLineas:   task.DefaultLimiteLineas,
		Notas:          notasListoCuando(stack),
	}
	if err := task.Save(dir, t); err != nil {
		return err
	}
	if err := out.Data(t); err != nil {
		return err
	}
	out.Line("✓ %s creada · completa listo_cuando con devclean task edit %s", id, id)
	imprimirPlantillasListoCuando(stack)
	return nil
}

func newTaskSealCmd() *cobra.Command {
	var visible, oculta string
	var forzar bool
	cmd := &cobra.Command{
		Use:   "seal <id>",
		Short: "sella una suite oculta escrita a mano, sin gastar modelo",
		Long: `Sella pruebas que escribiste vos —o que generó un modelo caro corriendo
aparte, fuera del bucle del implementador— en vez de dejárselas al
examinador ciego.

La suite --visible se copia al cuarto de la tarea cuando corra: el
implementador la ve y es su criterio de aceptación. La suite --oculta se
sella con hash y solo corre en la esclusa de salida (devclean ship), que
no distingue si la escribió un humano o el examinador.

Los archivos se leen del disco al sellar, así que después podés borrarlos:
lo sellado ya no depende de ellos.`,
		Example: `  devclean task seal T-001 --visible pruebas/visible_test.go --oculta pruebas/oculta_test.go
  devclean task seal T-001 --visible v.py --oculta o.py --forzar`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			if err := validTaskID(id); err != nil {
				return err
			}
			root, err := projectRoot()
			if err != nil {
				return err
			}
			return runTaskSeal(root, id, visible, oculta, forzar)
		},
	}
	cmd.Flags().StringVar(&visible, "visible", "", "archivo con las pruebas que el implementador SÍ ve")
	cmd.Flags().StringVar(&oculta, "oculta", "", "archivo con las pruebas que se sellan y solo corren en ship")
	cmd.Flags().BoolVar(&forzar, "forzar", false, "reemplaza una suite ya sellada")
	_ = cmd.MarkFlagRequired("visible")
	_ = cmd.MarkFlagRequired("oculta")
	return cmd
}

// runTaskSeal sella una suite escrita a mano. Las rutas dentro del cuarto
// salen de examiner.RutasSuite, el mismo camino que usa el examinador
// automático: de acá para abajo nadie distingue el origen.
func runTaskSeal(root, id, visiblePath, ocultaPath string, forzar bool) error {
	t, err := task.Load(config.TasksDir(root), id)
	if err != nil {
		return fmt.Errorf("no existe la tarea %s · créala con devclean task add", id)
	}
	if sealed.Exists(root, id) && !forzar {
		return fmt.Errorf("%s ya tiene una suite sellada · usa --forzar para reemplazarla", id)
	}

	visible, err := leerSuite(root, visiblePath)
	if err != nil {
		return err
	}
	oculta, err := leerSuite(root, ocultaPath)
	if err != nil {
		return err
	}

	rutaVisible, rutaOculta := examiner.RutasSuite(t.TocarSolo, config.DetectLanguage(root))
	s := sealed.SuiteOculta{
		Content:        oculta,
		Archivo:        rutaOculta,
		Visible:        visible,
		ArchivoVisible: rutaVisible,
	}
	if err := sealed.Write(root, id, s); err != nil {
		return fmt.Errorf("no se pudo sellar la suite de %s · %s", id, err)
	}

	if err := out.Data(map[string]string{
		"id":      id,
		"visible": rutaVisible,
		"oculta":  rutaOculta,
	}); err != nil {
		return err
	}
	out.Line("✓ %s sellada a mano · visible → %s en el cuarto", id, rutaVisible)
	out.Line("  la oculta se verifica en devclean ship y se quema al usarse")
	return nil
}

// leerSuite lee un archivo de pruebas del disco. Las rutas relativas se
// resuelven contra la raíz del repo, no contra el cuarto: sellar pasa
// antes de que el cuarto exista.
func leerSuite(root, p string) (string, error) {
	abs := p
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(root, p)
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return "", fmt.Errorf("no se pudo leer %s · %s", p, err)
	}
	if strings.TrimSpace(string(data)) == "" {
		return "", fmt.Errorf("%s está vacío · una suite sin pruebas no verifica nada", p)
	}
	return string(data), nil
}

func newTaskEditCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "edit <id>",
		Short: "abre la tarea en $EDITOR",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			if err := validTaskID(id); err != nil {
				return err
			}
			root, err := projectRoot()
			if err != nil {
				return err
			}
			path := task.Path(config.TasksDir(root), id)
			if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("no existe la tarea %s", id)
			}

			editor := strings.Fields(os.Getenv("EDITOR"))
			if len(editor) == 0 {
				editor = []string{"vi"}
			}
			edit := exec.Command(editor[0], append(editor[1:], path)...)
			edit.Stdin = os.Stdin
			edit.Stdout = os.Stdout
			edit.Stderr = os.Stderr
			if err := edit.Run(); err != nil {
				return fmt.Errorf("el editor falló: %s", err)
			}

			t, err := task.Load(config.TasksDir(root), id)
			if err != nil {
				return fmt.Errorf("%s quedó mal formada · %s · reábrela con devclean task edit %s", id, err, id)
			}
			if errs := t.Validate(); len(errs) > 0 {
				msgs := make([]string, len(errs))
				for i, e := range errs {
					msgs[i] = e.Error()
				}
				return fmt.Errorf("%s incompleta · %s", id, strings.Join(msgs, " · "))
			}
			if err := out.Data(t); err != nil {
				return err
			}
			out.Line("✓ %s guardada y válida", id)
			return nil
		},
	}
}

func newTaskRmCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm <id>",
		Short: "elimina una tarea",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			if err := validTaskID(id); err != nil {
				return err
			}
			root, err := projectRoot()
			if err != nil {
				return err
			}
			if err := task.Remove(config.TasksDir(root), id); err != nil {
				return err
			}
			if err := state.Remove(root, id); err != nil {
				return err
			}
			if err := out.Data(map[string]string{"eliminada": id}); err != nil {
				return err
			}
			out.Line("✓ %s eliminada", id)
			return nil
		},
	}
}

type listEntry struct {
	ID          string   `json:"id"`
	Titulo      string   `json:"titulo"`
	ListoCuando string   `json:"listo_cuando"`
	Estado      string   `json:"estado"`
	Problemas   []string `json:"problemas,omitempty"`
}

func newTaskListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "lista las tareas",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := projectRoot()
			if err != nil {
				return err
			}
			tasks, err := task.List(config.TasksDir(root))
			if err != nil {
				return err
			}

			entries := make([]listEntry, 0, len(tasks))
			for _, t := range tasks {
				entry := listEntry{
					ID:          t.ID,
					Titulo:      t.Titulo,
					ListoCuando: t.ListoCuando,
					Estado:      "válida",
				}
				if errs := t.Validate(); len(errs) > 0 {
					entry.Estado = "incompleta"
					for _, e := range errs {
						entry.Problemas = append(entry.Problemas, e.Error())
					}
				}
				entries = append(entries, entry)
			}

			if err := out.Data(entries); err != nil {
				return err
			}
			if len(entries) == 0 {
				out.Line("sin tareas · empieza con devclean task add \"lo que necesitas\"")
				return nil
			}
			for _, e := range entries {
				if e.Estado == "válida" {
					out.Line("✓ %s  %s", e.ID, e.Titulo)
				} else {
					out.Line("· %s  %s · incompleta: %s", e.ID, e.Titulo, strings.Join(e.Problemas, " · "))
				}
			}
			return nil
		},
	}
}
