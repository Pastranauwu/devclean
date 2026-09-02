package main

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Pastranauwu/devclean/internal/budget"
	"github.com/Pastranauwu/devclean/internal/config"
	"github.com/Pastranauwu/devclean/internal/ventanas"
)

// usoClaude es la salida JSON de la utilización real de la cuenta.
type usoClaude struct {
	Accesible bool   `json:"accesible"`
	CincoH    *int   `json:"5h,omitempty"`
	Semanal   *int   `json:"semanal,omitempty"`
	Motivo    string `json:"motivo,omitempty"`
}

func newUsageCmd() *cobra.Command {
	var sonda bool
	cmd := &cobra.Command{
		Use:   "usage",
		Short: "gasto por ventanas (5h, semanal, mensual) y presupuesto",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUsage(sonda)
		},
	}
	cmd.Flags().BoolVar(&sonda, "sonda", false, "fuerza la consulta en vivo de la utilización real de Claude (ignora la caché)")
	return cmd
}

func runUsage(sonda bool) error {
	root, err := projectRoot()
	if err != nil {
		return err
	}
	cfg, err := config.Load(root)
	if err != nil {
		return err
	}

	var lineas []string

	if cfg.PresupuestoTokens > 0 {
		usado := budget.GastoEnDisco(root)
		lineas = append(lineas, "presupuesto absoluto "+budget.Barra(usado, cfg.PresupuestoTokens))
	}

	registro := ventanas.Nuevo(ventanas.LedgerPath(), cfg.PresupuestoVentanas)
	proveedores := map[string]bool{"claude": true, "opencode": true}
	for p := range cfg.PresupuestoVentanas {
		proveedores[p] = true
	}
	var nombres []string
	for p := range proveedores {
		nombres = append(nombres, p)
	}
	sort.Strings(nombres)
	for _, p := range nombres {
		if l := ventanas.LineaVentanas(registro, p); l != "" {
			lineas = append(lineas, "gasto "+l)
		}
	}

	if err := out.Data(lineas); err != nil {
		return err
	}

	for _, l := range lineas {
		out.Line("%s", l)
	}

	// utilización real de la cuenta Claude, si la sonda puede leerla
	uso := ventanas.SondaCached(context.Background(), ventanas.KeyClaude(cfg.KeyEnvDe("claude")), sonda)
	if err := out.Data(usoClaude{Accesible: uso.Accesible, CincoH: uso.CincoH, Semanal: uso.Semanal}); err != nil {
		return err
	}
	if uso.Accesible {
		var partes []string
		if uso.CincoH != nil {
			partes = append(partes, fmt.Sprintf("5h %d%%", *uso.CincoH))
		}
		if uso.Semanal != nil {
			partes = append(partes, fmt.Sprintf("semanal %d%%", *uso.Semanal))
		}
		out.Line("cuenta real (claude) · %s", strings.Join(partes, " · "))
	} else {
		out.Line("cuenta real (claude) · no accesible · devclean mide su propio gasto en las ventanas; la cuenta la leés en claude.ai o con una key en ANTHROPIC_API_KEY")
	}
	return nil
}
