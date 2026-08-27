# Estado del proyecto — traspaso entre sesiones

Última actualización: 27 agosto 2026. El MVP v0.1 está completo y etiquetado
`v0.1.0`. Fases 1–5 cerradas; falta publicar el release y un par de pulidos.

**Orden de lectura para quien llegue nuevo:**
1. `docs/PRD-devclean.md` — la especificación.
2. `docs/PRD-adenda.md` — correcciones posteriores. **Donde contradiga al PRD, gana la adenda.**
3. Este archivo.
4. `git log --oneline`.

Ojo: la cabecera de la adenda dice "aplica sobre `docs/PRD.md`". Ese archivo no
existe; es `docs/PRD-devclean.md`.

---

## Qué está hecho y verificado

Todo lo de abajo compila en clon limpio y tiene pruebas verdes.

**Fase 1 — núcleo de tareas**
- `cmd/devclean` + `internal/{config,task,gate,ui}`. Única dependencia: cobra.
- `devclean init` detecta repo, rama base y comando de pruebas.
- Contrato §6.1 con parser estricto; `listo_cuando` obligatorio.
- `task add|edit|rm|list` con ids correlativos.
- `task check`: esclusa de entrada, ejecuta `listo_cuando` de verdad.
- `--plain` y `--json` en todos los comandos.

**Fase 2 — hecha**
- `internal/state`: estados en `.devclean/state/`, el cruce solo mira `en_curso`.
- `internal/room`: cuartos aislados con worktree, deps por manifiesto, puerto libre.
- `internal/executor`: adaptadores opencode y claude.
- `internal/loop`: el bucle de §6.4 con la instrumentación de la adenda
  A.2. Cada intento escribe una línea en `.devclean/runs/<id>/attempts.jsonl`
  con `salida_codigo`, tests parseados (null si no se puede), `archivos_tocados`,
  `lineas_mas/menos`, `simbolos_exportados` (go/ast, null si no hay Go),
  `revertidos_fuera_de_alcance`, tokens y modelo. Reversión de fuera de
  alcance y de rutas de prueba (A.3) incluida; puntos de restauración
  `wip:`. Declara su propia interfaz `Agent` (lado del consumidor).
- `devclean run [--agentes N] [--ejecutor ...] [--modelo ...]`: esclusa de
  entrada por tarea, asignación (A.4 + cruce), N tareas en paralelo, estados
  `en_curso → lista | detenida`. El adaptador `executor` → `loop.Agent` vive en
  `cmd/devclean/run.go`; el cuarto no se destruye en `run`, lo libera `ship`.

**Fase 3 — hecha**
- `internal/ship`: la esclusa de salida (§6.5), ocho pasos en orden. La
  compuerta se frena en el primero que falla y da la razón exacta.
  1. `base` (rebase sobre la rama base, conflicto → abortar),
  2. `historial` (aplanar los `wip:` en un commit Conventional + trailer `Agent:`),
  3. `ruido` (prints de debug, código comentado, temporales),
  4. `secretos` (claves de proveedores, privadas, credenciales en claro),
  5. `presupuesto` (`limite_lineas`, archivos),
  6. `bisectable` (corre `pruebas` en el commit aplanado),
  7. `handoff` (qué cambió, qué no, cómo verificar — determinista),
  8. `pr` (sube la rama, `gh pr create`, libera el cuarto).
- `devclean ship <id> [--dry-run]`: exige estado `lista`, corre la esclusa y
  muestra un paso por línea; `--dry-run` hace todo menos abrir el PR. El
  squash produce un solo commit (dentro de los 1–5 del criterio de
  aceptación); el split en varios es mejora de v0.2.

**Fase 4 — hecha**
- `internal/metrics`: las cinco métricas de §9 derivadas de los artefactos
  (`attempts.jsonl`, estados y un registro de entrega que `ship` deja en
  `.devclean/runs/<id>/entrega.json`). `friccion` queda en null: necesita el
  ciclo de revisión del PR, sin fuente en v0.1. `devclean report` las muestra
  con su **flecha de tendencia** (§16.4): cada corrida apunta un snapshot en
  `.devclean/historial.jsonl` y compara contra la anterior (↑ subió, ↓ bajó,
  · igual o sin dato previo).
- `devclean doctor`: verifica git, repo, configuración, ejecutores y keys.
- `devclean board`: tablero por estado (listo, en curso, detenido, pendiente).
- `devclean logs <id>`: los intentos de una tarea, uno por línea.
- `internal/tui`: el modo interactivo con la paleta del cuarto limpio
  (§16.2): logotipo de píxeles con degradado (fuente unsciithin de `bit`),
  tarjetas con borde, spinner braille y barras de progreso reales. Tres
  vistas — la compuerta animada de `ship` (§16.3), el tablero de `board` y la
  corrida en vivo de `run` (N tareas, spinner, reloj y barra global). El
  tablero además corre un **plasma truecolor animado** de fondo (suma de
  cuatro senos, medio bloque ▀, paleta neón verde) con el logo centrado como
  sticker de margen transparente. Cada comando usa el TUI cuando la salida es
  terminal y no hay `--plain` ni `--json`; si no, texto plano.
- `internal/plan` + `devclean plan "<texto>"`: el planificador (§5, §8.2)
  parte una petición en contratos. El texto lo produce un modelo (vía el
  ejecutor, cuyo `Result.Text` ahora trae la respuesta); devclean solo parsea
  el JSON, asigna ids y pide aprobación (`--aprobar` para no preguntar). El
  rol planificador usa `--modelo`/`--ejecutor` como `run`; la selección
  "el mejor disponible" queda para v0.2.
- **Config anidado `proveedores`** (§8.1): `config.yml` acepta el bloque
  `proveedores:` con un rol por línea (`planificador`, `ejecutor`, `revisor`),
  cada uno `{ modelo: X, key_env: Y }`. `run` y `plan` caen al modelo del rol
  (`ejecutor` / `planificador`) cuando no hay `--modelo`; `doctor` verifica
  también las `key_env` declaradas. El parser anidado vive en `internal/kv`
  (`Nested` + `ParseInlineMap`/`MarshalInlineMap`), no un tercero.

**Adenda, Parte A y C**
- **A.1** `version: 1` obligatoria. Un archivo con versión mayor a la del binario
  se lee igual, ignorando lo desconocido, y avisa: `contrato versión 2, binario
  soporta 1 · actualiza devclean`. La constante es `task.Version`.
- **A.3** el validador rechaza `tocar_solo` que apunte a rutas de prueba. Los
  patrones viven en `patrones_prueba` de `config.yml` y los siembra `init`.
- **A.4** `tocar_solo` vacío: permitido con una sola tarea en curso, obligatorio
  con dos o más.
- **C.1** el parser yaml salió a `internal/kv`; las dos copias de `config` y
  `task` están borradas.
- **C.2** `timeout_esclusa` configurable, default 5 min.
- **C.3** alias `devclean check`.
- **C.5** `init` muestra el comando de pruebas detectado y deja corregirlo;
  `--pruebas` lo fija sin preguntar.

**Parte B** documentada en el PRD (§6.7 a §6.11), sin implementar, más §6.2b y
el presupuesto de sobrecarga de A.5 en requisitos no funcionales.

**Fase 5 — lanzamiento**
- `README.md` con manifiesto, instalación, adopción en proyectos reales,
  métricas y el límite honesto.
- `.goreleaser.yml` (binario estático por plataforma) y `scripts/install.sh`.
- `scripts/demo.sh` (demo reproducible con agente falso, se autocompila) y
  `docs/demo.tape` para grabar el GIF con `vhs`. **El GIF (`docs/demo.gif`)
  ya está grabado.**
- Etiquetado y pusheado **`v0.1.0`** (`main` + tag). Falta publicar el release
  en GitHub (no hay `gh`; se hace desde la web o instalando `gh`).

---

## Qué falta

**El v0.1 está funcionalmente completo y el GIF grabado.**

- **Release `v0.2.0` publicada** en GitHub con binarios estáticos
  (linux/darwin/windows × amd64/arm64), `install.sh` y `checksums.txt`.
  `install.sh` y `go install github.com/Pastranauwu/devclean/cmd/devclean@v0.2.0`
  verificados. `v0.1.0` quedó etiquetada en un commit anterior (sin release).
- **Tap de Homebrew** pendiente: un repo `homebrew-*` aparte + `goreleaser`
  con `brews`. Se hace tras publicar la release.

**v0.2:** Parte B entera (examinador ciego, solapamiento funcional, duplicación
entre ramas, reglas de dependencia, constitución). Ya especificada, sin empezar.

**Deuda conocida, chica**
- `internal/executor` aparece en el historial de dos commits (`22a48f8`,
  `a781c73`) antes de que lo sacara con `git rm --cached`. Se limpia solo
  reescribiendo historia.
- `internal/task/store_test.go` construye `Task` sin `Version`. Pasa porque
  `Marshal` omite el cero y es `Validate` quien exige el campo, pero el fixture
  miente sobre el contrato.

---

## Cosas que muerden si no las sabes

- **`gate.Run` devuelve 6 chequeos, no 4.** El orden cambia cada vez que la
  adenda agrega uno: búscalos por nombre, no por índice. `Result` trae además
  un campo `Aviso` para el aviso de versión futura.
- **El chequeo 0 es `contrato válido`**, que llama a `Validate()`. Antes la
  esclusa no validaba el contrato y un archivo sin `version` pasaba en verde.
- **A.3 es más estrecho que la letra de la adenda, a propósito.** `globsOverlap`
  es deliberadamente conservador y `*_test.go` se cruza con *todo*, así que
  aplicarlo tal cual rechazaba `tocar_solo: ["src/export/**"]`, o sea todo
  contrato razonable. Lo que se rechaza es *apuntarle* a las pruebas. De los
  archivos de prueba que se editen igual se encarga la reversión del bucle,
  que es lo que dice la segunda frase de A.3 — **y esa reversión ya está
  implementada en `internal/loop`** (`revertFueraDeAlcance`).
- **El parser yaml vive en `internal/kv`.** No escribas un tercero.
- **Lo que genere contratos tiene que poner `version: 1`.**
- Los mensajes de error siguen §16.6: minúscula, sin punto final, dicen qué pasó
  y qué hacer.
