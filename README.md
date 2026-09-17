# devclean

Dirige varios agentes de IA programando **en paralelo** sobre el mismo repositorio
y deja pasar solo lo que compila, pasa pruebas y tiene historial legible.

Tú dices qué necesitas. devclean lo parte en tareas con un comando que decide
"ya está", reparte cada una a un agente en su propio `git worktree`, revierte lo
que se salga del alcance, y al final te entrega **un pull request** — no el
desorden ni los intentos fallidos.

```
$ devclean up "dos utilidades: formatear centavos a pesos, y validar un RFC" --agentes 2 --ship
· sin remoto origin · el PR queda local: la rama y su descripción en .devclean/pr/
propongo 2 tareas:
T-001  formatear centavos a pesos mexicanos [ejecutor]  · 60 líneas · listo cuando: go test ./moneda/...
T-002  validar rfc de persona fisica [ejecutor]  · 80 líneas · listo cuando: go test ./rfc/...
▸ T-001  intento 1 · haiku trabajando
▸ T-002  intento 1 · haiku trabajando
✓ T-001  formatear centavos a pesos mexicanos  · verde en 1 intento · 6.1k tokens
✓ T-002  validar rfc de persona fisica  · verde en 1 intento · 13.5k tokens

✓ esclusa T-001  · lista para integrar
✓ esclusa T-002  · lista para integrar
✓ rama de entrega  · devclean/_entrega desde main
✓ integrar  · 2 commits, uno por tarea
✓ integradas  · go test ./...
✓ pr  · local · devclean/_entrega · .devclean/pr/_entrega.md
entregado · 2 tareas en un PR · local · devclean/_entrega · .devclean/pr/_entrega.md
· revisa · git log -p main..devclean/_entrega
· integra · git merge --ff-only devclean/_entrega
```

Salida real de una corrida con `--plain`, sin líneas de progreso repetidas.

![demo](docs/demo.gif)

---

## Qué hace, en concreto

1. **Parte tu frase en contratos de tarea.** Cada uno lleva un comando ejecutable
   (`listo_cuando`) que **tiene que fallar hoy** y pasar cuando la tarea esté
   hecha. Sin ese comando, la tarea no existe.
2. **Cierra las interfaces antes de repartir.** Si dos tareas paralelas comparten
   un tipo, una lo declara en `expone` y la otra lo copia en `usa`: nadie inventa
   una firma que la otra no va a implementar.
3. **Un cuarto por tarea.** `git worktree` en `.devclean/rooms/T-00N/`. Lo que el
   agente edite fuera de su `tocar_solo` se revierte antes de verificar, incluso
   si lo commiteó por su cuenta.
4. **Verifica con código, nunca con el modelo.** El agente no decide si terminó:
   lo decide el código de salida de `listo_cuando`. Un comando que sale con 0 sin
   ejecutar pruebas tampoco cuenta como verde.
5. **Escribe el examen antes que la implementación.** En Go y Python un
   examinador ciego redacta las pruebas mirando solo la frontera pública, y sella
   el 30% fuera del alcance del agente hasta la entrega.
6. **Entrega en fila.** Diez pasos deterministas (rebase, historial aplanado,
   ruido, secretos, presupuesto, interfaces, dependencias, bisectable, examen
   oculto, handoff) y recién entonces el PR. El primer paso que falla frena la
   compuerta con la razón exacta.

Lo que **no** hace: no trae modelos, no vota en comité, no verifica interfaz
gráfica ni criterios difusos, y no abre nada que no haya pasado la compuerta.

## Instalación

```sh
curl -fsSL https://github.com/Pastranauwu/devclean/releases/latest/download/install.sh | sh
```

Deja el binario en `~/.local/bin`. Alternativa: `go install github.com/Pastranauwu/devclean/cmd/devclean@latest`.

**devclean no trae ningún modelo.** Dirige un CLI de agente que ya tienes y pagas:
[Claude Code](https://docs.anthropic.com/claude-code) (`claude`) u
[OpenCode](https://opencode.ai) (`opencode`). Necesitas uno instalado y logueado,
y `git`. Para PRs en GitHub, además `gh`; sin remoto `origin` el PR es local y no
hace falta. `devclean doctor` te dice qué falta.

## Empieza en tres comandos

```sh
cd tu-proyecto
devclean up "exportar clientes a csv" --agentes 3   # planea y ejecuta en paralelo
devclean board                                       # cómo va
devclean ship --todas                                # esclusa de salida y PR
```

No hay que configurar nada antes: `up` crea el repo si falta, detecta rama base,
comando de pruebas, CLI y modelos, y pregunta solo lo que no puede resolver solo.
Todo lo que decide queda en `.devclean/config.yml`, editable a mano.

Si quieres las tres cosas en una:

```sh
devclean up "exportar clientes a csv" --agentes 3 --ship
```

Variantes que vas a usar:

```sh
devclean up "…" --revisar        # además un modelo revisa el diff y deja informe en el PR
devclean up "…" --integrar       # además mergea si el revisor no pide cambios
devclean up "…" --fondo          # se desprende de la terminal; puedes cerrarla
devclean up --agentes 4          # sin petición: corre las tareas pendientes
devclean run --reintentar        # revive las detenidas reusando su cuarto y trabajo parcial
```

## La forma fácil en YAML: una línea por tarea

Cuando quieras versionar el plan o repartir algo grande, escribe
`devclean.spec.yml` en la raíz. Tú decides **qué**; el planificador escribe el
contrato:

```yaml
feature: "notas en markdown"
agentes: 2
tareas:
  - guardar y leer notas en un json
  - buscar notas por palabra
```

```sh
devclean apply --dry-run   # valida y dice qué va a completar, sin gastar tokens
devclean apply             # escribe los contratos; revísalos con devclean board
devclean up                # encuentra el spec solo, ejecuta y entrega si dice ship: true
```

De esas dos líneas sale un contrato completo, con el comando que lo juzga, el
alcance, las firmas y los riesgos:

```yaml
---
id: T-001
titulo: guardar y leer notas en un json
porque: sin guardar notas no hay nada que buscar; es la base del feature
listo_cuando: go test ./internal/notas/...
tocar_solo: ["internal/notas/**"]
expone: ["notas.Guardar(ruta string, ns []notas.Nota) error", "notas.Leer(ruta string) ([]notas.Nota, error)"]
riesgos: si el archivo no existe, Leer debe devolver lista vacía sin error
peso: liviana
agente: backend
---
un solo archivo internal/notas/notas.go con encoding/json y os.ReadFile/os.WriteFile
```

Lo que tú escribas se respeta tal cual; el planificador solo llena los huecos. Así
que puedes mezclar una línea suelta con una tarea detallada en el mismo archivo:

```yaml
feature: "wake on lan con alexa"
agentes: 3
ship: true
tareas:
  - enviar magic packet por udp
  - guardar la mac en json
  - titulo: endpoint de la skill alexa
    listo_cuando: go test ./internal/alexa/...
    tocar_solo: ["internal/alexa/**"]
```

## Comandos

Los que usarás todos los días:

| Comando | Qué hace |
|---|---|
| `up "<petición>" [--agentes N] [--ship]` | Todo: configura, planea, ejecuta y entrega. |
| `board` | Tablero por estado: listas, en curso, detenidas, pendientes. |
| `logs T-001` | Qué probó cada intento y con qué falló. |
| `ship T-001` / `ship --todas` | Esclusa de salida y PR. `--dry-run` hace todo menos publicar. |
| `run --reintentar` | Revive las detenidas reusando su cuarto y su trabajo parcial. |
| `doctor` | Verifica git, config, CLIs, keys y que los modelos existan. |

El resto, cuando te haga falta:

| Comando | Qué hace |
|---|---|
| `plan "<petición>" [--aprobar]` | Solo planea. `--export-spec archivo.yml` lo guarda como spec. |
| `apply [-f archivo] [--run\|--dry-run]` | Crea tareas desde un spec declarativo. |
| `ps` | Tareas y cuartos activos, estilo compose. |
| `standup` | Qué avanza, qué colisiona, qué está atascado. Datos, sin debates. |
| `report` | Las cinco métricas con tendencia respecto a la corrida anterior. |
| `usage` | Gasto por ventanas (5h, semanal, mensual) contra el presupuesto. |
| `init [--cli claude]` | Crea `.devclean/` a mano, eligiendo CLI y modelos. |
| `task add\|edit\|rm\|list\|check\|seal` | Contratos a mano. `seal` sella tus propias pruebas ocultas. |
| `constitution` | Genera `.devclean/constitution.md`, que va en cada prompt. |
| `skills sync` | Trae las skills que los agentes inyectan en su prompt. |

Todos aceptan `--plain` (una línea por evento, para CI y tuberías) y `--json`.
Sin ellos y en terminal, usan la interfaz interactiva.

### Dejarlo trabajando y volver

```
$ devclean up "migra la API a la versión 2" --agentes 4 --ship --fondo
corriendo en segundo plano · pid 41287
registro · .devclean/corridas/2026-09-10T18-02-29.log
parar · kill 41287 · retomar después · devclean run --reintentar
```

Las preguntas se hacen **antes** de desprenderse. Si la corrida muere de verdad
—suspendiste la laptop, se cayó el ssh— `devclean board` lo dice
(`interrumpida · sin señal hace X`) en vez de fingir que sigue viva.

## El contrato de tarea

Un archivo por tarea en `.devclean/tasks/T-001.md`. Solo `titulo` y
`listo_cuando` son obligatorios:

```yaml
---
version: 1
id: T-001
titulo: exportar clientes a CSV
porque: soporte pierde 3h/semana copiando a mano
listo_cuando: npm test -- export.spec.ts     # OBLIGATORIO, ejecutable, falla hoy
tocar_solo: ["src/export/**"]                # lo que puede editar; lo demás se revierte
no_tocar: ["src/auth/**"]                    # prohibido explícito
depende_de: ["T-000"]                        # ids que deben estar verdes antes
expone: ["export.ToCSV(rows []Row) []byte"]  # firmas que otras tareas consumen
usa: ["config.Load(p string) error"]         # firmas de otras, copiadas igual
peso: liviana                                # liviana | media | pesada → elige modelo
agente: backend                              # arquetipo o agente de config.yml
limite_intentos: 3
limite_lineas: 200
---
El cuerpo libre es el enfoque para quien implementa: por dónde empezar,
a qué no meterse. Se inyecta en el prompt de cada intento.
```

### Spec completo

Todo el contrato cabe en el spec, más los ajustes de la corrida. Esta plantilla,
tal cual, ya parsea:

```yaml
version: 1                            # obligatorio en modo completo
feature: ""                           # qué se construye · es el título del PR
agentes: 1                            # tareas en paralelo · el flag --agentes gana
ship: false                           # true = al terminar abre el PR
agente: ""                            # arquetipo por defecto de las tareas
limites: { intentos: 3, lineas: 200 } # topes por defecto de cada tarea
reglas: []                            # se anteponen a las notas de CADA tarea
tasks:
  - id: ""                            # lo asigna devclean si lo dejas vacío
    titulo: ""                        # OBLIGATORIO
    porque: ""                        # para qué · lo lee el revisor
    listo_cuando: ""                  # OBLIGATORIO · falla hoy, pasa al terminar
    tocar_solo: []                    # globs que el agente puede editar
    no_tocar: []                      # globs prohibidos
    depende_de: []                    # ids que deben estar verdes antes
    expone: []                        # firmas que otras tareas consumen
    usa: []                           # firmas de otras tareas, copiadas igual
    riesgos: ""                       # qué se puede romper
    peso: ""                          # liviana | media | pesada
    agente: ""                        # arquetipo o agente de config.yml
    limite_intentos: 0                # 0 = hereda de limites
    limite_lineas: 0                  # 0 = hereda de limites
    notas: ""                         # el enfoque · va en el prompt de cada intento
```

Un campo que no esté en esta lista es un error de parseo, no un campo ignorado:
`devclean apply --dry-run` te da el número de línea. Sin ids escritos,
`depende_de: ["T-001"]` apunta a la primera tarea **de ese spec**, sin chocar con
las que ya existan en el repo. Los flags de la línea de comandos siempre ganan.

## Configuración

`up` la escribe sola en `.devclean/config.yml`:

```yaml
base: main
pruebas: go test ./...
cli: claude                      # claude | opencode
modelos:                         # por peso de tarea; ids reales del CLI
  liviana: haiku
  media: sonnet
  pesada: opus
estrategia: equilibrada          # ligera | equilibrada | pesada (peso por defecto)
timeout_agente: 1200             # segundos por invocación del agente
timeout_pruebas: 300             # segundos por corrida de pruebas
presupuesto_tokens: 0            # tope por corrida; 0 = sin tope
presupuesto:                     # tope por ventana rodante y proveedor
  claude: { 5h: 40000, semanal: 120000 }
zonas_prohibidas: ["go.sum", "migrations/**", ".github/**"]
patrones_prueba: ["*_test.go", "test/**", "*.spec.ts"]
agentes:                         # arquetipos propios o sobreescritos
  specialist: { provider: claude, model: sonnet, skills: ["python", "ml"] }
reglas_import: ["api → dominio → datos"]
recursion_max: 0                 # tareas que se reparten en subtareas; 0 = apagado
```

Arquetipos listos para `agente:`: `ejecutor`, `backend`, `frontend`, `architect`,
`tester`, `refactor`.

## Qué cuesta

Medido con `claude` y modelos livianos, en proyectos chicos de Go:

| Trabajo | Tokens | Intentos |
|---|---|---|
| Una utilidad con su suite ciega | 1.5k – 7k | 1–2 |
| Una tarea que consume 4 paquetes hermanos (un CLI) | ~30k | 1 |
| Cinco tareas en paralelo (lexer → parser → eval → cli) | ~48k | 1 cada una, salvo una con 2 |

Sube el peso y sube el gasto: un `peso: pesada` cuesta bastante más por intento.
Si una tarea queda roja con el modelo barato, devclean escala al siguiente
escalón reusando su trabajo, así que una tarea que se atasca puede costar varios
intentos. `presupuesto:` corta la corrida antes de agotar tu cuota, y
`devclean usage` te dice en qué va la ventana.

## Cuando algo falla

- **Tarea detenida.** Agotó sus intentos. `devclean logs T-00N` muestra qué probó
  y qué falló. Corrige el contrato o sube `limite_intentos`, y
  `devclean run --reintentar`.
- **Rechazada en la esclusa de entrada.** Casi siempre "`listo_cuando` ya pasa":
  el comando tiene que fallar hoy. Apunta a lo que la tarea va a crear, no a la
  suite entera.
- **"`listo_cuando` pasó sin ejecutar ninguna prueba".** Salió con 0 pero no
  corrió nada: `go test ./pkg/...` sobre un paquete sin archivos de prueba
  devuelve 0. Un verde así no prueba nada, así que la tarea se detiene en vez de
  entregarse. Sella tu suite con `devclean task seal T-00N` o apunta
  `listo_cuando` a pruebas que existan.
- **"la suite oculta no compila".** El examen sellado quedó inservible; el motivo
  nombra la ruta a borrar. La salida completa queda en
  `.devclean/runs/<id>/suite-oculta.log`.
- **`ship` frenado.** Dice el paso y la razón exacta. Nada se publica hasta que
  pase. El examen oculto que falla **no** se consume: el siguiente `ship` lo
  vuelve a correr.
- **Presupuesto excedido.** `limite_lineas` lo estima el planificador antes de que
  exista el código, así que se aplica con tolerancia y solo sobre el código de la
  solución; las pruebas se cuentan aparte. El mensaje trae el número exacto a
  poner y dónde.

## Seguridad

- Nada escucha en la red. La única llamada HTTP directa es la sonda opcional de
  `devclean usage --sonda`.
- El agente solo escribe dentro de su cuarto y de su `tocar_solo`; lo demás se
  revierte, aunque lo haya commiteado.
- Las keys nunca entran al prompt ni a los logs.
- Escaneo de secretos y de ruido obligatorio antes de cualquier PR.
- `devclean ship --dry-run` muestra todo antes de publicar.

## Límite honesto

devclean sirve para lo que tiene oráculo: un comando que decide verde o rojo.

- **Nadie prueba la costura entre dos tareas.** Cada `listo_cuando` juzga su
  tarea, y la detección de solapamiento corre esos mismos comandos. Si el
  contrato de una tarea no pide algo que su consumidora sí promete, las dos salen
  verdes y la integración queda incompleta — visto en una calculadora donde el
  lexer y el parser tenían el operador `^` y el evaluador nunca lo pidió. Si el
  feature se integra, agrega una tarea final cuyo `listo_cuando` pruebe el camino
  completo.
- **El examinador ciego solo cubre Go y Python**, y solo donde hay una frontera
  importable: una tarea sobre un `package main` (`cmd/algo`) no se puede examinar,
  así que ahí las pruebas las escribe quien implementa. En node y rust, igual.
- **La suite oculta depende del examinador.** Si el modelo no devuelve el bloque
  oculto, no hay 30% sellado y el paso se omite. La suite visible sí se escribe.
- **`listo_cuando` tiene que fallar hoy.** Es lo que hace que una tarea signifique
  algo. Por eso `npm test` a secas no sirve en un repo verde.
- **Los agentes son los tuyos.** Si el CLI está sin cuota o sin login, la corrida
  falla ahí; devclean te lo dice, no lo arregla.
- **La suite oculta se esconde, no se blinda.** Vive en `.devclean/sealed/` del
  repo principal, fuera del cuarto del agente, con un hash que detecta corrupción
  accidental. No es a prueba de manipulación: el hash viaja en el mismo archivo
  que el contenido. Lo que separa al agente del examen es que trabaja en otro
  directorio, no un sandbox. Vale contra un modelo que se desvía, no contra uno
  que ataca.
- **El parser YAML es propio y minimalista** (`internal/kv`): no maneja jerarquías
  anidadas complejas, y dos claves con el mismo nombre a distinta profundidad se
  pisan.

## Licencia

MIT.
