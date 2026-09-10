# Estado del proyecto — traspaso entre sesiones

**Última actualización: 10 septiembre 2026.** Reescrito desde cero contra el
código, no heredado de versiones anteriores de este archivo.

- **HEAD:** `eea6d3b`, rama `main`, sincronizada con `origin/main`.
- **Última release publicada:** **v0.8.2** (10 sep 2026), etiquetada en HEAD.
  No hay commits después del tag.
- **Compila y pasa:** `go build ./...` limpio; `go test ./...` verde. 22
  paquetes con pruebas; tres sin ninguna: `internal/constitution`,
  `internal/sealed`, `internal/ui`.
- **No es v1.0.** Falta el examinador ciego fuera de go y python, y el nivel
  funcional de la detección de solapamiento. Ver "Qué falta".
- **Arreglado el 10 sep:** la instrumentación medía cero cuando el agente
  commiteaba por su cuenta, la demo escribía tokens falsos en el ledger real, y
  una corrida muerta dejaba la tarea atascada para siempre mientras el tablero
  la pintaba viva. Los tres abajo, en "Arreglado".

## Orden de lectura para quien llegue nuevo

1. `README.md` — qué hace la herramienta hoy y con qué comandos.
2. `docs/PRD-devclean.md` — la especificación.
3. Este archivo.
4. `git log --oneline`.

**`docs/PRD-adenda.md` ya no existe.** Se borró en `704f8b5` (1 sep 2026)
junto con código muerto. Eran 216 líneas y el PRD todavía la cita por número
(`adenda A.1`, `A.3`, `A.4`, `A.5`, `§6.7`–`§6.11`). Esas referencias apuntan a
un archivo ausente: para recuperarla, `git show 704f8b5^:docs/PRD-adenda.md`.
Todo lo que decía la adenda y sigue vigente está resumido abajo; no hace falta
leerla para trabajar.

**El PRD está parcialmente desactualizado a propósito**: es la especificación
original, no un registro de avance. Los marcadores de implementación de §6.7,
§6.8, §6.9 y §6.11 se corrigieron hoy contra el código, y la tabla de comandos
de §7 se completó. Para saber qué existe, manda este archivo y el README.

---

## Qué existe hoy

### Comandos

Todos aceptan `--plain` y `--json`. Sin ellos y con salida a terminal, usan el
TUI.

| Comando | Qué hace |
|---|---|
| `up ["<petición>"]` | De una petición a un PR limpio. Prepara el entorno, planea, ejecuta y entrega. |
| `plan "<texto>"` | Convierte una petición en contratos de tarea; pide aprobación (`--aprobar` la salta). `--export-spec` la vuelca a YAML. |
| `apply [-f archivo.spec.yml]` | Crea tareas desde una especificación declarativa. `--run` las ejecuta. |
| `run [--agentes N] [--reintentar]` | Ejecuta las tareas pendientes en paralelo. `--reintentar` revive las detenidas reusando su cuarto. |
| `ship [id] \| ship --todas` | Esclusa de salida y PR. `--dry-run` hace todo menos abrir el PR. |
| `board` | Tablero por estado. |
| `ps` | Estado de tareas y cuartos activos, estilo compose. |
| `logs <id>` | Los intentos de una tarea, uno por línea. |
| `standup` | Parte de datos duros de las tareas en curso (§6.7). |
| `report` | Las métricas de §9 con flecha de tendencia. |
| `usage` | Gasto por ventanas rodantes (5h, semanal, mensual) contra el presupuesto. |
| `doctor` | Verifica git, repo, configuración, ejecutores, keys y que los modelos existan. |
| `init` | Detecta repo, rama base y comando de pruebas; crea `.devclean/`. |
| `task add\|edit\|rm\|list\|seal` | Contratos a mano. `seal` sella una suite oculta escrita por ti, sin gastar modelo. |
| `check <id>` | Esclusa de entrada sobre una tarea (alias de `task check`). |
| `constitution` | Genera `.devclean/constitution.md` (§6.11). |
| `skills sync` | Trae con `npx` las skills que se inyectan en el prompt de cada agente. |

### Paquetes

24 en `internal/`. Los que no se explican solos:

- **`kv`** — el parser YAML propio (`Pairs`, `Nested`, `ParseList`,
  `MarshalList`). **No escribas un segundo.** Es deliberadamente chico y tiene
  un límite que muerde: no distingue indentación, así que dos claves con el
  mismo nombre a distinta profundidad se pisan.
- **`task`** / **`gate`** — el contrato de §6.1 y la esclusa de entrada de
  §6.3.
- **`room`** — cuartos aislados: un worktree por tarea, dependencias por
  manifiesto, puerto libre.
- **`loop`** — el bucle de trabajo del agente (§6.4) con la instrumentación por
  intento. Declara su propia interfaz `Agent` (lado del consumidor).
- **`executor`** — adaptadores para las CLIs `opencode` y `claude`.
- **`examiner`** + **`sealed`** — examinador ciego y suite oculta (§6.8). El
  directorio sellado vive en el repo principal (`.devclean/sealed/<id>/`),
  nunca en el cuarto: el cuarto es dominio del implementador.
- **`revisor`** — un modelo lee el diff y puede vetar el merge. Es el único
  paso que juzga intención en vez de mecánica. **Falla cerrado**, al revés que
  el examinador.
- **`ship`** — la esclusa de salida.
- **`overlap`** — detección de solapamiento (§6.9), niveles textual y
  semántico.
- **`constitution`** — `.devclean/constitution.md` (§6.11), inyectada en el
  contexto de todos los agentes.
- **`recurse`** — ejecución recursiva (§8.3): una tarea `recursivo: true` se
  parte en subtareas con contrato propio que corren en cuartos anidados dentro
  del cuarto padre. **Apagada por default** (`recursion_max: 0`), porque cada
  nivel multiplica el gasto.
- **`budget`** — tope de gasto de una corrida (`presupuesto_tokens`). Es la
  única salvaguarda contra una recursión que se descontrola.
- **`ventanas`** — ledger de gasto por ventanas rodantes, **global al usuario**
  (`~/.devclean/ventanas.jsonl`), no por repo. Existe porque los proveedores no
  exponen los límites reales de sus ventanas de 5h/semanal.
- **`spec`** — el modelo declarativo de `devclean.spec.yml`.
- **`standup`** — el parte de datos de §6.7, derivado de `attempts.jsonl`. Sin
  modelo: los detectores son deterministas.
- **`metrics`** — las cinco métricas de §9 derivadas de los artefactos.
- **`skills`** — trae SKILL.md reales y los inyecta como texto en el prompt, no
  como etiqueta. El fetch corre contra la raíz del repo, nunca dentro de un
  cuarto.
- **`plan`**, **`config`**, **`state`**, **`tui`**, **`ui`**.

### La esclusa de salida: 9 pasos, 10 con `reglas_import`

`internal/ship/ship.go`, en este orden. Se frena en el primero que falla y da
la razón exacta. **`dependencias` es condicional**: siempre se verifica, pero
solo aparece como paso cuando falla o cuando hay `reglas_import` declaradas —
por eso una corrida normal lista nueve.

1. `base` — rebase sobre la rama base; conflicto → abortar.
2. `historial` — aplana los `wip:` en un commit Conventional con trailer `Agent:`.
3. `ruido` — prints de debug, código comentado, temporales.
4. `secretos` — keys de proveedores, privadas, credenciales en claro.
5. `presupuesto` — `limite_lineas` y archivos.
6. `interfaces` — el diff contiene lo que `expone` prometía (§6.10).
7. `dependencias` — el grafo de imports del diff respeta `reglas_import` (§6.10). Condicional, ver arriba.
8. `bisectable` — corre `pruebas` sobre el commit aplanado. Con `pruebas` vacío falla con `sin comando de pruebas · decláralo en config.yml`.
9. `handoff` — qué cambió, qué no, cómo verificar. Determinista.
10. `pr` — sube la rama, `gh pr create`, libera el cuarto.

### Configuración (`.devclean/config.yml`)

`base`, `pruebas`, `cli`, `zonas_prohibidas`, `patrones_prueba`,
`timeout_esclusa`, `timeout_agente`, `timeout_pruebas`, `recursion_max`,
`subagentes`, `presupuesto_tokens`, `presupuesto:` (por proveedor y ventana),
`proveedores:` (roles `planificador`/`ejecutor`/`revisor`, cada uno
`{modelo, key_env}`), `agentes:`, `estrategia`, `modelos:`, `reglas_import`.

**`cli` se llama `cli` y no `ejecutor` a propósito**: ese nombre ya lo usa el
rol `ejecutor` dentro de `proveedores`, y `kv.Pairs` no distingue indentación.

### Programación agéntica como código (`devclean.spec.yml`)

El spec define la corrida completa, no solo la lista de tareas: `feature`,
`reglas:` (se anteponen a las `notas` de cada tarea, que es el canal que el
prompt ya inyecta), `agentes: N` y `ship: true`. Lo respetan `up` (el flag
`--agentes` gana) y `apply --run`.

### Distribución

- Releases en GitHub de `v0.2.0` a `v0.8.2`, cada una con seis binarios
  estáticos (linux/darwin/windows × amd64/arm64), `checksums.txt` e
  `install.sh` como asset.
- `.goreleaser.yml` corre `go mod tidy` y `go test ./...` antes de construir.
- `scripts/install.sh` agrega el destino al `PATH` del shell que detecte.
- `scripts/demo.sh` + `docs/demo.tape` graban `docs/demo.gif` con `vhs`, usando
  un agente falso.
- `go install github.com/Pastranauwu/devclean/cmd/devclean@latest` funciona.

---

## Qué falta

En el orden en que conviene atacarlo.

### 1. Árbol de trabajo sin commitear

`cmd/devclean/apply.go`, `plan.go`, `ps.go` tienen cambios puramente
cosméticos: concatenaciones con `+` partidas en `WriteString` sucesivos. Cero
cambio de conducta. Commitear o descartar antes de empezar otra cosa.

### 2. Examinador ciego: solo go y python — bloquea v1.0

`internal/examiner/lenguaje.go`. `rust` está descartado a propósito y con
motivo escrito: la stdlib de Go no parsea rust, validar exige el crate `syn` o
`cargo check`, y eso arrastra el toolchain completo dentro del cuarto. Node y
el resto, sin empezar.

Sin validador de sintaxis la suite generada es basura que rompe la compilación
del cuarto, y el implementador no puede tocarla (A.3). Por eso `lenguajeExamen`
devuelve `""` en vez de improvisar.

### 3. Solapamiento funcional (§6.9)

Los tres niveles son textual, semántico y funcional. Los dos primeros están;
falta el tercero: merge en seco de dos ramas y correr las suites de ambas sobre
el resultado. Es el que atrapa el fallo clásico — dos ramas verdes por separado
que rompen juntas — y el único que cuesta caro, así que solo debe dispararse
cuando textual o semántico marcaron sospecha.

### 4. Tap de Homebrew

`.goreleaser.yml` no tiene bloque `brews`. Hace falta un repo `homebrew-*`
aparte y agregarlo.

### 5. Deuda chica

- `internal/executor` quedó en el historial de `22a48f8` y `a781c73` antes de
  sacarlo con `git rm --cached`. Solo se limpia reescribiendo historia.
- `internal/task/store_test.go` construye `Task` sin `Version`. Pasa porque
  `Marshal` omite el cero y es `Validate` quien exige el campo, pero el fixture
  miente sobre el contrato.
- `internal/constitution`, `internal/sealed` e `internal/ui` no tienen pruebas.
- `friccion` queda en `null` en `report`: necesita el ciclo de revisión del PR
  y no hay fuente todavía.

---

## Arreglado el 10 septiembre 2026

**La instrumentación medía cero cuando el agente commiteaba solo.**
`internal/loop` medía cada intento con `git diff --cached ... HEAD` después de
`git add -A`. Un agente real commitea por su cuenta dentro del cuarto (la skill
`implement` lo hace): HEAD se movía, el índice quedaba vacío y el intento se
escribía en `attempts.jsonl` con `archivos_tocados: []` y `lineas_mas/menos: 0`
pese a haber trabajo. Esa mentira era la fuente de las cinco métricas de §9,
del `standup` y del nivel semántico de `overlap`.

Ahora cada intento captura el commit con que arranca (`antes`, vía
`resolveCommit`) y se mide con `git diff <antes>`, que compara esa ref contra
el árbol de trabajo y por eso cuenta igual lo commiteado y lo suelto.
`stagedFiles`/`stagedNumstat` se fueron; quedan `filesSince`/`numstatSince`, y
`changedVsBase` era ya lo mismo que `filesSince`, así que se fusionaron.
Pruebas: `TestStatsConArchivoNuevo`, `TestStatsCuandoElAgenteCommiteaSolo` (la
segunda falla con el código viejo).

**La demo escribía en el ledger real.** `scripts/demo.sh` no aislaba `HOME`, así
que sus tokens inventados por el agente falso entraban en
`~/.devclean/ventanas.jsonl` — global al usuario, y la base sobre la que
`devclean usage` y los topes de `presupuesto:` deciden si el trabajo real puede
seguir. De las 45 entradas del ledger, 20 eran ruido de demo. Ahora `demo.sh`
aísla `HOME` dentro de su `mktemp -d`, igual que ya hacía `demo-env.sh`.
Verificado: la demo corre idéntica y el ledger no cambia.

**El ledger ya se limpió** (10 sep): salieron las 30 entradas de demo —
`opencode` con `tokens` 120 o 7 del 9 y 10 sep — y quedaron 15 reales. El 120
es el agente falso (`input:100, output:20`) y el 7 su revisor: son los dos
sitios donde `internal/loop` llama a `Registrar`, por eso venían apareados. Las
cuatro entradas de `claude` del 9 sep a las 20:34 son el dogfooding real y se
conservaron. Respaldo en `~/.devclean/ventanas.jsonl.bak-20260910`.

**Una corrida muerta dejaba la tarea atascada para siempre.** Un SIGKILL —
la laptop suspende, se cae el ssh — mataba `run` sin bajar el estado de la
tarea. Quedaba en `en_curso`, y `run` mandaba ese estado a `existentes` (que
solo sirve para el cruce), así que ni `run` ni `run --reintentar` la volvían a
mirar: recuperarla pedía editar `.devclean/state/` a mano. Peor, `latido.json`
sobrevivía al kill, y como todo el mundo leía "existe el archivo = corre", el
tablero decía **en curso** y el standup **"dentro de contrato"** de una tarea
muerta. La premisa falsa estaba escrita en el propio comentario del tipo.

Ahora `Latido` lleva `Visto` y una corrida viva lo refresca cada
`LatidoIntervalo` (15 s) desde una goroutine, no solo al cambiar de fase — la
fase `agente` dura lo que dure la invocación y se volvía rancia sola. Vivo es
`Visto` a menos de `LatidoRancioTras` (90 s, seis intervalos). Con eso:

- `LeerLatido` devuelve "no corre" si el latido está rancio; `Interrumpida`
  distingue "murió a media tarea" de "nunca arrancó".
- `board` y el TUI muestran `interrumpida · sin señal hace X`, el segundo en
  rojo; `standup` levanta `⚠ MUERTA` en vez de `✓ dentro de contrato`.
- `run --reintentar` la retoma por el mismo camino que una detenida
  (`room.Ensure` ya reusaba el cuarto); `run` a secas dice qué pasó y qué
  escribir.

No se revive solo, a propósito: si otra corrida la está trabajando de verdad su
latido está fresco, y revivirla pondría dos agentes en el mismo cuarto. El
margen de 6× sobre el intervalo (en vez de los 2–3× de la regla común) es por
lo mismo — declarar muerta una corrida viva es el error caro; al revés solo se
esperan 90 s.

Se descartó comparar el PID: en Linux un pid se reutiliza apenas el proceso
muere, así que un pid vivo no prueba que sea *tu* proceso, y `os.FindProcess`
en Unix devuelve un Process exista o no (haría falta `Signal(0)`, que además no
es portable a Windows). El latido fresco no tiene ese problema y es igual en
las tres plataformas del release.

Pruebas: `TestLatidoVivoSoloMientrasLoRefrescan`,
`TestSinLatidoNoHayCorridaNiInterrupcion`, `TestLatidoSinVistoSeDaPorMuerto`,
`TestAnalizarDistingueAtascoDeCorridaMuerta`.

---

## Cosas que muerden si no las sabes

- **`gate.Run` devuelve 6 chequeos, no 4.** El orden cambia cada vez que se
  agrega uno: **búscalos por nombre, no por índice.** `Result` trae además un
  campo `Aviso` para el aviso de versión futura.
- **El chequeo 0 es `contrato válido`**, que llama a `Validate()`. Sin él, un
  archivo sin `version` pasaba en verde.
- **A.3 es más estrecho que la letra de la adenda, a propósito.** `globsOverlap`
  es conservador y `*_test.go` se cruza con *todo*, así que aplicarlo tal cual
  rechazaba `tocar_solo: ["src/export/**"]`, o sea cualquier contrato
  razonable. Lo que se rechaza es *apuntarle* a las pruebas. De los archivos de
  prueba que se editen igual se encarga `revertFueraDeAlcance` en
  `internal/loop`.
- **El parser YAML vive en `internal/kv`.** No escribas un tercero. Y ojo con
  la indentación: `kv.Pairs` no la distingue.
- **Todo lo que genere contratos tiene que poner `version: 1`.** La constante es
  `task.Version`.
- **El paso `interfaces` compara el nombre, no la firma completa**
  (`task.NombreDeFirma`). El lenguaje reescribe nombres de parámetros y orden de
  tipos; el texto literal rechazaría implementaciones correctas.
- **`overlap.mergeTree` no puede tratar el exit 1 de `git merge-tree` como
  conflicto**: una rama que aún no existe sale con 1. Y el parseo va por líneas
  de etapa (`<modo> <oid> <etapa>\t<ruta>`), no por el prefijo `CONFLICT`, que
  está localizado y no sale con `--no-messages`.
- **El revisor degrada en abierto, el examinador falla cerrado.** Un revisor que
  no responde no frena trabajo verde; un examinador que no responde sí frena.
- **La recursión viene apagada** (`recursion_max: 0`). Una tarea
  `recursivo: true` corre plana hasta que la subas.
- **El ledger de ventanas es global al usuario**, en `~/.devclean/`, no en el
  repo. Borrar `.devclean/` no lo reinicia.
- **`ship` sin `origin` fallaba con `exit status 128`.** El error de git se lee
  de su salida, no de `err.Error()`, que es solo "exit status N".
- **Un latido en disco no significa que la tarea corra.** Un SIGKILL lo deja
  ahí para siempre. La pregunta es `l.Vivo()` (o sea `Visto` reciente), y para
  eso una corrida viva tiene que estar refrescándolo: si agregas un camino que
  escriba latidos, tiene que latir, no solo escribir una vez.
- **`standup.Analizar` quiere los latidos EN CRUDO** (`LeerLatidosCrudos`), no
  los filtrados. La diferencia entre latido fresco y rancio es la que separa
  ATASCO de MUERTA; filtrados, las dos se ven igual que un hueco.
- Los mensajes de error siguen §16.6: minúscula, sin punto final, dicen qué pasó
  y qué hacer.
