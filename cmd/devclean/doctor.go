package main

import (
	"errors"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Pastranauwu/devclean/internal/config"
	"github.com/Pastranauwu/devclean/internal/executor"
)

type chequeoDoctor struct {
	Nombre  string `json:"nombre"`
	OK      bool   `json:"ok"`
	Detalle string `json:"detalle,omitempty"`
}

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "verifica git, configuración, keys y ejecutores",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDoctor()
		},
	}
}

// runDoctor verifica el entorno (§7): git, repo, configuración, keys y
// al menos un ejecutor instalado. Reporta todo y falla solo si algo
// crítico falta.
func runDoctor() error {
	var checks []chequeoDoctor

	// git
	if _, err := exec.LookPath("git"); err != nil {
		checks = append(checks, chequeoDoctor{"git", false, "git no está instalado"})
	} else {
		checks = append(checks, chequeoDoctor{"git", true, ""})
	}

	// repo
	root, repoErr := config.RepoRoot(mustGetwd())
	if repoErr != nil {
		checks = append(checks, chequeoDoctor{"repositorio", false, repoErr.Error()})
	} else {
		checks = append(checks, chequeoDoctor{"repositorio", true, root})
	}

	// configuración
	var cfg config.Config
	if repoErr == nil && config.Exists(root) {
		if c, err := config.Load(root); err != nil {
			checks = append(checks, chequeoDoctor{"configuración", false, err.Error()})
		} else {
			cfg = c
			checks = append(checks, chequeoDoctor{"configuración", true, ""})
		}
	} else {
		checks = append(checks, chequeoDoctor{"configuración", false, "sin .devclean · corre devclean init"})
	}

	// ejecutores
	ejecutores := []executor.Executor{executor.OpenCode{}, executor.Claude{}}
	var disponibles []string
	for _, e := range ejecutores {
		if err := e.Available(); err == nil {
			disponibles = append(disponibles, e.Name())
		}
	}
	if len(disponibles) > 0 {
		checks = append(checks, chequeoDoctor{"ejecutores", true, strings.Join(disponibles, ", ")})
	} else {
		checks = append(checks, chequeoDoctor{"ejecutores", false, "ninguno instalado · instala opencode o claude"})
	}

	// keys: al menos una variable de proveedor en el entorno
	keys := []string{"OPENCODE_API_KEY", "ANTHROPIC_API_KEY", "DEEPSEEK_API_KEY", "OPENAI_API_KEY"}
	var presentes []string
	for _, k := range keys {
		if os.Getenv(k) != "" {
			presentes = append(presentes, k)
		}
	}
	if len(presentes) > 0 {
		checks = append(checks, chequeoDoctor{"keys", true, strings.Join(presentes, ", ")})
	} else {
		checks = append(checks, chequeoDoctor{"keys", false, "ninguna key de proveedor en el entorno"})
	}

	if err := out.Data(checks); err != nil {
		return err
	}
	critico := false
	for _, c := range checks {
		if c.OK {
			out.Line("✓ %s%s", c.Nombre, detalleDoctor(c))
		} else {
			out.Line("✗ %s  · %s", c.Nombre, c.Detalle)
			if c.Nombre != "keys" {
				critico = true
			}
		}
	}
	if critico {
		return errors.New("el entorno no está listo · arregla lo marcado con ✗")
	}
	out.Line("entorno listo")
	_ = cfg
	return nil
}

func detalleDoctor(c chequeoDoctor) string {
	if c.Detalle == "" {
		return ""
	}
	return "  · " + c.Detalle
}

func mustGetwd() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}
