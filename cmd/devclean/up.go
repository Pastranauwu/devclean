package main

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/Pastranauwu/devclean/internal/spec"
)

func newUpCmd() *cobra.Command {
	var file string
	var agentes int
	var modelo, ejecutor, titulo string
	var reintentar, entregar, integrar bool

	cmd := &cobra.Command{
		Use:   `up ["<petición>"]`,
		Short: "de una petición a un PR limpio, sin pasos intermedios",
		Long: `Encadena todo el trabajo de devclean en un solo comando.

Con una petición en lenguaje natural, la parte en tareas (devclean plan),
las ejecuta en paralelo en cuartos aislados (devclean run) y, con --ship,
las entrega en UN pull request limpio (devclean ship --todas).

Sin petición, aplica la especificación del repo (devclean.spec.yml) si
existe y ejecuta las tareas pendientes.`,
		Example: `  devclean up "cli en go que despierta equipos por wake-on-lan" --ship
  devclean up "arreglar el login con tildes" --integrar
  devclean up --agentes 4 --ship
  devclean up -f specs/auth.yml`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := projectRoot()
			if err != nil {
				return err
			}

			if frase := strings.TrimSpace(strings.Join(args, " ")); frase != "" {
				// una petición manda sobre la spec: es lo que el humano
				// acaba de pedir, aquí y ahora
				if err := runPlan(frase, modelo, ejecutor, "", true); err != nil {
					return err
				}
				out.Line("")
				if titulo == "" {
					titulo = frase
				}
			} else {
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
			}

			if agentes < 1 {
				agentes = 1
			}
			if err := runCmd(agentes, ejecutor, modelo, reintentar); err != nil {
				return err
			}
			if !entregar && !integrar {
				return nil
			}
			out.Line("")
			return runShipTodas(false, titulo, integrar)
		},
	}

	cmd.Flags().StringVarP(&file, "file", "f", "", "ruta al archivo de especificación (.yml)")
	cmd.Flags().IntVar(&agentes, "agentes", 1, "número de trabajadores en paralelo (por defecto, 1)")
	cmd.Flags().StringVar(&modelo, "modelo", "", "fuerza un modelo para todas las tareas")
	cmd.Flags().StringVar(&ejecutor, "ejecutor", "", "opencode o claude")
	cmd.Flags().BoolVar(&reintentar, "reintentar", false, "vuelve a correr también las tareas detenidas")
	cmd.Flags().BoolVar(&entregar, "ship", false, "al terminar, entrega todas las tareas listas en un solo PR")
	cmd.Flags().BoolVar(&integrar, "integrar", false, "además de entregar, revisa el diff con un modelo y mergea el PR si aprueba")
	cmd.Flags().StringVar(&titulo, "titulo", "", "título del PR (por defecto, la petición)")

	return cmd
}
