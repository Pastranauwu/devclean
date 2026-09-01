package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Pastranauwu/devclean/internal/config"
	"github.com/Pastranauwu/devclean/internal/spec"
	"github.com/Pastranauwu/devclean/internal/tui"
)

func newApplyCmd() *cobra.Command {
	var file string
	var runImmediately bool
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "apply [-f archivo.spec.yml]",
		Short: "aplica una especificación declarativa de tareas (Requerimientos como Código)",
		Long: `Lee una especificación declarativa (devclean.spec.yml por defecto), valida
los contratos de las tareas, asigna IDs correlativos a las que no tengan,
y las guarda en .devclean/tasks/ listas para ser verificadas o ejecutadas.

Con --run (o -r), lanza inmediatamente la ejecución en paralelo (devclean run).`,
		Example: `  devclean apply
  devclean apply -f specs/auth.yml
  devclean apply --dry-run
  devclean apply --run`,
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := projectRoot()
			if err != nil {
				return err
			}
			return runApply(root, file, runImmediately, dryRun)
		},
	}

	cmd.Flags().StringVarP(&file, "file", "f", "", "ruta al archivo de especificación (.yml)")
	cmd.Flags().BoolVarP(&runImmediately, "run", "r", false, "ejecuta las tareas inmediatamente tras aplicarlas")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "valida la especificación sin escribir archivos en disco")

	return cmd
}

func runApply(root, filePath string, runImmediately, dryRun bool) error {
	var err error
	if filePath == "" {
		filePath, err = spec.Find(root)
		if err != nil {
			return fmt.Errorf("no se encontró archivo de especificación en %s · especifícalo con -f o crea devclean.spec.yml", root)
		}
	} else if !filepath.IsAbs(filePath) {
		filePath = filepath.Join(root, filePath)
	}

	s, err := spec.Load(filePath)
	if err != nil {
		return fmt.Errorf("error al leer %s: %w", filePath, err)
	}

	tasksDir := config.TasksDir(root)
	applied, err := spec.Apply(tasksDir, s, dryRun)
	if err != nil {
		return err
	}

	if dryRun {
		out.Line("✓ especificación válida (%d tareas en %s, modo --dry-run):", len(applied), filepath.Base(filePath))
		for _, t := range applied {
			ag := ""
			if t.Agente != "" {
				ag = " [" + t.Agente + "]"
			}
			out.Line("  %s  %s%s  · listo cuando: %s", t.ID, t.Titulo, ag, t.ListoCuando)
		}
		return nil
	}

	if err := out.Data(applied); err != nil {
		return err
	}

	if esTUI() {
		var cuerpo strings.Builder
		titulo := fmt.Sprintf("APLICADAS %d TAREAS", len(applied))
		if s.Feature != "" {
			titulo += " (" + s.Feature + ")"
		}
		cuerpo.WriteString(tui.Titulo(titulo) + "\n\n")
		for _, t := range applied {
			ag := ""
			if t.Agente != "" {
				ag = " [" + t.Agente + "]"
			}
			cuerpo.WriteString(t.ID + "  " + t.Titulo + ag + "  " + tui.Apagado("· listo cuando: "+t.ListoCuando) + "\n")
		}
		out.Line("%s", tui.Caja(strings.TrimRight(cuerpo.String(), "\n")))
	} else {
		out.Line("✓ %d tareas aplicadas desde %s:", len(applied), filepath.Base(filePath))
		for _, t := range applied {
			ag := ""
			if t.Agente != "" {
				ag = " [" + t.Agente + "]"
			}
			out.Line("  %s  %s%s  · listo cuando: %s", t.ID, t.Titulo, ag, t.ListoCuando)
		}
	}

	if runImmediately {
		out.Line("")
		return runCmd(1, "", "", false)
	}

	out.Line("\n· corre devclean run (o devclean up) para ejecutarlas en paralelo")
	return nil
}
