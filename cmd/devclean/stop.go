package main

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Pastranauwu/devclean/internal/state"
	"github.com/spf13/cobra"
)

// corridaPath guarda el pid de la corrida viva (run o up), para que
// `devclean stop` sepa a quién parar desde otra terminal o con la
// corrida en --fondo.
func corridaPath(root string) string {
	return filepath.Join(root, ".devclean", "corrida.pid")
}

// marcarCorrida anota este proceso como la corrida viva y devuelve con
// qué soltarla. Si ya está anotado (up que llama a run), no hace nada:
// así el run de adentro no borra la marca antes de que up entregue.
func marcarCorrida(root string) func() {
	pid := strconv.Itoa(os.Getpid())
	if b, err := os.ReadFile(corridaPath(root)); err == nil && strings.TrimSpace(string(b)) == pid {
		return func() {}
	}
	if os.WriteFile(corridaPath(root), []byte(pid), 0o644) != nil {
		return func() {}
	}
	return func() {
		if b, err := os.ReadFile(corridaPath(root)); err == nil && strings.TrimSpace(string(b)) == pid {
			_ = os.Remove(corridaPath(root))
		}
	}
}

func newStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "para la corrida en curso y a sus agentes",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := projectRoot()
			if err != nil {
				return err
			}
			b, err := os.ReadFile(corridaPath(root))
			if errors.Is(err, os.ErrNotExist) {
				out.Line("no hay corrida en curso")
				return nil
			}
			if err != nil {
				return err
			}
			pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
			if err != nil {
				_ = os.Remove(corridaPath(root))
				return errors.New("corrida.pid corrupto · se borró · no había nada que parar")
			}
			if !esDevclean(pid) {
				_ = os.Remove(corridaPath(root))
				out.Line("la corrida %d ya no existe · marca borrada", pid)
				return nil
			}
			if err := pararProceso(pid); err != nil {
				return errors.New("no se pudo parar la corrida " + strconv.Itoa(pid) + " · " + err.Error())
			}
			_ = os.Remove(corridaPath(root))

			// lo que estaba en curso queda detenido: sin esto el tablero
			// la pinta corriendo hasta que el latido se pone rancio
			var paradas []string
			estados, _ := state.List(root)
			for _, s := range estados {
				if s.Estado != state.EnCurso {
					continue
				}
				s.Estado, s.UltimoError = state.Detenida, "parada con devclean stop"
				if state.Save(root, s) == nil {
					paradas = append(paradas, s.ID)
				}
			}
			out.Line("corrida %d parada", pid)
			if len(paradas) > 0 {
				out.Line("detenidas · %s · retómalas con devclean run --reintentar", strings.Join(paradas, ", "))
			}
			return nil
		},
	}
}
