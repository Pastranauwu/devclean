# Estado del proyecto — traspaso entre sesiones

Última actualización: 26 agosto 2026. Escrito al cerrar la Parte A y la Parte C
de la adenda.

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

**Fase 2 — parcial, otro agente**
- `internal/state`: estados en `.devclean/state/`, el cruce solo mira `en_curso`.
- `internal/room`: cuartos aislados con worktree, deps por manifiesto, puerto libre.
- `internal/executor`: adaptadores opencode y claude. **En disco pero sin
  commitear** — es trabajo en curso de ese agente, no lo commitees por él.

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

**Crítico, bloquea todo lo demás**

- **A.2 · instrumentación del bucle.** Le toca a quien construya `internal/loop`.
  El formato exacto de `attempts.jsonl` está en `docs/attempts-jsonl.md`. Sin
  esto no hay métricas, ni parte de datos (§6.7), ni forma de medir A.5.
- **`internal/executor` sin commitear.**

**Fase 2, lo que queda:** el bucle (§6.4), `cmd/run`, y la esclusa de salida
(§6.5) que es fase 3.

**v0.2:** Parte B entera. Ya está especificada, nadie la ha empezado.

**Deuda conocida, chica**
- `internal/executor` aparece en el historial de dos commits (`22a48f8`,
  `a781c73`) antes de que lo sacara con `git rm --cached`. Se limpia solo
  reescribiendo historia; no se hizo porque hay otro agente trabajando en el
  mismo árbol.
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
  archivos de prueba que se editen igual se encarga la reversión del bucle, que
  es lo que dice la segunda frase de A.3 — **y esa reversión todavía no existe**.
- **El parser yaml vive en `internal/kv`.** No escribas un tercero.
- **Lo que genere contratos tiene que poner `version: 1`.**
- Los mensajes de error siguen §16.6: minúscula, sin punto final, dicen qué pasó
  y qué hacer.
