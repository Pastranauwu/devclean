package main

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/Pastranauwu/devclean/internal/ui"
)

// out is the shared printer for all commands, configured in the root
// PersistentPreRun from --plain/--json and the nature of stdout.
var out *ui.Printer

func newRootCmd() *cobra.Command {
	var jsonOut bool
	var plain bool

	root := &cobra.Command{
		Use:           "devclean",
		Short:         "dirige agentes de IA y entrega solo código limpio",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if !jsonOut && !plain && !isTerminal(os.Stdout) {
				plain = true
			}
			out = ui.New(os.Stdout, jsonOut)
		},
	}

	root.PersistentFlags().BoolVar(&plain, "plain", false, "salida en texto plano, una línea por evento")
	root.PersistentFlags().BoolVar(&jsonOut, "json", false, "salida estructurada en JSON")

	return root
}

func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}
