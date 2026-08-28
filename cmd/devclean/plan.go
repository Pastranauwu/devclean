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
	"github.com/Pastranauwu/devclean/internal/constitution"
	"github.com/Pastranauwu/devclean/internal/executor"
	"github.com/Pastranauwu/devclean/internal/plan"
	"github.com/Pastranauwu/devclean/internal/task"
	"github.com/Pastranauwu/devclean/internal/tui"
)

// propuesta es una tarea del plan, ya con id asignado.
type propuesta struct {
	ID          string   `json:"id"`
	Titulo      string   `json:"titulo"`
	Porque      string   `json:"porque,omitempty"`
	ListoCuando string   `json:"listo_cuando"`
	TocarSolo   []string `json:"tocar_solo,omitempty"`
	DependeDe   []string `json:"depende_de,omitempty"`
	Expone      []string `json:"expone,omitempty"`
	Usa         []string `json:"usa,omitempty"`
	Riesgos     string   `json:"riesgos,omitempty"`
	Peso        string   `json:"peso,omitempty"`
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

	if ejecutor == "" {
		ejecutor = cfg.Cli
	}
	ex, err := elegirEjecutor(ejecutor)
	if err != nil {
		return err
	}

	constitucion, err := constitution.Load(root)
	if err != nil {
		return err
	}
	esVacio := config.DetectEmpty(root)
	ctx := plan.Contexto{
		Lenguaje:     config.DetectLanguage(root),
		EsVacio:      esVacio,
		Pruebas:      cfg.Pruebas,
		Constitucion: constitucion,
	}
	if esVacio && !aprobar && isTerminal(os.Stdin) {
		ctx.Stack, ctx.Requisitos = pedirRequisitos(os.Stdin, esTUI())
	}

	var borradores []plan.Borrador
	generar := func() error {
		var err error
		borradores, err = plan.Generar(context.Background(), generadorPlan{ex: ex, modelo: modelo, root: root}, ctx, frase)
		return err
	}
	if esTUI() {
		err = tui.Esperar("generando plan · "+modelo, generar)
	} else {
		err = generar()
	}
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
			DependeDe:   b.DependeDe,
			Expone:      b.Expone,
			Usa:         b.Usa,
			Riesgos:     b.Riesgos,
			Peso:        b.Peso,
		}
	}

	if err := out.Data(props); err != nil {
		return err
	}
	if esTUI() {
		var cuerpo strings.Builder
		cuerpo.WriteString(tui.Titulo(fmt.Sprintf("PROPONGO %d TAREAS", len(props))) + "\n\n")
		for _, p := range props {
			cuerpo.WriteString(p.ID + "  " + p.Titulo + "  " + tui.Apagado("· listo cuando: "+p.ListoCuando) + "\n")
		}
		out.Line("%s", tui.Caja(strings.TrimRight(cuerpo.String(), "\n")))
	} else {
		out.Line("propongo %d tareas:", len(props))
		for _, p := range props {
			out.Line("%s  %s  · listo cuando: %s", p.ID, p.Titulo, p.ListoCuando)
		}
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
			DependeDe:      b.DependeDe,
			Expone:         b.Expone,
			Usa:            b.Usa,
			Riesgos:        b.Riesgos,
			Peso:           b.Peso,
			LimiteIntentos: task.DefaultLimiteIntentos,
			LimiteLineas:   task.DefaultLimiteLineas,
		}
		if err := task.Save(dir, t); err != nil {
			return err
		}
	}
	out.Line("✓ %s creadas · revisa con devclean check %s", strings.Join(ids, ", "), ids[0])

	// `pruebas` es lo que corre el paso bisectable de la esclusa de
	// salida. Si está vacío, ship falla recién al final, cuando el
	// trabajo ya está hecho; avisar acá cuesta una línea y llega a
	// tiempo. En greenfield `init` no pudo detectarlo: no había código.
	if strings.TrimSpace(cfg.Pruebas) == "" {
		out.Line("· sin comando de pruebas en config.yml · decláralo antes de devclean ship")
	}
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

// pedirRequisitos reúne el stack y los requisitos extra del humano
// cuando el repo está vacío (Fase 2): sin esto, el planificador no
// tiene de dónde agarrarse y alucina un stack.
func pedirRequisitos(in io.Reader, tuiMode bool) (stack, requisitos string) {
	leer := bufio.NewReader(in)

	titulo, etiquetaStack, etiquetaPieza := "repositorio vacío · define el stack y los requisitos antes de planear",
		"stack (go, node, python, rust, ...) · enter para que lo elija el modelo:",
		"requisito o pieza (una línea, enter para terminar):"
	if tuiMode {
		titulo, etiquetaStack, etiquetaPieza = tui.Titulo(titulo), tui.Apagado(etiquetaStack), tui.Apagado(etiquetaPieza)
	}

	out.Line("%s", titulo)
	out.Line("%s", etiquetaStack)
	if l, _ := leer.ReadString('\n'); strings.TrimSpace(l) != "" {
		stack = strings.ToLower(strings.TrimSpace(l))
	}

	var piezas []string
	for {
		out.Line("%s", etiquetaPieza)
		l, _ := leer.ReadString('\n')
		l = strings.TrimSpace(l)
		if l == "" {
			break
		}
		piezas = append(piezas, l)
	}
	return stack, strings.Join(piezas, "; ")
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
