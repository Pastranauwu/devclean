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

	"github.com/Pastranauwu/devclean/internal/capturas"
	"github.com/Pastranauwu/devclean/internal/config"
	"github.com/Pastranauwu/devclean/internal/esqueleto"
	"github.com/Pastranauwu/devclean/internal/examiner"
	"github.com/Pastranauwu/devclean/internal/executor"
	"github.com/Pastranauwu/devclean/internal/loop"
	"github.com/Pastranauwu/devclean/internal/plan"
	"github.com/Pastranauwu/devclean/internal/room"
	"github.com/Pastranauwu/devclean/internal/skills"
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
const notaCambio = "Cambias código que ya funciona. Primero escribe la prueba del comportamiento nuevo en el archivo que corre listo_cuando, con los \"Casos:\" de arriba; después haz el cambio. No cambies firmas, nombres ni exportaciones que otros módulos usan salvo que el contrato lo pida, y las pruebas que ya existen tienen que seguir pasando. De una prueba que ya existía solo puedes cambiar lo que el contrato declara en \"Obsoleto:\"; el resto de sus casos se queda igual. Si un comentario del archivo describe lo que cambias, actualízalo o bórralo: no dejes comentarios que contradigan el código. El contexto está en " + esqueleto.Documento + "."

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
const notaRelleno = "Primero escribe la prueba del archivo que corre listo_cuando con los \"Casos:\" del contrato de cada stub (y los casos límite que el contrato nombre): una prueba por caso, sin variantes que prueben lo mismo; las dependencias de otros módulos van con fakes. Después rellena los cuerpos que lanzan \"" + esqueleto.Marca + "\" siguiendo la \"Idea:\" del contrato: el diseño ya está decidido, no lo cambies ni busques otro. No cambies firmas, nombres ni exportaciones: otros módulos ya dependen de ellas. El contrato está en el comentario de cada stub y en " + esqueleto.Documento + ". Al terminar, el comentario de contrato deja de ser una instrucción para ti y pasa a ser documentación: redúcelo a lo que necesita quien mantenga el código (qué hace y lo que no es obvio) y borra \"Idea:\", \"Casos:\", la lista de quién llama a quién y cualquier maqueta o paso que el código ya dice; los casos viven en la prueba."

const notaRellenoExamen = "Un examinador independiente escribe la prueba visible desde las firmas y Casos: del esqueleto antes de que empieces. No edites pruebas: implementa los stubs que lanzan \"" + esqueleto.Marca + "\" siguiendo la Idea: del contrato y haz pasar listo_cuando. Mantén firmas y exportaciones. Al terminar, reduce los comentarios de contrato a documentación útil y borra Idea: y Casos:."

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
	// cómo se ve hoy, si ya se sabe levantar: el arquitecto planea un
	// cambio visual contra la pantalla, no contra el código
	antes := tomarCapturas(ctx, r.Path, cfg.Pantallas, r.Puerto, filepath.Join(loop.RunsDir(root), id, "antes"))
	original := esqueleto.Prompt(esqueleto.Pedido{Texto: pedido, Previo: previo, PrimerID: ids[1], SinDocker: sinDocker, Capturas: antes}, pctx)

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
			// sin script, la revisión visual solo ve cada ruta recién
			// abierta: en closet el formulario que se cambiaba solo aparece
			// después de elegir una foto, y nadie lo vio nunca
			if p, _ := cfg.Pantallas.Completar(res.Pantallas); !p.Vacia() && p.Script == "" && tocaUI(res.Tareas) {
				problemas = append(problemas, "hay tareas de interfaz y falta \"pantallas.script\": escribe un script que recorra con un navegador los flujos que cambian (p. ej. elegir una foto y ver el formulario) y guarde un PNG por paso en $CAPTURAS; sin él la revisión visual no ve esas pantallas")
			}
			// un script roto le muestra al revisor una página de error y le
			// hace rechazar tareas que no lo son; se prueba antes de repartir
			if p, _ := cfg.Pantallas.Completar(res.Pantallas); len(problemas) == 0 && p.Script != "" && !p.Vacia() && capturas.Navegador() != "" {
				dir := filepath.Join(loop.RunsDir(root), id, fmt.Sprintf("flujos-prueba-%d", vuelta))
				if err := capturas.ProbarFlujos(ctx, r.Path, p, r.Puerto, room.Entorno(r.Path), dir); err != nil {
					problemas = append(problemas, "\"pantallas.script\" no funciona contra la app levantada (revisa rutas, selectores y que el navegador que usa esté instalado): "+err.Error())
				}
			}
			if c := cicloDelPlan(root, res.Tareas); c != nil {
				problemas = append(problemas, "dependencia circular en depende_de: "+strings.Join(c, " → ")+" · quita una de esas dependencias")
			}
			// "no hay nada que cambiar" contra un pedido lo confirma quien
			// solo mira la pantalla. En closet el arquitecto vio los inputs
			// de archivo sin estilo, dijo que las capturas estaban viejas y
			// le creyó al código dos veces seguidas
			if len(problemas) == 0 && len(res.Tareas) == 0 && len(antes) > 0 {
				if rv, ok := revisorVisualPara(root, cfg, ex).(revisorVisualEnBucle); ok {
					cumple, cambios, tk := rv.juzgar(ctx, r.Path, antes, pedido)
					reg.Registrar(rv.ex.Name(), tk.Gasto())
					if !cumple {
						problemas = append(problemas, "dijiste que no hay nada que cambiar, pero una revisión de las capturas actuales (son del código de ahora) dice que no cumple el pedido:\n"+cambios)
					}
				}
			}
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

	if len(res.Tareas) == 0 {
		// el arquitecto no encontró nada que cambiar: lo que haya tocado
		// en el cuarto (ARCHITECTURE.md) no se entrega sin tareas
		_ = room.Destroy(ctx, root, id)
		out.Line("· el arquitecto no encontró nada que cambiar · el código ya cumple el pedido")
		return nil
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
	// cómo levantar y recorrer la interfaz: lo usan el revisor visual y
	// la entrega. Lo declarado en config manda; lo nuevo se suma
	if p, cambio := cfg.Pantallas.Completar(res.Pantallas); cambio {
		cfg.Pantallas = p
		if err := cfg.Save(root); err != nil {
			return err
		}
		flujos := ""
		if p.Script != "" {
			flujos = " · con script de flujos"
		}
		out.Line("· pantallas del proyecto · %s · %d rutas%s", p.Levantar, len(p.Rutas), flujos)
	}
	// la suite que la esclusa de salida y la integración van a exigir
	if strings.TrimSpace(cfg.Pruebas) == "" && res.Pruebas != "" {
		cfg.Pruebas = res.Pruebas
		if err := cfg.Save(root); err != nil {
			return err
		}
		out.Line("· pruebas del proyecto · %s", res.Pruebas)
	}
	// un plan de solo cambios a código existente puede no tocar nada en
	// el esqueleto: sin cambios no hay tarea de esqueleto que entregar ni
	// de la que depender (closet: T-055 vacío frenó la entrega)
	vacio := strings.TrimSpace(creados) == ""
	if vacio {
		_ = room.Destroy(ctx, root, id)
	} else if err := state.Save(root, state.State{ID: id, Estado: state.Lista, Rama: r.Rama, Puerto: r.Puerto, Commit: r.Commit}); err != nil {
		return err
	}

	bs := res.Tareas
	// sin patrones de prueba: con el plano, cada tarea escribe su prueba
	sanearAlcance(bs, zonas, []string{}, pctx.Ocupados)
	// la interfaz la hace el modelo pesado: en closet 9 tareas de modelos
	// baratos dieron parches sin diseño, y el arquitecto sigue marcando
	// "media" aunque el prompt pida "pesada"
	for i := range bs {
		if skills.TocaUI(bs[i].TocarSolo) {
			bs[i].Peso = "pesada"
		}
	}
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
	var deEsqueleto []string
	if !vacio {
		deEsqueleto = []string{id}
		s.Tasks = append(s.Tasks, task.Task{
			Version: task.Version, ID: id, Titulo: "esqueleto · " + s.Feature,
			Porque:      "estructura, interfaces, stubs y pruebas que cada tarea rellena",
			ListoCuando: res.Verificar, TocarSolo: strings.Fields(creados), Peso: "pesada",
			Notas: res.Arquitectura, LimiteIntentos: intentos,
		})
	}
	for i, b := range bs {
		// las firmas viven en el código y las valida el compilador: sin
		// expone/usa en prosa no hay nada que comparar a mano
		pruebaVisible := pruebaVisibleDeTarea(b.TocarSolo, b.ListoCuando)
		examenCiego := esqueleto.ConStub(ctx, r.Path, b.TocarSolo) && !strings.Contains(b.Como, "Obsoleto:") &&
			pruebaVisible != "" && examiner.Examinable(r.Path, task.Task{TocarSolo: b.TocarSolo, ExamenEsqueleto: true, ExamenVisible: pruebaVisible}, config.DetectLanguage(r.Path))
		alcance := b.TocarSolo
		nota := notaPara(ctx, r.Path, b.TocarSolo)
		if examenCiego {
			alcance = sinPruebasDeTarea(b.TocarSolo)
			nota = notaRellenoExamen
		}
		s.Tasks = append(s.Tasks, task.Task{
			Version: task.Version, ID: idsRelleno[i], Titulo: b.Titulo, Porque: b.Porque,
			ListoCuando: conVerificar(res.Verificar, b.ListoCuando), TocarSolo: alcance, NoTocar: b.NoTocar,
			DependeDe: append(append([]string(nil), deEsqueleto...), b.DependeDe...), Peso: b.Peso, Agente: b.Agente,
			Skills: b.Skills, Cubre: b.Cubre, Notas: b.Como + "\n\n" + nota,
			ExamenEsqueleto: examenCiego,
			ExamenVisible:   pruebaVisible,
			LimiteIntentos:  intentos, LimiteLineas: s.Limites.Lineas,
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

// pruebaVisibleDeTarea es el archivo de prueba de tocar_solo que corre
// listo_cuando. En un monorepo el comando lo nombra relativo a un `cd` o
// a un --prefix ("cd backend && pytest tests/test_x.py" para
// backend/tests/test_x.py): vale si el comando trae el resto de la ruta y
// también la carpeta que le falta.
func pruebaVisibleDeTarea(alcance []string, comando string) string {
	for _, p := range alcance {
		if !task.EsArchivoDePrueba(p) || strings.ContainsAny(p, "*?[") {
			continue
		}
		if strings.Contains(comando, p) {
			return p
		}
		partes := strings.Split(p, "/")
		for i := 1; i < len(partes); i++ {
			carpeta, resto := strings.Join(partes[:i], "/"), strings.Join(partes[i:], "/")
			if strings.Contains(comando, resto) && strings.Contains(comando, carpeta) {
				return p
			}
		}
	}
	return ""
}

func sinPruebasDeTarea(alcance []string) []string {
	var out []string
	for _, p := range alcance {
		if !task.EsArchivoDePrueba(p) {
			out = append(out, p)
		}
	}
	return out
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
	if b, err := json.Marshal(res.Tokens); err == nil {
		_ = os.WriteFile(filepath.Join(dir, fmt.Sprintf("esqueleto-%d.usage.json", vuelta+1)), b, 0o644)
	}
}

// tomarCapturas fotografía la interfaz si el proyecto declara cómo
// levantarla. Degrada en abierto: sin navegador (y sin poder instalarlo)
// o si la app no levanta, avisa y sigue sin capturas.
func tomarCapturas(ctx context.Context, dir string, p capturas.Pantallas, puerto int, outDir string) []string {
	if p.Vacia() {
		return nil
	}
	if capturas.Navegador() == "" {
		out.Line("· instalando chrome-headless-shell para las capturas (una vez por máquina)")
		if err := capturas.Instalar(ctx); err != nil {
			out.Line("· sin capturas · %s", err)
			return nil
		}
	}
	fotos, err := capturas.Tomar(ctx, dir, p, puerto, room.Entorno(dir), outDir)
	if err != nil {
		out.Line("· sin capturas · %s", err)
		return nil
	}
	return fotos
}

// cicloDelPlan traduce las dependencias del plan a los ids que van a
// tener sus tareas y busca un ciclo, sin tocar el plan.
func cicloDelPlan(root string, bs []plan.Borrador) []string {
	ids, err := idsCorrelativos(config.TasksDir(root), len(bs)+1)
	if err != nil || len(ids) < 2 {
		return nil
	}
	copia := make([]plan.Borrador, len(bs))
	for i, b := range bs {
		copia[i] = b
		copia[i].DependeDe = append([]string(nil), b.DependeDe...)
	}
	traducirDependencias(copia, ids[1:], idsPrevios(config.TasksDir(root)))
	return ciclo(copia, ids[1:])
}

func tocaUI(bs []plan.Borrador) bool {
	for _, b := range bs {
		if skills.TocaUI(b.TocarSolo) {
			return true
		}
	}
	return false
}
