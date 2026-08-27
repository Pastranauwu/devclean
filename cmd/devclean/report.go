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

	historial, err := metrics.LeerHistorial(root)
	if err != nil {
		return err
	}
	var prev *metrics.Metricas
	if n := len(historial); n > 0 {
		prev = &historial[n-1].Metricas
	}
	tendencia := metrics.Comparar(prev, m)

	if err := metrics.GuardarHistorial(root, m); err != nil {
		return err
	}

	if err := out.Data(metrics.Reporte{Metricas: m, Tendencia: tendencia}); err != nil {
		return err
	}
	out.Line("intentos hasta verde  %s %.1f   (meta ≤ 2)", tendencia.IntentosHastaVerde, m.IntentosHastaVerde)
	out.Line("ruido                 %s %.1f%%  (meta < 5%%)", tendencia.Ruido, m.Ruido)
	out.Line("roce                  %s %.1f por 10 entregas  (meta < 1)", tendencia.Roce, m.Roce)
	if m.Friccion != nil {
		out.Line("fricción              %s %.0f min", tendencia.Friccion, *m.Friccion)
	} else {
		out.Line("fricción              %s —  sin datos", tendencia.Friccion)
	}
	out.Line("rechazo en entrada    %s %.1f%%", tendencia.RechazoEntrada, m.RechazoEntrada)
	out.Line("tokens                %d", m.Tokens)
	return nil
}
