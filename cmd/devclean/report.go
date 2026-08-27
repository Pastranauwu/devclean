package main

import (
	"github.com/spf13/cobra"

	"github.com/Pastranauwu/devclean/internal/metrics"
)

func newReportCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "report",
		Short: "métricas del proyecto",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runReport()
		},
	}
}

func runReport() error {
	root, err := projectRoot()
	if err != nil {
		return err
	}
	d, err := metrics.Recoger(root)
	if err != nil {
		return err
	}
	m := metrics.Calcular(d)

	if err := out.Data(m); err != nil {
		return err
	}
	out.Line("intentos hasta verde  %.1f   (meta ≤ 2)", m.IntentosHastaVerde)
	out.Line("ruido                 %.1f%%  (meta < 5%%)", m.Ruido)
	out.Line("roce                  %.1f por 10 entregas  (meta < 1)", m.Roce)
	if m.Friccion != nil {
		out.Line("fricción              %.0f min", *m.Friccion)
	} else {
		out.Line("fricción              —  sin datos")
	}
	out.Line("rechazo en entrada    %.1f%%", m.RechazoEntrada)
	out.Line("tokens                %d", m.Tokens)
	return nil
}
