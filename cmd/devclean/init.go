package main

import (
	"bufio"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/Pastranauwu/devclean/internal/config"
	"github.com/Pastranauwu/devclean/internal/gate"
	"github.com/Pastranauwu/devclean/internal/loop"
)

type initResult struct {
	Base    string `json:"base"`
	Pruebas string `json:"pruebas"`
	Config  string `json:"config"`
}

func newInitCmd() *cobra.Command {
	var pruebas, plantilla string
	var sinSkills bool
	cmd := &cobra.Command{
		Use:   "init",
		Short: "detecta el repo y crea .devclean/",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			var in io.Reader
			if pruebas == "" && plantilla == "" && !out.JSON() && isTerminal(os.Stdin) {
				in = os.Stdin
			}
			return runInit(cwd, pruebas, plantilla, in, sinSkills)
		},
	}
	cmd.Flags().StringVar(&pruebas, "pruebas", "", "comando de pruebas del proyecto, en vez del detectado")
	cmd.Flags().StringVar(&plantilla, "pruebas-plantilla", "", "stack de pruebas: go, node o python")
	cmd.Flags().BoolVar(&sinSkills, "sin-skills", false, "no traer las skills por defecto (podés hacerlo luego con devclean skills sync)")
	return cmd
}

// confirmarPruebas shows the detected test command and takes a
// correction (adenda C.5): una detección silenciosa equivocada cuesta
// horas. Enter en blanco acepta lo detectado.
func confirmarPruebas(in io.Reader, detectado string) string {
	if detectado == "" {
		out.Line("· comando de pruebas no detectado · escribe el de este proyecto, o enter para dejarlo vacío")
	} else {
		out.Line("· comando de pruebas detectado: %s", detectado)
		out.Line("  enter para aceptarlo, o escribe el correcto")
	}
	linea, _ := bufio.NewReader(in).ReadString('\n')
	if respuesta := strings.TrimSpace(linea); respuesta != "" {
		return respuesta
	}
	return detectado
}

func runInit(cwd, pruebasFlag, plantilla string, in io.Reader, sinSkills bool) error {
	root, err := config.RepoRoot(cwd)
	if err != nil {
		return err
	}
	if config.Exists(root) {
		return errors.New("ya existe .devclean en este repo · nada que hacer")
	}

	base := config.DetectBaseBranch(root)
	pruebas, pruebasOK := config.DetectTestCommand(root)
	switch {
	case pruebasFlag != "":
		pruebas, pruebasOK = pruebasFlag, true
	case plantilla != "":
		cmd, ok := config.PlantillaPruebas(plantilla)
		if !ok {
			return errors.New("plantilla desconocida: " + plantilla + " · usa go, node o python")
		}
		pruebas, pruebasOK = cmd, true
	case in != nil:
		pruebas = confirmarPruebas(in, pruebas)
		pruebasOK = pruebas != ""
	}

	if err := os.MkdirAll(config.TasksDir(root), 0o755); err != nil {
		return err
	}
	// los cuartos son worktrees: nunca deben versionarse
	gitignore := filepath.Join(config.Dir(root), ".gitignore")
	if err := os.WriteFile(gitignore, []byte("rooms/\n"), 0o644); err != nil {
		return err
	}
	cfg := config.Config{
		Base:            base,
		Pruebas:         pruebas,
		ZonasProhibidas: config.DefaultForbiddenZones(),
		PatronesPrueba:  config.DefaultTestPatterns(),
		TimeoutEsclusa:  int(gate.DefaultTimeout / time.Second),
		TimeoutAgente:   int(loop.DefaultAgentTimeout / time.Second),
		TimeoutPruebas:  int(loop.DefaultTimeout / time.Second),
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
	if config.DetectEmpty(root) {
		out.Line("· repositorio vacío · devclean plan arrancará desde cero proponiendo un stack")
	}
	out.Line("✓ configuración creada en .devclean/")
	if sinSkills {
		out.Line("· skills sin traer · corre devclean skills sync cuando quieras")
	} else {
		out.Line("· trayendo skills por defecto (npx skills add) …")
		if err := runSkillsSync(); err != nil {
			out.Line("· no se pudieron traer las skills · %s · reintenta con devclean skills sync", err)
		}
	}
	stack := config.StackDePlantilla(plantilla)
	if stack == "" {
		stack = config.DetectLanguage(root)
	}
	imprimirPlantillasListoCuando(stack)
	return nil
}
