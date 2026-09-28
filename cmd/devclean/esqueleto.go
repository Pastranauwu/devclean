package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Pastranauwu/devclean/internal/config"
	"github.com/Pastranauwu/devclean/internal/esqueleto"
	"github.com/Pastranauwu/devclean/internal/executor"
	"github.com/Pastranauwu/devclean/internal/loop"
	"github.com/Pastranauwu/devclean/internal/room"
	"github.com/Pastranauwu/devclean/internal/spec"
	"github.com/Pastranauwu/devclean/internal/state"
	"github.com/Pastranauwu/devclean/internal/task"
	"github.com/Pastranauwu/devclean/internal/ventanas"
)

// correccionesEsqueleto es cuántas veces se le devuelve al arquitecto lo
// que la verificación encontró antes de rendirse.
const correccionesEsqueleto = 2

// timeoutArquitecto: escribir el esqueleto de un proyecto es la
// invocación más larga de la corrida (el plan en prosa de closet ya
// tardó 19 minutos sin escribir un archivo).
const timeoutArquitecto = 45 * time.Minute

// notaCambio va en las tareas que cambian código que ya funciona: no hay
// stub que rellenar y lo que ya existe no se puede romper.
const notaCambio = "Cambias código que ya funciona. Primero escribe la prueba del comportamiento nuevo en el archivo que corre listo_cuando, con los \"Casos:\" de arriba; después haz el cambio. No cambies firmas, nombres ni exportaciones que otros módulos usan salvo que el contrato lo pida, y las pruebas que ya existen tienen que seguir pasando. El contexto está en " + esqueleto.Documento + "."

// notaPara elige la nota según lo que toca la tarea: rellenar un stub o
// cambiar código existente.
func notaPara(ctx context.Context, dir string, tocar []string) string {
	if esqueleto.ConStub(ctx, dir, tocar) {
		return notaRelleno
	}
	return notaCambio
}

// notaRelleno va en las notas de cada tarea de relleno: el agente barato
// no necesita más que saber qué reemplazar, dónde está el contrato y que
// la prueba sale de sus casos.
const notaRelleno = "Primero escribe la prueba del archivo que corre listo_cuando con los \"Casos:\" del contrato de cada stub (y los casos límite que el contrato nombre); las dependencias de otros módulos van con fakes. Después rellena los cuerpos que lanzan \"" + esqueleto.Marca + "\". No cambies firmas, nombres ni exportaciones: otros módulos ya dependen de ellas. El contrato está en el comentario de cada stub y en " + esqueleto.Documento + "."

// planearEsqueleto es el camino de requirements: el modelo grande
// escribe el esqueleto en el cuarto de la primera tarea, devclean lo
// verifica sin modelo y lo deja como tarea `lista`; cada módulo stub
// pasa a ser una tarea de relleno que depende de él. run lo integra por
// sembrarVerdesPrevias como cualquier verde de una corrida anterior.
func planearEsqueleto(root string, s *spec.Spec, pedido string) error {
	cfg, err := config.Load(root)
	if err != nil {
		return err
	}
	ex, err := elegirEjecutor(cfg.Cli)
	if err != nil {
		return err
	}
	pctx, zonas, _, err := contextoPlan(root, cfg)
	if err != nil {
		return err
	}
	ids, err := idsCorrelativos(config.TasksDir(root), 2)
	if err != nil {
		return err
	}
	id := ids[0]
	ctx := context.Background()
	// Ensure y no Create: si una corrida anterior del arquitecto se cortó
	// o no pasó la verificación, lo que escribió sigue ahí y se corrige
	r, err := room.Ensure(ctx, root, id, cfg.Base)
	if err != nil {
		return err
	}
	// de la base y no del cuarto: al retomar, el cuarto ya tiene el
	// ARCHITECTURE.md que escribió este mismo arquitecto
	previo, err := gitEn(root, "show", cfg.Base+":"+esqueleto.Documento)
	if err != nil {
		previo = ""
	}
	// docker por defecto: el humano lo apaga en sus reglas ("sin docker")
	// cuando lo que se construye no se despliega (una librería, un CLI)
	sinDocker := strings.Contains(strings.ToLower(pedido), "sin docker")
	original := esqueleto.Prompt(esqueleto.Pedido{Texto: pedido, Previo: previo, PrimerID: ids[1], SinDocker: sinDocker}, pctx)

	modelo := config.ModeloRol(cfg, "planificador")
	timeout := timeoutArquitecto
	if t := time.Duration(cfg.TimeoutAgente) * time.Second; t > timeout {
		timeout = t
	}
	pruebaTimeout := loop.DefaultTimeout
	if cfg.TimeoutPruebas > 0 {
		pruebaTimeout = time.Duration(cfg.TimeoutPruebas) * time.Second
	}
	reg := ventanas.Nuevo(ventanas.LedgerPath(), cfg.PresupuestoVentanas)

	var res esqueleto.Resultado
	prompt := original
	guardada := respuestaEsqueleto(root, id, original)
	for vuelta := 0; ; vuelta++ {
		titulo := fmt.Sprintf("el arquitecto escribe el esqueleto · %s · %s", id, modelo)
		if vuelta > 0 {
			titulo = fmt.Sprintf("el arquitecto corrige el esqueleto (%d/%d) · %s", vuelta, correccionesEsqueleto, modelo)
		}
		var texto string
		if vuelta == 0 && guardada != "" {
			// el esqueleto ya se pagó: lo que falló fue devclean después
			// (instalar, parsear, verificar). Se reverifica sin llamarlo.
			out.Line("· reusando la respuesta guardada del arquitecto · borra %s para rehacerlo", rutaRespuestaEsqueleto(root, id))
			texto, err = guardada, nil
		} else {
			err = esperarPlan(titulo, func(avance func(string)) error {
				out, err := ex.Run(ctx, executor.Request{
					RoomPath: r.Path,
					Prompt:   prompt,
					Model:    modelo,
					Timeout:  timeout,
					Env:      room.Entorno(r.Path),
					Effort:   "medium",
					Avance:   avance,
				})
				guardarLogEsqueleto(root, id, vuelta, prompt, out)
				reg.Registrar(ex.Name(), tokensDe(out.Tokens).Gasto())
				if out.Text != "" {
					guardarRespuestaEsqueleto(root, id, original, out.Text)
				}
				texto = out.Text
				return err
			})
		}
		if err != nil {
			return fmt.Errorf("el arquitecto no terminó · %w · lo escrito sigue en %s, vuelve a correr para que lo corrija", err, r.Path)
		}
		// lo que el arquitecto instaló después de crear el cuarto
		// (package.json nuevo, pyproject) también tiene que estar. Si no
		// instala es un error del arquitecto (un paquete que no existe):
		// se le devuelve para que lo corrija, no se tira lo pagado
		var problemas []string
		if err := room.InstalarDependencias(ctx, r.Path); err != nil {
			problemas = append(problemas, "las dependencias no instalan (corrige el manifiesto: quita o reemplaza lo que no existe en el registro): "+err.Error())
		}
		res, err = esqueleto.Parse(texto)
		if err != nil {
			problemas = append(problemas, err.Error())
		} else {
			problemas = append(problemas, esqueleto.Problemas(ctx, esqueleto.Verificacion{
				Dir: r.Path, Base: r.Commit, Docker: !sinDocker, Timeout: pruebaTimeout, Env: room.Entorno(r.Path),
			}, res)...)
		}
		if len(problemas) == 0 {
			break
		}
		if vuelta == correccionesEsqueleto {
			return fmt.Errorf("el esqueleto no pasó la verificación · %s · lo escrito sigue en %s", strings.Join(problemas, " · "), r.Path)
		}
		for _, p := range problemas {
			out.Line("  · %s", p)
		}
		prompt = esqueleto.PromptCorregir(original, problemas)
	}

	if salida, err := gitEn(r.Path, "add", "-A"); err != nil {
		return fmt.Errorf("no se pudo indexar el esqueleto · %s", strings.TrimSpace(salida))
	}
	// al retomar, el esqueleto puede estar ya commiteado: nada que agregar
	if _, err := gitEn(r.Path, "diff", "--cached", "--quiet"); err != nil {
		if salida, err := gitEn(r.Path, "-c", "user.name=devclean", "-c", "user.email=devclean@local", "commit", "--quiet", "-m", "wip: "+id+" esqueleto"); err != nil {
			return fmt.Errorf("no se pudo commitear el esqueleto · %s", strings.TrimSpace(salida))
		}
	}
	creados, err := gitEn(r.Path, "diff", "--name-only", r.Commit, "HEAD")
	if err != nil {
		return fmt.Errorf("no se pudo listar lo que creó el esqueleto · %s", strings.TrimSpace(creados))
	}
	// la suite que la esclusa de salida y la integración van a exigir
	if strings.TrimSpace(cfg.Pruebas) == "" && res.Pruebas != "" {
		cfg.Pruebas = res.Pruebas
		if err := cfg.Save(root); err != nil {
			return err
		}
		out.Line("· pruebas del proyecto · %s", res.Pruebas)
	}
	if err := state.Save(root, state.State{ID: id, Estado: state.Lista, Rama: r.Rama, Puerto: r.Puerto, Commit: r.Commit}); err != nil {
		return err
	}

	bs := res.Tareas
	// sin patrones de prueba: con el plano, cada tarea escribe su prueba
	sanearAlcance(bs, zonas, []string{}, pctx.Ocupados)
	idsRelleno, err := idsCorrelativos(config.TasksDir(root), len(bs)+1)
	if err != nil {
		return err
	}
	idsRelleno = idsRelleno[1:]
	traducirDependencias(bs, idsRelleno, idsPrevios(config.TasksDir(root)))
	intentos := s.Limites.Intentos
	if intentos < 1 {
		intentos = task.DefaultLimiteIntentos
	}
	s.Tasks = append(s.Tasks, task.Task{
		Version: task.Version, ID: id, Titulo: "esqueleto · " + s.Feature,
		Porque:      "estructura, interfaces, stubs y pruebas que cada tarea rellena",
		ListoCuando: res.Verificar, TocarSolo: strings.Fields(creados), Peso: "pesada",
		Notas: res.Arquitectura, LimiteIntentos: intentos,
	})
	for i, b := range bs {
		// las firmas viven en el código y las valida el compilador: sin
		// expone/usa en prosa no hay nada que comparar a mano
		s.Tasks = append(s.Tasks, task.Task{
			Version: task.Version, ID: idsRelleno[i], Titulo: b.Titulo, Porque: b.Porque,
			ListoCuando: conVerificar(res.Verificar, b.ListoCuando), TocarSolo: b.TocarSolo, NoTocar: b.NoTocar,
			DependeDe: append([]string{id}, b.DependeDe...), Peso: b.Peso, Agente: b.Agente,
			Skills: b.Skills, Notas: b.Como + "\n\n" + notaPara(ctx, r.Path, b.TocarSolo),
			LimiteIntentos: intentos, LimiteLineas: s.Limites.Lineas,
		})
	}
	if res.Verificar != "" {
		s.Acceptance = append(s.Acceptance, spec.Acceptance{Criterion: "build y typecheck del proyecto integrado", Command: res.Verificar})
	}
	// las imágenes se construyen sobre lo integrado: ahí aparece lo que
	// solo falla dentro del contenedor (una dependencia que no existe, un
	// archivo que .dockerignore deja fuera)
	if !sinDocker {
		if _, err := exec.LookPath("docker"); err == nil {
			s.Acceptance = append(s.Acceptance, spec.Acceptance{Criterion: "las imágenes de docker compose construyen", Command: "docker compose build"})
		} else {
			out.Line("· docker no está instalado · la entrega no va a construir las imágenes del compose")
		}
	}
	if res.Integracion != "" {
		s.Acceptance = append(s.Acceptance, spec.Acceptance{Criterion: "flujo de punta a punta del esqueleto", Command: res.Integracion})
	}
	out.Line("· esqueleto %s listo · %d tareas de relleno · integración: %s", id, len(bs), valorO(res.Integracion, "sin prueba de punta a punta"))
	return nil
}

// conVerificar antepone el build/typecheck del esqueleto al listo_cuando
// de una tarea de relleno. La prueba sola no basta: en closet vitest
// pasaba las 29 tareas y la integración, y `npm run build` (tsc) tenía 7
// errores de tipos que nadie corrió. Con esto el agente los ve y los
// arregla en su intento, no el humano al levantar la app.
func conVerificar(verificar, cmd string) string {
	if strings.TrimSpace(verificar) == "" || strings.TrimSpace(cmd) == "" {
		return cmd
	}
	return verificar + " && (" + cmd + ")"
}

func valorO(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}

func rutaRespuestaEsqueleto(root, id string) string {
	return filepath.Join(loop.RunsDir(root), id, "esqueleto-respuesta.json")
}

// respuestaEsqueleto es la última respuesta del arquitecto para este
// mismo pedido, "" si no hay o si el spec cambió desde entonces.
func respuestaEsqueleto(root, id, pedido string) string {
	b, err := os.ReadFile(rutaRespuestaEsqueleto(root, id))
	if err != nil {
		return ""
	}
	var g struct{ Prompt, Texto string }
	if json.Unmarshal(b, &g) != nil || g.Prompt != fmt.Sprintf("%x", sha256.Sum256([]byte(pedido))) {
		return ""
	}
	return g.Texto
}

func guardarRespuestaEsqueleto(root, id, pedido, texto string) {
	b, _ := json.Marshal(struct{ Prompt, Texto string }{fmt.Sprintf("%x", sha256.Sum256([]byte(pedido))), texto})
	_ = os.WriteFile(rutaRespuestaEsqueleto(root, id), b, 0o644)
}

// guardarLogEsqueleto deja el prompt y la salida del arquitecto junto a
// los intentos de la tarea: sin eso un esqueleto fallido no se depura.
func guardarLogEsqueleto(root, id string, vuelta int, prompt string, res executor.Result) {
	dir := filepath.Join(loop.RunsDir(root), id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	contenido := fmt.Sprintf("=== esqueleto %d · %d tokens de entrada · %d de salida · $%.3f\n--- prompt\n%s\n--- respuesta\n%s\n--- stderr\n%s\n",
		vuelta+1, res.Tokens.Input, res.Tokens.Output, res.Tokens.CostUSD, prompt, res.Text, res.Stderr)
	_ = os.WriteFile(filepath.Join(dir, fmt.Sprintf("esqueleto-%d.log", vuelta+1)), []byte(contenido), 0o644)
}
