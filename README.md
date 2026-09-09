# devclean

Dirige agentes de IA programando en paralelo sobre un mismo repositorio y
garantiza que lo único que llega al proyecto sea **código limpio, probado y
con historial legible**.

Tú dices qué quieres. devclean configura, planea, reparte, supervisa, prueba y
entrega. Recibes un pull request, no el desorden.

```
$ devclean up "exportar clientes a CSV y arreglar el login con tildes" --agentes 3 --ship
  · sin .devclean · configurando
  ✓ ejecutor: claude · modelos: haiku / sonnet / opus
  T-001  exportar clientes a CSV       listo cuando: npm test -- export
  T-002  login acepta tildes           listo cuando: npm test -- auth
  T-001  ✓ verde en 2 intentos
  T-002  ✓ verde en 1 intento
  ✓ base · ✓ historial · ✓ ruido · ✓ secretos · ✓ presupuesto · ✓ interfaces · ✓ bisectable · ✓ integradas · ✓ pr
  entregado · https://github.com/tu/repo/pull/142
```

![demo](docs/demo.gif)

---

## Instalación

```sh
curl -fsSL https://github.com/Pastranauwu/devclean/releases/latest/download/install.sh | sh
```

Deja el binario en `~/.local/bin` y lo agrega al `PATH`. Alternativas:

```sh
go install github.com/Pastranauwu/devclean/cmd/devclean@latest
```

**devclean no trae ningún modelo.** Dirige un CLI de agente que ya tienes
instalado y pagas: [Claude Code](https://docs.anthropic.com/claude-code)
(`claude`) u [OpenCode](https://opencode.ai) (`opencode`). Necesitas al menos
uno instalado y logueado, y `git`. Para abrir PRs, además `gh`.

## Uso en una línea

Entra a la carpeta del proyecto (con o sin git, con o sin código) y pide:

```sh
devclean up "api rest de tareas con sqlite" --agentes 3 --ship
```

No hay nada que configurar antes. `up` es el orquestador y resuelve lo que
falte:

| Falta | Qué hace `up` |
|---|---|
| repo git | `git init` |
| `.devclean/` | lo crea, detecta rama base, comando de pruebas, CLI y modelos reales |
| commits | hace el inicial si lo único sin versionar es lo suyo; si hay archivos tuyos, pregunta |
| el CLI configurado no está instalado | usa el que sí esté |
| un modelo que el CLI no reconoce | lo reasigna del catálogo real |
| `gh` o remoto `origin` (con `--ship`) | pide la URL en terminal; sin terminal, corta **antes** de gastar un token |
| ningún CLI de agente | ofrece instalar `claude` con npm; sin terminal, corta con la instrucción |

Con dos CLIs instalados pregunta cuál usar una sola vez. Todo lo que decide
queda escrito en `.devclean/config.yml`, editable a mano.

Variantes:

```sh
devclean up "arreglar el login con tildes"              # planea y ejecuta; tú revisas y haces ship
devclean up "…" --ship                                    # además entrega todo en un solo PR
devclean up "…" --revisar                                 # además un modelo revisa el diff y deja informe en el PR
devclean up "…" --integrar                                # además mergea si el revisor no pide cambios
devclean up --agentes 4                                   # sin petición: corre las tareas pendientes
devclean up -f devclean.spec.yml --ship                   # desde una especificación declarativa
```

`plan` y `run` sueltos hacen la misma preparación automática. `devclean init`
sigue existiendo para quien quiera elegir CLI y modelos a mano
(`devclean init --cli claude`).

## Cómo funciona

Cada tarea es un **contrato** con un comando ejecutable que dice "ya está"
(`listo_cuando`). Sin ese comando, la tarea no existe. La verificación la
hace código, nunca el modelo.

1. **Plan.** Un modelo (rol planificador) convierte tu frase en contratos de
   tarea en `.devclean/tasks/T-00N.md`. En terminal apruebas con casillas;
   `up` los aprueba solo.
2. **Esclusa de entrada.** Cada contrato tiene que ser válido, su
   `listo_cuando` tiene que **fallar hoy**, y su alcance no puede pisar el
   de otra tarea en curso. Lo que no pasa se rechaza con motivo, sin gastar
   un token.
3. **Bucle de intentos.** Cada tarea corre en un **cuarto aislado**
   (`git worktree` en `.devclean/rooms/T-00N/`). El agente edita dentro de
   `tocar_solo`; devclean revierte lo que se salió, corre `listo_cuando` y
   decide: verde, lista; rojo, le devuelve el error; agotados los intentos,
   se detiene con una pregunta concreta. Antes del primer intento un
   **examinador ciego** escribe pruebas contra la interfaz pública
   (`expone`) sin ver la implementación, y sella el 30% con hash (go y
   python). Cuando los tests dan verde, un **revisor** (rol `revisor`,
   modelo pesado por defecto) juzga el diff contra el contrato: si pide
   cambios, el intento queda rojo y su veredicto entra como contexto del
   siguiente — tests verdes no implican contrato cumplido.
4. **Esclusa de salida (`ship`).** Rebase sobre la base, historial aplanado
   en un commit por tarea, sin prints de debug, sin secretos, dentro del
   presupuesto de líneas, las interfaces prometidas están en el diff,
   bisectable, la suite completa pasa sobre el conjunto integrado, y hasta
   entonces el PR. El primer paso que falla frena la compuerta con la razón
   exacta.

Arriba de todo, una **constitución** (`.devclean/constitution.md`) va en cada
prompt para que dos tareas paralelas no elijan arquitecturas incompatibles.
`devclean standup` deriva colisiones y atascos de los artefactos, sin que los
agentes hablen entre sí.

## Comandos

| Comando | Qué hace |
|---|---|
| `up "<petición>" [--agentes N] [--ship\|--revisar\|--integrar]` | Todo: configura, planea, ejecuta y entrega. |
| `plan "<petición>"` | Solo planea. Muestra las tareas propuestas y las crea si apruebas. |
| `run [--agentes N] [--reintentar]` | Ejecuta las tareas pendientes en paralelo. `--reintentar` revive las detenidas reusando su cuarto. |
| `ship T-001` / `ship --todas` | Esclusa de salida y PR. `--dry-run` hace todo menos abrir el PR. |
| `board` | Tablero por estado: listas, en curso, detenidas, pendientes. |
| `ps` | Estado de tareas y cuartos activos. |
| `logs T-001` | Intentos de una tarea, uno por línea. |
| `standup` | Parte de datos: qué avanza, qué colisiona, qué está atascado. |
| `report` | Métricas con tendencia respecto a la corrida anterior. |
| `usage` | Gasto por ventanas (5h, semanal, mensual) contra el presupuesto. |
| `doctor` | Verifica git, config, CLIs, keys y que los modelos existan. |
| `init [--cli claude] [--pruebas "…"]` | Crea `.devclean/` a mano, eligiendo CLI y modelos. |
| `task add\|edit\|rm\|list\|check\|seal` | Contratos a mano. `seal` sella tus propias pruebas ocultas. |
| `apply spec.yml [--run\|--dry-run]` | Crea tareas desde una especificación declarativa. `--run` las ejecuta con los `agentes` del spec. |
| `constitution` | Genera la constitución del proyecto. |
| `skills sync` | Trae las skills que los agentes inyectan en su prompt. |

Todos aceptan `--plain` (una línea por evento) y `--json`. Sin ellos y en
terminal, usan la interfaz interactiva.

### Cuando algo falla

- **Tarea detenida.** Agotó sus intentos. `devclean logs T-00N` muestra qué
  probó y qué falló. Corrige el contrato si estaba mal, o sube
  `limite_intentos`, y `devclean run --reintentar`.
- **Rechazada en la esclusa de entrada.** El motivo más común es
  "`listo_cuando` ya pasa": el comando tiene que fallar hoy. Apunta a lo que
  la tarea va a crear, no a la suite entera.
- **`ship` frenado.** Dice el paso y la razón exacta. Nada se publica hasta
  que pase.
- **Presupuesto excedido.** `limite_lineas` lo estima el planificador antes de
  que exista el código, así que se aplica con tolerancia y solo sobre el
  código de la solución: las pruebas se cuentan aparte. Si aun así frena, el
  mensaje trae el número exacto a poner y en qué archivo.

## El contrato de tarea

Un archivo por tarea en `.devclean/tasks/T-001.md`:

```yaml
---
version: 1
id: T-001
titulo: exportar clientes a CSV
porque: soporte pierde 3h/semana copiando a mano
listo_cuando: npm test -- export.spec.ts     # OBLIGATORIO, ejecutable, falla hoy
tocar_solo: ["src/export/**"]
depende_de: ["T-000"]                        # ids que deben estar verdes antes
expone: ["export.ToCSV(rows []Row) []byte"]  # firmas que otras tareas consumen
usa: ["config.Load(p string) error"]         # firmas de otras, copiadas igual
peso: liviana                                # liviana | media | pesada → elige modelo
agente: backend                              # arquetipo o agente de config.yml
limite_intentos: 3
limite_lineas: 200
---
```

Solo `titulo` y `listo_cuando` son obligatorios; `plan` rellena el resto.
El cuerpo libre bajo el frontmatter (`notas`) lleva el **enfoque** que el
planificador deja al ejecutor (`como`): cómo encarar la tarea, qué tocar
primero, a qué no meterse. Se inyecta en el prompt de cada intento.

## Programación agéntica como código

Para features grandes, o para versionar el plan, **todo el trabajo cabe en
un `devclean.spec.yml`**: qué se hace, cómo se encara, cuándo se da por
hecho, con cuántos agentes y cuáles. `devclean up` lo encuentra solo en la
raíz del repo y hace el resto — aplicar, ejecutar en paralelo y entregar:

```yaml
version: 1
feature: "Autenticación de usuarios y JWT"   # qué se construye (y título del PR)
agentes: 3                    # cuántos trabajadores en paralelo (el flag --agentes gana)
ship: true                    # al terminar, entrega todo en un PR (equivale a --ship)
agente: backend               # arquetipo por defecto de las tareas
limites: { intentos: 5, lineas: 500 }
reglas:                       # se inyectan en el prompt de CADA tarea
  - "tokens stateless, sin sesiones en memoria"
tasks:
  - titulo: "modelo de usuario y hash de contraseñas"
    listo_cuando: "go test ./internal/auth/ -run TestPasswordHash"
    tocar_solo: ["internal/auth/**"]
    notas: "empieza por bcrypt; no toques el router"   # el cómo, para el ejecutor
  - titulo: "endpoint de login con JWT"
    listo_cuando: "go test ./internal/auth/ -run TestLogin"
    depende_de: ["T-001"]
    expone: ["POST /api/login -> 200 {token}"]
    peso: media               # liviana | media | pesada → elige modelo
```

Cada tarea acepta todos los campos del contrato (ver *El contrato de
tarea*): `tocar_solo`, `no_tocar`, `depende_de`, `expone`, `usa`, `peso`,
`agente`, `limite_intentos`, `limite_lineas` y `notas` (el enfoque que
recibe el agente en cada intento). Las `reglas` globales se anteponen a
las `notas` de cada tarea al aplicar.

| Comando | Qué hace |
|---|---|
| `devclean up` | encuentra `devclean.spec.yml` solo, lo aplica, corre con sus `agentes` y entrega si dice `ship: true` |
| `devclean up -f specs/auth.yml` | lo mismo con otra ruta |
| `devclean apply [-f archivo]` | crea las tareas sin ejecutar; las revisas con `devclean board` |
| `devclean apply --run` | aplica y ejecuta con los `agentes` del spec |
| `devclean apply --dry-run` | valida la especificación sin escribir nada |
| `devclean plan "…" --export-spec devclean.spec.yml` | genera un spec desde una frase, para editarlo a mano y versionarlo |

Los flags de la línea de comandos siempre ganan sobre el spec:
`devclean up --agentes 1` corre en serie aunque el spec diga `agentes: 4`.

## Configuración

`up` la escribe sola. Se edita a mano en `.devclean/config.yml` cuando quieres
otra cosa:

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
subagentes: 2
```

Arquetipos predefinidos para `agente:`: `ejecutor`, `backend`, `frontend`,
`architect`, `tester`, `refactor`. `devclean doctor` avisa si un modelo de la
config no existe en el CLI antes de gastar un token.

## Seguridad

- Nada escucha en la red. La única llamada HTTP directa es la sonda opcional
  de `devclean usage --sonda`.
- El agente solo escribe dentro de su cuarto y dentro de `tocar_solo`; lo
  demás se revierte.
- Las keys nunca entran al prompt ni a los logs.
- Escaneo de secretos y de ruido obligatorio antes de cualquier PR.
- `devclean ship --dry-run` muestra todo antes de publicar.

## Límite honesto

devclean sirve para lo que tiene oráculo: un comando que decide verde o rojo.
No verifica interfaz gráfica ni criterios difusos.

- **El examinador ciego solo existe para go y python.** En node y rust las
  pruebas las escribe quien implementa; el `listo_cuando` sigue siendo
  vinculante, pero sin la garantía de que el examen lo redactó alguien que no
  vio la implementación.
- **`listo_cuando` tiene que fallar hoy.** Es lo que hace que una tarea
  signifique algo. Por eso `npm test` a secas no sirve en un repo verde.
- **Los agentes son los tuyos.** Si el CLI está sin cuota o sin login, la
  corrida falla ahí; devclean te lo dice, no lo arregla.

## Licencia

MIT.
