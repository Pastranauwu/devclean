package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Pastranauwu/devclean/internal/config"
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
