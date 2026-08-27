package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/Pastranauwu/devclean/internal/config"
	"github.com/Pastranauwu/devclean/internal/executor"
	"github.com/Pastranauwu/devclean/internal/plan"
	"github.com/Pastranauwu/devclean/internal/task"
)

// propuesta es una tarea del plan, ya con id asignado.
type propuesta struct {
	ID          string   `json:"id"`
	Titulo      string   `json:"titulo"`
	Porque      string   `json:"porque,omitempty"`
	ListoCuando string   `json:"listo_cuando"`
	TocarSolo   []string `json:"tocar_solo,omitempty"`
	Riesgos     string   `json:"riesgos,omitempty"`
}

func newPlanCmd() *cobra.Command {
	var modelo, ejecutor string
	var aprobar bool
	cmd := &cobra.Command{
		Use:   `plan "<texto>"`,
		Short: "convierte una petición en contratos de tarea",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPlan(strings.Join(args, " "), modelo, ejecutor, aprobar)
		},
	}
	cmd.Flags().StringVar(&modelo, "modelo", "", "modelo del planificador (por defecto, el suyo)")
	cmd.Flags().StringVar(&ejecutor, "ejecutor", "", "opencode o claude (por defecto, el primero disponible)")
	cmd.Flags().BoolVar(&aprobar, "aprobar", false, "crea las tareas sin preguntar")
	return cmd
}

func runPlan(frase, modelo, ejecutor string, aprobar bool) error {
	root, err := projectRoot()
	if err != nil {
		return err
	}
	dir := config.TasksDir(root)

	cfg, err := config.Load(root)
	if err != nil {
		return err
	}
	if modelo == "" {
		modelo = config.ModeloRol(cfg, "planificador")
	}

	ex, err := elegirEjecutor(ejecutor)
	if err != nil {
		return err
	}

	borradores, err := plan.Generar(context.Background(), generadorPlan{ex: ex, modelo: modelo, root: root}, frase)
	if err != nil {
		return err
	}

	ids, err := idsCorrelativos(dir, len(borradores))
	if err != nil {
		return err
	}
	props := make([]propuesta, len(borradores))
	for i, b := range borradores {
		props[i] = propuesta{
			ID:          ids[i],
			Titulo:      b.Titulo,
			Porque:      b.Porque,
			ListoCuando: b.ListoCuando,
			TocarSolo:   b.TocarSolo,
			Riesgos:     b.Riesgos,
		}
	}

	if err := out.Data(props); err != nil {
		return err
	}
	out.Line("propongo %d tareas:", len(props))
	for _, p := range props {
		out.Line("%s  %s  · listo cuando: %s", p.ID, p.Titulo, p.ListoCuando)
	}

	if !aprobar {
		if !isTerminal(os.Stdin) {
			out.Line("sin confirmación interactiva · usa --aprobar para crearlas")
			return nil
		}
		if !confirmar(os.Stdin) {
			out.Line("plan descartado")
			return nil
		}
	}

	for i, b := range borradores {
		t := task.Task{
			Version:        task.Version,
			ID:             ids[i],
			Titulo:         b.Titulo,
			Porque:         b.Porque,
			ListoCuando:    b.ListoCuando,
			TocarSolo:      b.TocarSolo,
			NoTocar:        b.NoTocar,
			Riesgos:        b.Riesgos,
			LimiteIntentos: task.DefaultLimiteIntentos,
			LimiteLineas:   task.DefaultLimiteLineas,
		}
		if err := task.Save(dir, t); err != nil {
			return err
		}
	}
	out.Line("✓ %s creadas · revisa con devclean check", strings.Join(ids, ", "))
	return nil
}

// idsCorrelativos devuelve n ids libres a partir del primero.
func idsCorrelativos(dir string, n int) ([]string, error) {
	first, err := task.NextID(dir)
	if err != nil {
		return nil, err
	}
	num, err := strconv.Atoi(strings.TrimPrefix(first, "T-"))
	if err != nil {
		return nil, err
	}
	ids := make([]string, n)
	for i := range ids {
		ids[i] = fmt.Sprintf("T-%03d", num+i)
	}
	return ids, nil
}

// confirmar pregunta s/n y devuelve si el usuario aprobó.
func confirmar(in io.Reader) bool {
	out.Line("¿crear estas tareas? [s/n]")
	linea, _ := bufio.NewReader(in).ReadString('\n')
	r := strings.ToLower(strings.TrimSpace(linea))
	return r == "s" || r == "si" || r == "y" || r == "yes"
}

// generadorPlan adapta el ejecutor al generador de texto del planificador.
type generadorPlan struct {
	ex     executor.Executor
	modelo string
	root   string
}

func (g generadorPlan) Generar(ctx context.Context, prompt string) (string, error) {
	res, err := g.ex.Run(ctx, executor.Request{
		RoomPath: g.root,
		Prompt:   prompt,
		Model:    g.modelo,
		Timeout:  5 * time.Minute,
	})
	if err != nil {
		return "", err
	}
	return res.Text, nil
}
