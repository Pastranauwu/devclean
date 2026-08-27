# Estado del proyecto — traspaso entre sesiones

Última actualización: 27 agosto 2026. Cierra la fase 3: `devclean ship`,
la esclusa de salida, está hecho.

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

---

## Qué falta

**Fases 1–3 cerradas.** Queda la fase 4 (TUI con bubbletea según §16,
las cinco métricas y `devclean report`, `devclean doctor`) y la fase 5
(lanzamiento: GoReleaser, instalador, README).

**v0.2:** Parte B entera. Ya está especificada, nadie la ha empezado.

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
