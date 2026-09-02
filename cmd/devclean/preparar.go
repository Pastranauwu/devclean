package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/Pastranauwu/devclean/internal/config"
	"github.com/Pastranauwu/devclean/internal/executor"
)

// entornoListo es prepararEntorno desde el cwd y la terminal, una sola
// vez por proceso: up encadena plan y run, que también lo llaman, y el
// catálogo de modelos no hace falta consultarlo tres veces.
var entornoPreparado *struct {
	root string
	cfg  config.Config
}

func entornoListo(entregar bool) (string, config.Config, error) {
	if entornoPreparado != nil {
		return entornoPreparado.root, entornoPreparado.cfg, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", config.Config{}, err
	}
	// el orquestador configura; el humano solo pide. Lo que no se
	// pueda arreglar solo se pregunta en terminal.
	var in io.Reader
	if !out.JSON() && isTerminal(os.Stdin) {
		in = os.Stdin
	}
	root, cfg, err := prepararEntorno(cwd, in, entregar)
	if err != nil {
		return "", config.Config{}, err
	}
	entornoPreparado = &struct {
		root string
		cfg  config.Config
	}{root, cfg}
	return root, cfg, nil
}

// prepararEntorno deja el repo listo para `up` sin que el humano
// configure nada a mano: git init, .devclean, commit inicial, ejecutor,
// modelos, comando de pruebas y, si va a entregar, gh y el remoto. Lo
// que se puede arreglar solo se arregla y se avisa en una línea; lo que
// no, se pregunta en terminal (`in` no nil) o se corta con qué hacer.
// Devuelve la raíz del repo y la configuración ya saneada.
func prepararEntorno(cwd string, in io.Reader, entregar bool) (string, config.Config, error) {
	var lector *bufio.Reader
	if in != nil {
		lector = bufio.NewReader(in)
	}

	if _, err := exec.LookPath("git"); err != nil {
		return "", config.Config{}, errors.New("git no está instalado · instálalo y vuelve a correr")
	}

	// 1. repositorio
	root, err := config.RepoRoot(cwd)
	if err != nil {
		if _, err := gitEn(cwd, "init", "--quiet"); err != nil {
			return "", config.Config{}, fmt.Errorf("no se pudo crear el repo git · %s", err)
		}
		out.Line("· sin repo git · creado con git init")
		if root, err = config.RepoRoot(cwd); err != nil {
			return "", config.Config{}, err
		}
	}

	// 2. .devclean
	if !config.Exists(root) {
		out.Line("· sin .devclean · configurando")
		// con dos CLIs instalados, la única decisión que vale preguntar
		cli := ""
		if lector != nil && esTUI() {
			cli = elegirCLIAMano(clisInstalados())
		}
		if err := runInit(root, "", "", cli, nil, false); err != nil {
			return "", config.Config{}, err
		}
	}
	cfg, err := config.Load(root)
	if err != nil {
		return "", config.Config{}, err
	}
	guardar := false

	// 3. rama base
	if cfg.Base == "" {
		if cfg.Base = config.DetectBaseBranch(root); cfg.Base == "" {
			cfg.Base = "main"
		}
		out.Line("· rama base: %s", cfg.Base)
		guardar = true
	}

	// 4. commit inicial: los cuartos son worktrees y sin commit no nacen
	if _, err := gitEn(root, "rev-parse", "--verify", "--quiet", "HEAD"); err != nil {
		if err := commitInicial(root, lector); err != nil {
			return "", config.Config{}, err
		}
	}

	// 5. ejecutor: el configurado, o el que sí esté instalado
	ex, err := elegirEjecutor(cfg.Cli)
	if err != nil && cfg.Cli != "" {
		if ex, err = elegirEjecutor(""); err == nil {
			out.Line("· %s no está instalado · se usa %s", cfg.Cli, ex.Name())
		}
	}
	if err != nil {
		ex, err = instalarEjecutor(lector)
		if err != nil {
			return "", config.Config{}, err
		}
	}
	if cfg.Cli != ex.Name() {
		cfg.Cli, guardar = ex.Name(), true
	}
	if key := keyDe(cfg, ex.Name()); key != "" && os.Getenv(key) == "" {
		out.Line("· sin %s en el entorno · si %s no está logueado, la corrida va a fallar", key, ex.Name())
	}

	// 6. modelos: ids que el CLI no reconoce mueren en cada intento
	if catalogo, err := catalogoDe(ex); err == nil && len(catalogo) > 0 {
		var declarados []string
		for _, peso := range config.Pesos {
			declarados = append(declarados, cfg.Modelos[peso])
		}
		if malos := config.ModelosValidos(declarados, catalogo); len(malos) > 0 || len(cfg.Modelos) == 0 {
			if len(malos) > 0 {
				out.Line("· modelos que %s no reconoce: %s · se reasignan del catálogo", ex.Name(), strings.Join(malos, ", "))
			}
			cfg.Modelos, guardar = config.ElegirModelos(catalogo), true
			for _, peso := range config.Pesos {
				out.Line("· modelo %s: %s", peso, cfg.Modelos[peso])
			}
		}
	}

	// 7. comando de pruebas: sin él, ship no verifica el conjunto
	if strings.TrimSpace(cfg.Pruebas) == "" && !config.DetectEmpty(root) {
		if p, ok := config.DetectTestCommand(root); ok {
			cfg.Pruebas, guardar = p, true
			out.Line("· comando de pruebas detectado: %s", p)
		} else if lector != nil {
			if p := confirmarPruebas(lector, ""); p != "" {
				cfg.Pruebas, guardar = p, true
			}
		}
		if strings.TrimSpace(cfg.Pruebas) == "" {
			out.Line("· sin comando de pruebas · ship no va a verificar el conjunto · decláralo en .devclean/config.yml")
		}
	}

	if guardar {
		if err := cfg.Save(root); err != nil {
			return "", config.Config{}, err
		}
	}

	// 8. entrega: fallar acá, antes de gastar un token, no al final
	if entregar {
		if err := prepararEntrega(root, lector); err != nil {
			return "", config.Config{}, err
		}
	}
	return root, cfg, nil
}

// commitInicial crea el primer commit. Si lo único sin versionar es lo
// que devclean acaba de escribir (.devclean, las skills), lo hace solo;
// si hay más archivos, pregunta antes de meter al historial algo que el
// humano no revisó.
func commitInicial(root string, lector *bufio.Reader) error {
	estado, _ := gitEn(root, "status", "--porcelain")
	soloNuestro := true
	for _, l := range strings.Split(strings.TrimSpace(estado), "\n") {
		if l == "" {
			continue
		}
		ruta := strings.TrimSpace(l[3:])
		if !strings.HasPrefix(ruta, config.DirName) && !strings.HasPrefix(ruta, ".agents") && ruta != "skills-lock.json" {
			soloNuestro = false
			break
		}
	}
	if !soloNuestro {
		if lector == nil {
			return errors.New("no hay commits y hay archivos sin versionar · haz un commit inicial y reintenta")
		}
		if !aprobar(lector, "no hay commits · ¿hago un commit inicial con todo lo que hay? [s/N]") {
			return errors.New("sin commit inicial · hazlo vos y vuelve a correr")
		}
	}
	if _, err := gitEn(root, "add", "-A"); err != nil {
		return fmt.Errorf("no se pudo preparar el commit inicial · %s", err)
	}
	if _, err := gitEn(root, "commit", "--quiet", "--allow-empty", "-m", "chore: inicio"); err != nil {
		return fmt.Errorf("no se pudo hacer el commit inicial · %s", err)
	}
	out.Line("· sin commits · commit inicial creado")
	return nil
}

// instalarEjecutor ofrece instalar un CLI de agente con npm cuando no
// hay ninguno. Solo en terminal y solo con permiso: instalar globales
// sin preguntar no es plug and play, es intrusión.
func instalarEjecutor(lector *bufio.Reader) (executor.Executor, error) {
	faltan := errors.New("ningún ejecutor instalado · instala opencode (npm i -g opencode-ai) o claude (npm i -g @anthropic-ai/claude-code) y vuelve a correr")
	if lector == nil {
		return nil, faltan
	}
	if _, err := exec.LookPath("npm"); err != nil {
		return nil, faltan
	}
	if !aprobar(lector, "ningún ejecutor instalado · ¿instalo claude con npm i -g @anthropic-ai/claude-code? [s/N]") {
		return nil, faltan
	}
	cmd := exec.Command("npm", "i", "-g", "@anthropic-ai/claude-code")
	cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("npm no pudo instalar claude · %s", err)
	}
	ex, err := elegirEjecutor("claude")
	if err != nil {
		return nil, err
	}
	out.Line("✓ claude instalado · inicia sesión con `claude` si todavía no lo hiciste")
	return ex, nil
}

// prepararEntrega verifica lo que `ship` necesita: gh y un remoto origin.
// Sin remoto, en terminal lo pide; si no, corta con la instrucción.
func prepararEntrega(root string, lector *bufio.Reader) error {
	if _, err := exec.LookPath("gh"); err != nil {
		return errors.New("--ship necesita gh para abrir el PR · instálalo (https://cli.github.com) o corre sin --ship")
	}
	if _, err := gitEn(root, "remote", "get-url", "origin"); err == nil {
		return nil
	}
	if lector == nil {
		return errors.New("--ship necesita un remoto origin · git remote add origin <url>, o corre sin --ship")
	}
	url := preguntar(lector, "sin remoto origin · url del repositorio (enter para correr sin entregar):")
	if url == "" {
		return errors.New("sin remoto origin · corre sin --ship o agrégalo con git remote add origin <url>")
	}
	if salida, err := gitEn(root, "remote", "add", "origin", url); err != nil {
		return fmt.Errorf("no se pudo agregar el remoto · %s", strings.TrimSpace(salida))
	}
	out.Line("· remoto origin: %s", url)
	return nil
}

func catalogoDe(ex executor.Executor) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return ex.Models(ctx)
}

// keyDe devuelve la variable de entorno de la key del CLI: la declarada
// en config, o la habitual del proveedor.
func keyDe(cfg config.Config, cli string) string {
	if k := cfg.KeyEnvDe(cli); k != "" {
		return k
	}
	if cli == "opencode" {
		return "OPENCODE_API_KEY"
	}
	return "ANTHROPIC_API_KEY"
}

func preguntar(lector *bufio.Reader, msg string) string {
	out.Line("%s", msg)
	l, _ := lector.ReadString('\n')
	return strings.TrimSpace(l)
}

func aprobar(lector *bufio.Reader, msg string) bool {
	r := strings.ToLower(preguntar(lector, msg))
	return r == "s" || r == "si" || r == "sí" || r == "y"
}

func gitEn(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	salida, err := cmd.CombinedOutput()
	return string(salida), err
}
