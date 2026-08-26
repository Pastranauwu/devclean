package main

import (
	"errors"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/Pastranauwu/devclean/internal/config"
	"github.com/Pastranauwu/devclean/internal/gate"
)

type initResult struct {
	Base    string `json:"base"`
	Pruebas string `json:"pruebas"`
	Config  string `json:"config"`
}

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "detecta el repo y crea .devclean/",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			return runInit(cwd)
		},
	}
}

func runInit(cwd string) error {
	root, err := config.RepoRoot(cwd)
	if err != nil {
		return err
	}
	if config.Exists(root) {
		return errors.New("ya existe .devclean en este repo · nada que hacer")
	}

	base := config.DetectBaseBranch(root)
	pruebas, pruebasOK := config.DetectTestCommand(root)

	if err := os.MkdirAll(config.TasksDir(root), 0o755); err != nil {
		return err
	}
	cfg := config.Config{
		Base:            base,
		Pruebas:         pruebas,
		ZonasProhibidas: config.DefaultForbiddenZones(),
		TimeoutEsclusa:  int(gate.DefaultTimeout / time.Second),
	}
	if err := cfg.Save(root); err != nil {
		return err
	}

	if err := out.Data(initResult{Base: base, Pruebas: pruebas, Config: config.Path(root)}); err != nil {
		return err
	}
	out.Line("✓ repositorio detectado")
	if base != "" {
		out.Line("✓ rama base: %s", base)
	} else {
		out.Line("· rama base no detectada · edita .devclean/config.yml")
	}
	if pruebasOK {
		out.Line("✓ comando de pruebas detectado: %s", pruebas)
	} else {
		out.Line("· comando de pruebas no detectado · edita .devclean/config.yml")
	}
	out.Line("✓ configuración creada en .devclean/")
	return nil
}
