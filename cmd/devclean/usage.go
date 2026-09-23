package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Pastranauwu/devclean/internal/budget"
	"github.com/Pastranauwu/devclean/internal/config"
	"github.com/Pastranauwu/devclean/internal/loop"
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

	if err := mostrarGastoPorTarea(root); err != nil {
		return err
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

// gastoTarea es el gasto de una tarea con un modelo, sumado de sus
// intentos. La entrada sin caché es una fracción mínima del prompt real:
// sin las dos columnas de caché el consumo parece casi nulo.
//
// PrimerTurno es el contexto base del primer intento: lo que el CLI carga
// (prompt de sistema, herramientas, skills, MCP) más el prompt de devclean,
// antes de que el agente haga nada.
type gastoTarea struct {
	Tarea              string  `json:"tarea"`
	Modelo             string  `json:"modelo"`
	Intentos           int     `json:"intentos"`
	Turnos             int     `json:"turnos"`
	PrimerTurno        int     `json:"primer_turno"`
	PrimerTurnoEscrita int     `json:"primer_turno_escrita"`
	Entrada            int     `json:"entrada"`
	Salida             int     `json:"salida"`
	CacheLeida         int     `json:"cache_leida"`
	CacheEscrita       int     `json:"cache_escrita"`
	CostoUSD           float64 `json:"costo_usd"`
}

// mostrarGastoPorTarea lee .devclean/runs/*/attempts.jsonl y muestra el
// gasto por tarea y por modelo. Los intentos anteriores a la medición de
// turnos y caché salen con esas columnas en cero.
func mostrarGastoPorTarea(root string) error {
	dirs, err := os.ReadDir(loop.RunsDir(root))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	var filas []gastoTarea
	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}
		as, err := loop.ReadAttempts(root, d.Name())
		if err != nil {
			return err
		}
		por := map[string]*gastoTarea{}
		var modelos []string
		for _, a := range as {
			g := por[a.Modelo]
			if g == nil {
				g = &gastoTarea{Tarea: d.Name(), Modelo: a.Modelo, PrimerTurno: a.Tokens.PrimerTurno, PrimerTurnoEscrita: a.Tokens.PrimerTurnoEscrita}
				por[a.Modelo] = g
				modelos = append(modelos, a.Modelo)
			}
			g.Intentos++
			g.Turnos += a.Tokens.Turnos
			g.Entrada += a.Tokens.Entrada
			g.Salida += a.Tokens.Salida
			g.CacheLeida += a.Tokens.CacheLeida
			g.CacheEscrita += a.Tokens.CacheEscrita
			g.CostoUSD += a.Tokens.CostoUSD
		}
		sort.Strings(modelos)
		for _, m := range modelos {
			filas = append(filas, *por[m])
		}
	}
	if err := out.Data(filas); err != nil {
		return err
	}
	if len(filas) == 0 {
		return nil
	}
	const formato = "%-8s %-18s %4d %6d %9d %9d %9d %9d %11d %11d %8.3f"
	out.Line("")
	out.Line("%-8s %-18s %4s %6s %9s %9s %9s %9s %11s %11s %8s", "tarea", "modelo", "int", "turnos", "base 1er", "base escr", "entrada", "salida", "caché leída", "caché escr", "usd")
	var total gastoTarea
	for _, f := range filas {
		modelo := f.Modelo
		if modelo == "" {
			modelo = "(por defecto)"
		}
		out.Line(formato, f.Tarea, modelo, f.Intentos, f.Turnos, f.PrimerTurno, f.PrimerTurnoEscrita, f.Entrada, f.Salida, f.CacheLeida, f.CacheEscrita, f.CostoUSD)
		total.Intentos += f.Intentos
		total.Turnos += f.Turnos
		total.PrimerTurno += f.PrimerTurno
		total.PrimerTurnoEscrita += f.PrimerTurnoEscrita
		total.Entrada += f.Entrada
		total.Salida += f.Salida
		total.CacheLeida += f.CacheLeida
		total.CacheEscrita += f.CacheEscrita
		total.CostoUSD += f.CostoUSD
	}
	out.Line(formato, "total", "", total.Intentos, total.Turnos, total.PrimerTurno, total.PrimerTurnoEscrita, total.Entrada, total.Salida, total.CacheLeida, total.CacheEscrita, total.CostoUSD)
	return nil
}
