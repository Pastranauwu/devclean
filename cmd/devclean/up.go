package main

import (
	"github.com/spf13/cobra"

	"github.com/Pastranauwu/devclean/internal/spec"
)

func newUpCmd() *cobra.Command {
	var file string
	var agentes int
	var modelo, ejecutor string

	cmd := &cobra.Command{
		Use:   "up",
		Short: "aplica la especificación y ejecuta todas las tareas en paralelo (estilo compose)",
		Long: `Si existe un archivo de especificación (devclean.spec.yml), lo aplica
sincronizando las tareas, y a continuación ejecuta todas las tareas
pendientes en paralelo en cuartos aislados.`,
		Example: `  devclean up
  devclean up -f specs/auth.yml
  devclean up --agentes 4`,
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := projectRoot()
			if err != nil {
				return err
			}

			// Intentar aplicar la spec si existe
			specFile := file
			if specFile == "" {
				if found, err := spec.Find(root); err == nil {
					specFile = found
				}
			}
			if specFile != "" {
				if err := runApply(root, specFile, false, false); err != nil {
					return err
				}
				out.Line("")
			}

			if agentes < 1 {
				agentes = 1
			}

			return runCmd(agentes, ejecutor, modelo)
		},
	}

	cmd.Flags().StringVarP(&file, "file", "f", "", "ruta al archivo de especificación (.yml)")
	cmd.Flags().IntVar(&agentes, "agentes", 1, "número de trabajadores en paralelo (por defecto, 1)")
	cmd.Flags().StringVar(&modelo, "modelo", "", "fuerza un modelo para todas las tareas")
	cmd.Flags().StringVar(&ejecutor, "ejecutor", "", "opencode o claude")

	return cmd
}
