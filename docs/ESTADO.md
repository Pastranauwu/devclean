# Estado del proyecto — traspaso entre sesiones

**Última actualización: 14 septiembre 2026.** Reescrito contra el código.

- **HEAD:** `main`, sincronizada con `origin/main`.
- **Última release publicada:** **v1.0.0** (14 sep 2026), etiquetada en HEAD.
- **Compila y pasa:** `go build ./...` y `go vet ./...` limpios; `go test ./...`
  verde. **Los 24 paquetes de `internal/` tienen pruebas; ninguno queda sin.**
- **Es la 1.0.** Los tres niveles de §6.9 corren, las cinco métricas de §9 dan
  número, y el alcance del examinador ciego (go y python) es una decisión, no
  un hueco. Ver "Decisiones de alcance".
- **Cerrado el 14 sep:** la corrida ya sobrevive a cerrar la terminal
  (`--fondo`), la fricción dejó de ser `null`, el nivel funcional de
  solapamiento entró, y no queda paquete sin pruebas. Todo abajo, en
  "Cerrado para la 1.0".

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
| `up ["<petición>"]` | De una petición a un PR limpio. Prepara el entorno, planea, ejecuta y entrega. `--fondo` desprende todo el encadenado de la terminal. |
| `plan "<texto>"` | Convierte una petición en contratos de tarea; pide aprobación (`--aprobar` la salta). `--export-spec` la vuelca a YAML. |
| `apply [-f archivo.spec.yml]` | Crea tareas desde una especificación declarativa. `--run` las ejecuta. |
| `run [--agentes N] [--reintentar] [--fondo]` | Ejecuta las tareas pendientes en paralelo. `--reintentar` revive las detenidas y las interrumpidas reusando su cuarto. `--fondo` la desprende de la terminal. |
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

24 en `internal/`, **todos con pruebas**. Los que no se explican solos:

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
- **`overlap`** — detección de solapamiento (§6.9) en sus tres niveles.
  Textual y semántico corren antes de la oleada; el funcional
  (`funcional.go`) **después**, y solo sobre pares verdes y sospechosos.
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
- **`metrics`** — las cinco métricas de §9. Cuatro salen de los artefactos del
  repo y las calcula `Calcular`, que es pura; la fricción sale de `gh` y la
  pone `Friccion` aparte, para no meter red en una función de cálculo.
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

## Decisiones de alcance

No son pendientes. Son cosas que se miraron y se decidió no hacer; están acá
para que nadie las vuelva a abrir creyendo que se olvidaron.

### El examinador ciego cubre go y python, y con eso basta

`internal/examiner/lenguaje.go`. No es una implementación a medias: es el
alcance. Sin validador de sintaxis la suite generada es basura que rompe la
compilación del cuarto y el implementador no puede tocarla (A.3), así que
`lenguajeExamen` devuelve `""` en vez de improvisar y la tarea corre sin
examinador ciego. Rust queda fuera con motivo escrito: la stdlib de Go no lo
parsea, validar exige el crate `syn` o `cargo check`, y eso arrastra el
toolchain completo dentro del cuarto. Node y el resto, igual: se agregan si
aparece la necesidad real, no antes.

### Sin Homebrew

Se evaluó y se descartó el 14 sep. Un tap es un repo aparte más un PAT en los
secretos, y `brew` solo instala casks en macOS. La distribución es
`scripts/install.sh`, los binarios de cada release y `go install`, que es por
donde llega todo el mundo. Si algún día hace falta empaquetar para un gestor,
el candidato es el AUR, no Homebrew.

## Qué falta

Nada bloquea la 1.0. Lo que queda es para después:

- **Mutation score como control del examinador** (§6.8). El examinador ya
  corre; falta medir si su suite de verdad mata mutantes.
- **Duplicación entre ramas** (§6.10).
- **Modo API directa.** Hoy todo pasa por la CLI del agente.
- **Un tercer proveedor.** Hoy `opencode` y `claude`.
- **`internal/executor` quedó en el historial** de `22a48f8` y `a781c73` antes
  de sacarlo con `git rm --cached`; hoy está versionado normal, así que la
  nota es de historia y nada más. Solo se limpiaría reescribiendo historia y
  no vale la pena: es código, no secretos.

## Cerrado para la 1.0 — 14 septiembre 2026

**La corrida no sobrevivía a cerrar la terminal.** Ver el detalle abajo, en la
entrada del 10 sep: el trabajo estaba hecho y sin commitear, y entró tal cual.

**El solapamiento funcional (§6.9, nivel 3).** Era el único de los tres
niveles que faltaba, y el que atrapa el fallo que justifica al resto: dos
ramas verdes por separado que rompen juntas. Los niveles 1 y 2 miran el diff
y ahí no hay nada que ver — no hay conflicto de texto ni símbolo en común
cuando lo que cambió fue un comportamiento del que la otra dependía.

`overlap.CheckFuncional` monta la fusión en un worktree suelto y corre ahí los
`listo_cuando` de las dos tareas. El árbol no se recalcula: `merge-tree` ya lo
escribía en `CheckPar` y se tiraba; ahora vuelve en `Resultado.Arbol` y de él
sale un commit detached con los dos padres. No toca las ramas de las tareas ni
el árbol de trabajo del repo.

Tres cosas que muerden si las tocas:

- **Corre DESPUÉS de la oleada**, al revés que los otros dos niveles. "Verdes
  por separado" exige que las dos estén verdes; al arrancar, las ramas están
  vacías. Por lo mismo el repaso vuelve a llamar a `CheckPar` en vez de reusar
  el de antes, y saltea las alertas que ya se dijeron.
- **Dos filtros antes de gastar:** solo pares verdes (una suite que ya fallaba
  en su rama no dice nada sobre la fusión) y solo pares sospechosos
  (`Resultado.Sospechoso`), porque es el único nivel que ejecuta código.
- **El worktree se destruye con contexto propio.** Con el de la corrida ya
  cancelado quedaría montado y el próximo par chocaría contra él.

Pruebas: `TestDosVerdesQueRompenAlFusionarse` (el caso real),
`TestElWorktreeDeLaFusionNoSobrevive`, `TestSinListoCuandoAvisaQueNoSePudo`.

**La fricción dejó de ser `null`.** De las cinco métricas de §9 era la única
que `report` imprimía siempre como "— sin datos". Su fuente no está en el
repo: el ciclo de revisión pasa en GitHub, así que se le pregunta a `gh` por
cada entrega que dejó URL de PR.

Se mide contra la **primera** aprobación, no la última: lo que §9 mide es
cuánto tarda el trabajo en quedar desbloqueado, y una segunda aprobación ya no
desbloquea nada. Un PR sin aprobar no cuenta como cero —cero minutos de
fricción sería un número excelente y una mentira—, cuenta como sin dato.
Degrada en abierto, como el revisor: sin `gh`, sin red o sin PRs aprobados
vuelve a `null`. Techo de 20 s, porque `report` es de lectura y colgarse contra
una red mala es peor que no dar el número. `metrics.Calcular` sigue siendo
pura y sin red; la fricción se pone aparte, en `metrics.Friccion`.

**Ya no hay paquetes sin pruebas.** `internal/constitution`, `internal/sealed`
e `internal/ui` tenían cero. Cubren lo que se puede romper en silencio: que
una constitución ausente no sea un error (todo el mundo la carga al arrancar),
que el directorio sellado nunca caiga dentro del cuarto, y que `--json` no
mezcle líneas de texto con el documento.

**El fixture de `internal/task/store_test.go` mentía sobre el contrato.**
Construía `Task` sin `Version`. Pasaba porque `Marshal` omite el cero y es
`Validate` quien exige el campo, así que guardaba en verde una tarea que
`Validate` rechaza. Ahora lo pone y verifica que dé la vuelta y valide.

## Arreglado el 10 septiembre 2026

(La entrada de `--fondo`, al final, se implementó el 10 y se commiteó el 14.)

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

**La corrida no sobrevivía a cerrar la terminal.** `run` y `up` eran de primer
plano: cerrar la terminal les mandaba SIGHUP y se iban con ella, justo en el
escenario que devclean existe para servir —dejar tareas pesadas e irse.

`--fondo` (en `run` y en `up`) vuelve a arrancar el mismo comando desprendido y
devuelve el control. Detalles que importan:

- **Se desprende después de preparar el entorno.** La preparación es lo único
  que puede preguntar, y preguntar sirve mientras el humano sigue ahí. El hijo
  hereda un `config.yml` ya completo.
- En `up` cubre el encadenado entero (plan, run y entrega), no solo la corrida.
- La salida va a `.devclean/corridas/<fecha>.log`. Como stdout deja de ser
  terminal, `esTUI()` cae a texto plano solo, sin tocar nada.
- `sinFondo` quita la bandera al relanzar. Sin eso el hijo se desprende otra
  vez, y otra, para siempre — por eso tiene prueba propia.
- Unix usa `Setsid`; Windows, `DETACHED_PROCESS | CREATE_NEW_PROCESS_GROUP` en
  archivos con etiqueta de build. Las seis plataformas del release compilan.

Parar es `kill <pid>`, y retomar `run --reintentar` — que funciona justo por el
arreglo del latido de arriba: sin él, matar una corrida desprendida dejaba la
tarea atascada para siempre.

Pruebas: `TestSinFondoQuitaLaBanderaYNadaMas`, `TestDesprenderPideSesionPropia`.
Verificado en vivo: corrida lanzada con `--fondo`, SIGHUP a la sesión de la
terminal, la corrida sobrevivió (sesión propia, sin tty, reparentada a init) y
terminó la tarea en verde.

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
- **El nivel funcional de overlap corre después de la oleada, no antes.** Si
  lo mueves al arranque mide ramas vacías y no encuentra nada. Y necesita el
  árbol de `merge-tree` en `Resultado.Arbol`: si alguien deja de devolverlo,
  el nivel 3 se apaga en silencio.
- **`report` toca la red.** La fricción pregunta a `gh` por cada PR. Tiene
  techo de 20 s y degrada a `null`, pero si agregas otro llamador de
  `metrics.Friccion` acuérdate de que no es una función de cálculo.
- Los mensajes de error siguen §16.6: minúscula, sin punto final, dicen qué pasó
  y qué hacer.
