# Contexto del Proyecto: devclean

> **Documento maestro de contexto para Claude y agentes de IA.**
> Resume el propósito del proyecto, la arquitectura, el estado actual de implementación (qué está hecho y qué funciona), las decisiones de diseño tomadas, y qué detalles faltan por pulir.

---

## 1. Visión General y Filosofía

**devclean** es una herramienta de terminal en Go (Go 1.22+, binario estático sin runtime externo) que convierte **requerimientos declarados como código** en cambios verificados: dirige y orquesta a múltiples agentes de IA programando en paralelo sobre un mismo repositorio, garantizando que lo único que llegue a la rama principal sea **código limpio, probado y con historial legible**.

La interfaz del producto es `devclean.spec.yml` (**Requirements as Code**), no el agente. Los agentes son parte de la ejecución; la coordinación ocurre por contratos, dependencias, interfaces, git y pruebas.

```text
Requirements as Code (devclean.spec.yml)
        ↓
Planning / Task Graph (validación estática del IR)
        ↓
Internal Task Contracts (.devclean/tasks/*.md)
        ↓
Parallel Agents (cuartos aislados)
        ↓
Task Verification (listo_cuando + esclusas por tarea)
        ↓
Integration / Feature Acceptance (comandos globales del spec)
        ↓
PR
```

### La Analogía Central
devclean actúa como una **gerencia técnica de software**:
1. El humano declara en `devclean.spec.yml` **QUÉ** debe existir: `requirements`, `rules`, `acceptance` y `constraints`. No describe tareas, archivos ni agentes.
2. devclean inspecciona el repo, genera el plan y lo compila a contratos de tarea (el IR), validando su grafo antes de gastar tokens.
3. Reparte las tareas entre múltiples agentes que trabajan en **cuartos aislados** (`git worktree`).
4. Aplica una **doble esclusa** (entrada y salida) para validar cada tarea con código determinista.
5. Sobre el conjunto integrado corre la suite del proyecto y los comandos de `acceptance` del spec: una tarea verde no implica un feature correcto.
6. El humano recibe un pull request limpio, bisectable y probado, no el desorden ni los intentos fallidos.

### Principios Rectores
1. **El historial del agente no es el historial del proyecto:** Los guardados intermedios (`wip:`) se aplana en commits semánticos (Conventional Commits) con trailer `Agent: <modelo>`.
2. **Nadie toca lo que no reclamó:** Aislamiento estricto por `tocar_solo`. Si el agente edita un archivo no autorizado o de pruebas, devclean lo revierte automáticamente antes de verificar.
3. **Verificar es trabajo de código, no del modelo:** El agente **nunca** decide si terminó. El oráculo es un comando ejecutable (`listo_cuando`) que devuelve código de salida `0` o distinto de `0`. Si no hay comando ejecutable, la tarea no existe.
4. **Lo que no se hizo, se declara:** Generación de handoffs deterministas basados en el contrato y el diff.
5. **Paralelo para trabajar, en fila para entregar:** Los agentes trabajan en paralelo en sus propios cuartos; la integración y el `ship` son estrictamente secuenciales.
6. **Sin debates entre agentes:** Los modelos no se coordinan entre sí ni votan en grupo (evita sesgos de conformidad y gastos innecesarios de tokens). Toda coordinación se mide desde los artefactos (`attempts.jsonl`).

---

## 2. Pila Tecnológica y Herramientas Integradas

- **Lenguaje:** Go 1.22+ (binario único sin CGO).
- **TUI / Terminal:** `charmbracelet/bubbletea`, `charmbracelet/lipgloss`, `charmbracelet/bubbles`, `charmbracelet/huh`. Estética de "cuarto limpio industrial" (paleta sobria, acento único `#4FB3A2`, bordes rectos, sin emojis extravagantes). Soporta `--plain` y `--json`.
- **Comandos CLI:** `spf13/cobra`.
- **Control de Versiones:** `git` (CLI nativo).
- **Gestión de PRs:** `gh` CLI para GitHub, con fallback automático a entrega local en rama `devclean/_entrega` si el repositorio no tiene remoto `origin`.
- **Motores de Agentes Integrados (Ejecutores):**
  - **Claude Code (`claude`):** Se ejecuta en modo print/headless (`claude -p <prompt> --output-format json --permission-mode bypassPermissions [--model <modelo>]`).
  - **OpenCode (`opencode`):** Se ejecuta en modo no interactivo (`opencode run <prompt> --dir <cuarto> --format json --auto [--model <modelo>]`).
- **Parseo YAML:** `gopkg.in/yaml.v3` para el spec humano (`internal/spec/yaml.go`, YAML 1.2 completo con anidación real). `internal/kv` sigue siendo el parser del frontmatter de contratos (`internal/task`), de `config` y del `Marshal` del spec.
- **Filosofía de Dependencias:** Cero frameworks pesados. Una sola dependencia de parseo (`yaml.v3`), el resto stdlib. Cero servidores escuchando en red.

---

## 3. Arquitectura del Sistema

```
devclean/
├── cmd/devclean/             # Entrypoints y comandos de la CLI
│   ├── main.go               # Arranque
│   ├── root.go               # Comando raíz, banderas globales (--plain, --json)
│   ├── up.go                 # Orquestador integral (prepara -> plan -> run -> ship)
│   ├── plan.go               # Generación y aprobación de contratos desde peticiones
│   ├── apply.go              # Aplicación de especificaciones declarativas (spec)
│   ├── completar.go          # Completado inteligente de specs rápidos (1 línea por tarea)
│   ├── run.go                # Despacho en paralelo por oleadas en cuartos
│   ├── ship.go               # Esclusa de salida y entrega (PR remoto o local)
│   ├── board.go / ps.go      # Tableros y visualizadores de estado
│   ├── doctor.go / init.go   # Diagnóstico de herramientas y setup
│   ├── fondo.go              # Desprendimiento de procesos para background
│   └── ...                   # constitution, skills, logs, report, usage, task, check
├── internal/                 # 24 paquetes modulares (100% testeados)
│   ├── task/                 # Contrato de tarea (YAML frontmatter + notas libres)
│   ├── state/                # Máquina de estados de tareas (pendiente, en_curso, lista, detenida)
│   ├── room/                 # Gestión de cuartos aislados vía `git worktree`
│   ├── loop/                 # Bucle de intentos, latidos (heartbeat), reversión y verificación
│   ├── gate/                 # Esclusa de Entrada (validaciones previas al gasto de tokens)
│   ├── ship/                 # Esclusa de Salida (10 pasos + aceptación del feature)
│   ├── executor/             # Adaptadores para los CLIs `claude` y `opencode`
│   ├── plan/                 # Lógica de planificación, saneamiento de alcance y dependencias
│   ├── spec/                 # Requirements as Code: spec humano, IR y aceptación
│   │   ├── yaml.go           #   parser yaml.v3 del spec (requirements/acceptance/constraints)
│   │   ├── validate.go       #   ValidatePlan: análisis estático del task graph
│   │   ├── state.go          #   .devclean/feature.json: intención humana separada del IR
│   │   └── spec.go           #   Apply (IR -> .devclean/tasks) y Marshal
│   ├── examiner/             # Examinador ciego de pruebas (Go y Python)
│   ├── sealed/               # Almacenamiento y verificación de suite oculta sellada (hash)
│   ├── revisor/              # Modelo que juzga intención en diffs contra el contrato
│   ├── overlap/              # Detección de colisiones en 3 niveles (textual, semántico, funcional)
│   ├── constitution/         # Gestión e inyección de `.devclean/constitution.md`
│   ├── skills/               # Sincronización e inyección de SKILL.md en el prompt
│   ├── standup/              # Parte de datos duros determinista (sin debates)
│   ├── metrics/              # Las 5 métricas del proyecto (intentos, ruido, roce, fricción, rechazo)
│   ├── budget/               # Presupuesto absoluto de tokens por corrida
│   ├── ventanas/             # Ledger global (`~/.devclean/ventanas.jsonl`) de ventanas rodantes
│   ├── kv/                   # Parser minimalista del frontmatter de contratos y config
│   ├── tui/ & ui/            # Vistas interactivas Bubble Tea y formateadores
│   └── recurse/              # Ejecución recursiva de subtareas (subagentes)
```

---

## 4. Conceptos Clave y Cómo Funcionan

### 4.1. Tareas como Código y el Contrato de Tarea
Las tareas se almacenan en `.devclean/tasks/T-00N.md`.
```yaml
---
version: 1
id: T-001
titulo: exportar clientes a CSV
porque: soporte pierde 3h/semana copiando a mano
listo_cuando: npm test -- export.spec.ts     # OBLIGATORIO: ejecutable, debe fallar hoy
tocar_solo: ["src/export/**"]                # Globs autorizados
no_tocar: ["src/auth/**", "migrations/**"]   # Globs restringidos
depende_de: ["T-000"]                        # Tareas que deben estar listas antes
expone: ["export.ToCSV(rows []Row) []byte"]  # Firmas públicas prometidas
usa: ["config.Load(p string) error"]         # Firmas consumidas de otras tareas
peso: liviana                                # liviana (haiku) | media (sonnet) | pesada (opus)
agente: backend                              # Arquetipo o rol
limite_intentos: 3
limite_lineas: 200
---
Notas libres: Enfoque sugerido para el ejecutor.
```

### 4.2. Requirements as Code: el Spec Humano (`devclean.spec.yml`)

Hay **dos niveles de especificación**, y no son alternativas sino un pipeline: el spec humano es el source, los contratos de tarea son el IR generado.

```text
devclean.spec.yml        → source humano (QUÉ debe existir)
.devclean/tasks/*.md     → task IR generado (CÓMO ejecutar y verificar)
código + commits         → resultado de ejecución
```

#### Nivel 1: Spec Humano (la interfaz recomendada)
```yaml
feature: recuperación de contraseña

requirements:            # lista simple o categorías YAML anidadas
  functional:
    - solicitar recuperación por email
    - cambiar contraseña usando el token
  security:
    - token de un solo uso

rules:                   # también acepta `reglas`; se inyectan en las notas de cada prompt
  - no modificar sesiones

acceptance:              # también `aceptacion`
  - token expirado debe rechazarse          # textual: entra al razonamiento del plan
  - criterion: integración completa         # con comando: oráculo determinista al integrar
    command: go test ./... -run TestPasswordRecovery

constraints:
  no_tocar:
    - internal/session/**

ship: true               # `up` llega hasta la entrega
```
Con `requirements` y sin `tasks`, `completarSpec` delega en `planearRequirements` (`cmd/devclean/completar.go`): el planificador produce **todo** el IR desde la intención, las reglas y la aceptación, más el lenguaje, comando de pruebas, constitución, zonas prohibidas y alcances ocupados detectados en el repo.

**Dos formas de `acceptance` con garantías distintas:**
- **Textual:** entra en el prompt del planificador y debe quedar cubierta por el plan. Por sí sola **no** es compuerta determinista; sin cobertura reconocible sale como advertencia.
- **Con `command`/`comando`:** se guarda en `.devclean/feature.json` (`spec.SaveFeatureState`) y corre sobre la rama donde ya se integraron todas las tareas. Salida distinta de cero frena la entrega.

Campos `architecture` y `delivery` están **reservados**: el parser los acepta y los ignora sin error.

#### Nivel 2: Tasks escritos a mano en el spec (interfaz avanzada, no desapareció)
1. **Modo Rápido:** Una sola línea por tarea (`- enviar magic packet por udp`). El planificador completa `listo_cuando`, `tocar_solo`, dependencias y firmas, respetando siempre lo que el humano haya escrito.
2. **Modo Completo:** Lista detallada con límites globales, reglas comunes, agentes y configuración de `ship`.
3. **Resolución de dependencias relativas:** Si no hay IDs asignados, `depende_de: ["T-001"]` se interpreta relativo a ese spec sin colisionar con tareas preexistentes en el repo.

#### 4.2.1. Análisis Estático del Task Graph (`spec.ValidatePlan`)
Corre en `Apply` sobre el IR ya con IDs, **antes** de escribir tareas o gastar implementación. Los `Issue{Level:"error"}` abortan el `Apply` completo; los `"warning"` solo se imprimen.

Errores (bloquean): `cycle` (dependencia circular), `missing_dependency`, `orphan_interface` (`usa` que nadie expone), `incompatible_interface` (existe una firma expuesta con el mismo nombre pero distinta signatura), `duplicate_interface` (dos tareas exponen lo mismo), `write_overlap` (globs de `tocar_solo` que se pisan).

Advertencias (no bloquean): `requirement_coverage`, `acceptance_coverage` (no se reconoce cobertura textual en títulos/motivos/notas/comandos) e `integration_test` (varias tareas y ninguna aceptación global). La cobertura se estima por coincidencia de palabras de más de 4 letras: señala huecos evidentes, **no** demuestra corrección semántica.

### 4.3. La Doble Esclusa

#### Esclusa de Entrada (`internal/gate`)
Antes de invocar al agente o gastar un token:
1. Contrato válido (`version: 1`, campos reconocidos).
2. `listo_cuando` es ejecutable y **falla hoy** (si ya pasa, la tarea carece de sentido).
3. `tocar_solo` no se solapa con tareas activas en la misma oleada.
4. `tocar_solo` no contiene zonas prohibidas (`migrations/**`, `go.sum`, CI, etc.).
5. `tocar_solo` no apunta a archivos de pruebas (evita que el agente altere las pruebas para falsear el verde).
6. No tiene `usa` huérfanos (no consume firmas que ninguna otra tarea exponga).

#### Esclusa de Salida (`internal/ship`)
Secuencia determinista de 9 a 10 pasos en `devclean ship`:
1. `base`: Rebase limpio sobre la rama base.
2. `historial`: Aplana los commits `wip:` internos en Conventional Commits con trailer `Agent: <modelo>`.
3. `ruido`: Escaneo de prints de debug (`console.log`, `fmt.Println`), bloques de código comentado y archivos temporales.
4. `secretos`: Detección de claves privadas, API keys y credenciales en claro.
5. `presupuesto`: Comprueba que las líneas y archivos respeten `limite_lineas`.
6. `interfaces`: Comprueba que el diff realmente implemente lo prometido en `expone`.
7. `dependencias` (condicional): Valida el grafo de imports según `reglas_import`.
8. `bisectable`: Comprueba que cada commit compile y pase la suite de pruebas individualmente.
9. `handoff`: Genera un informe determinista (qué cambió, qué NO se hizo, riesgos y verificación).
10. `pr`: Publica en GitHub (`gh pr create`) o entrega en rama local `devclean/_entrega` con descripción en `.devclean/pr/`.

En la entrega conjunta (`ship --todas` → `ship.EntregarTodas`) la secuencia sobre el resultado integrado es: esclusa por tarea → integración commit por commit → paso `integradas` (suite `pruebas` del proyecto) → paso **`aceptación`** (un paso por cada `acceptance.command` del spec, leído de `.devclean/feature.json`) → `pr`. El primer comando de aceptación que falle corta la entrega. Este paso **solo existe en `--todas`**: el `ship` de una tarea suelta no tiene conjunto integrado que aceptar.

### 4.4. Aislamiento y Bucle de Trabajo (`internal/loop`, `internal/room`)
- Cada tarea corre en un cuarto aislado: `.devclean/rooms/T-00N/` montado mediante `git worktree` sobre la rama `devclean/T-00N`.
- En cada intento:
  1. Se inyecta en el prompt: Constitución, Skills, Contrato, Interfaces expuestas/usadas, Presupuesto y el error/diagnóstico del intento anterior.
  2. El agente (`claude` u `opencode`) ejecuta modificaciones.
  3. `revertFueraDeAlcance` revierte cualquier archivo modificado fuera de `tocar_solo` o dentro de `patrones_prueba`.
  4. Se crea un commit de respaldo `wip: <id> intento <n>`.
  5. Se ejecuta `listo_cuando`.
  6. Si pasa: el `revisor` (modelo evaluador de intención) revisa el diff. Si pide cambios, el intento se marca rojo y su crítica entra en el siguiente prompt.
  7. Si falla: se capturan las últimas líneas del error y se reintenta hasta `limite_intentos`.
- **Latido (Heartbeat):** Una goroutine refresca `latido.json` cada 15 segundos. Si un proceso muere por SIGKILL (suspensión de laptop, caída de SSH), el latido queda rancio tras 90 segundos y el sistema lo detecta como corrida interrumpida/muerta, permitiendo retomarla con `run --reintentar`.

### 4.5. Detección de Solapamiento en 3 Niveles (`internal/overlap`)
1. **Nivel 1 (Textual):** `git merge-tree` entre pares de ramas activas.
2. **Nivel 2 (Semántico):** Símbolos exportados modificados en común (extraídos de `attempts.jsonl`).
3. **Nivel 3 (Funcional):** Corre **después** de la oleada sobre pares verdes sospechosos. Monta la fusión en un worktree temporal y corre los `listo_cuando` de ambas tareas para atrapar el escenario crítico: **dos ramas verdes por separado que rompen al juntarse**.

### 4.6. Examinador Ciego y Suite Oculta (`internal/examiner`, `internal/sealed`)
- Implementado para **Go** y **Python**.
- Antes de que el agente implemente, un examinador genera pruebas basadas únicamente en la frontera pública (`expone`).
- El 70% de las pruebas queda visible para el agente en el cuarto; el 30% restante se sella con hash en `.devclean/sealed/<id>/` en el repositorio principal y corre solo una vez en la esclusa de salida.

### 4.7. Presupuesto y Ventanas Rodantes (`internal/ventanas`, `internal/budget`)
- Los proveedores de IA tienen límites de gasto en ventanas de 5 horas, semanales y mensuales.
- devclean mantiene un ledger en `~/.devclean/ventanas.jsonl` (global a la máquina, compartido entre proyectos). Si una corrida va a superar el presupuesto configurado, se detiene con advertencia antes de agotar cuota.

### 4.8. Ejecución en Segundo Plano (`--fondo`)
- Disponible en `devclean up --fondo` y `devclean run --fondo`.
- Realiza las preguntas interactivas primero; una vez listo el entorno, se desprende de la terminal (`setsid` en Unix, flags de proceso desacoplado en Windows) y redirige la salida a `.devclean/corridas/<fecha>.log`.
- Sobrevive al cierre de terminal o pérdida de conexión.

---

## 5. Qué Está Hecho y Qué Funciona

El proyecto alcanzó **v1.1.0** (18 de septiembre de 2026), el estado objetivo de **Requirements as Code**: `devclean.spec.yml` declara requirements, reglas, aceptación y restricciones; los contratos de tarea quedan como IR compatible; el grafo se valida antes de ejecutar y la aceptación global corre sobre el conjunto integrado. La base v1.0.0 (14 de septiembre de 2026) sigue vigente en todo lo demás.

**Verificado de punta a punta con agentes reales (17 de septiembre de 2026):** `devclean up "<petición>" --agentes 3 --ship` sobre un repo Go vacío planifica 3 tareas, las corre en paralelo con `claude` (haiku), genera la suite ciega de cada una, pasa los 10 pasos de la esclusa de salida y entrega un PR local que mergea verde. También verificados por separado: `init`, `doctor`, `check`, `plan --aprobar`, `run`, `ship T-00N`, `ship --todas`, `board`, `report`, `standup`.

- **Suite de pruebas:** `go test ./...` pasa al **100% verde** en todos los paquetes (24 paquetes en `internal/` y `cmd/devclean`).
- **Linter:** `go vet ./...` 100% limpio.
- **Flujo end-to-end `up`:** Configura entorno, autodetecta herramientas, genera planes, ejecuta en paralelo, revisa y entrega.
- **Spec rápido:** Soporte para sintaxis simplificada de una línea por tarea con autocompletado de contratos.
- **Entrega local:** `ship` funciona sin remoto git, creando la rama de entrega y el PR en Markdown dentro de `.devclean/pr/`.
- **Doble esclusa completa:** Validaciones de entrada y los 10 pasos de compuerta en salida, más el paso `aceptación` sobre el conjunto integrado en `ship --todas`.
- **Requirements as Code:** spec humano con `requirements`/`rules`/`acceptance`/`constraints`, plan generado completo cuando no hay `tasks`, `ValidatePlan` sobre el IR y `.devclean/feature.json` guardando la intención humana separada del IR.
- **Solapamiento en 3 niveles:** Textual, semántico y funcional plenamente funcionales.
- **TUI interactivo y modos headless:** Bubble Tea interactivo, `--plain` para CI/tuberías y `--json` estructurado.

---

## 6. Decisiones de Alcance Tomadas (No Reabrir)

- **El examinador ciego cubre solo Go y Python:** Validar sintaxis sin ejecutar exige parsers confiables en stdlib. Rust y Node quedan fuera hasta que se justifique arrastrar toolchains o dependencias pesadas.
- **Sin Homebrew:** La distribución se realiza mediante `scripts/install.sh`, releases de GitHub (binarios GoReleaser) y `go install`.
- **El nombre del campo de CLI es `cli` y no `ejecutor`:** Evita colisiones con el rol `ejecutor` en `proveedores` debido a las particularidades del parser `kv`.
- **El spec humano se parsea con `yaml.v3`; `kv` se queda en el frontmatter (v1.1.0):** el spec necesita anidación real (`requirements` por categorías, `acceptance` con `criterion`/`command`, `constraints`) y `kv` pisa claves repetidas a distinta profundidad. Reescribir un parser YAML 1.2 correcto cuesta más que una dependencia de la stdlib extendida. El frontmatter de contratos y `config` son planos: ahí `kv` sigue siendo suficiente y no se migra.
- **La aceptación textual no es compuerta:** una aceptación sin `command` entra al prompt del planificador y se reporta como advertencia si nadie la cubre, pero nunca frena la entrega. Fingir determinismo a partir de coincidencia de palabras sería peor que declarar el límite.
- **Sin debates ni auto-reportes de agentes:** El standup se calcula de forma pura y determinista desde los artefactos.

---

## 7. Qué Falta y Detalles por Pulir (Roadmap y Mejoras Pendientes)

Aunque el núcleo es sólido y funcional, existen áreas identificadas que requieren pulido y evolución:

### 7.1. Mejoras Técnicas y de Precisión
1. **Mutation Score para el Examinador Ciego (§6.8):** Falta integrar análisis de mutación (ej. `go-mutesting`) para verificar que las suites generadas realmente detecten fallos y no sean triviales.
2. **Validación de Firmas por AST (§6.10):** En el paso `interfaces` de la esclusa de salida, la comparación se hace por nombre de función/símbolo (`task.NombreDeFirma`). Falta implementar análisis sintáctico por AST para validar signaturas completas respetando tipos.
3. **Detección de Duplicación de Código entre Ramas (§6.10):** Comparación estructural de funciones nuevas entre ramas activas de una misma oleada para alertar si dos agentes están reimplementando la misma utilidad.
4. **`constraints.no_tocar` no se reaplica en el camino automático:** `spec.Parse` añade esos globs al `no_tocar` de cada tarea que venía escrita en el spec, pero `planearRequirements` agrega sus contratos **después** de parsear, así que el IR generado no los recibe. Con `requirements` y sin `tasks`, esa frontera global hoy no se garantiza: hay que revisar los contratos antes de ejecutar.
5. **`spec.Marshal` no escribe `constraints`:** exportar un spec (`plan --export-spec`) pierde el `no_tocar` global, así que el round-trip no es fiel.
6. **`parseLegacy` quedó muerto:** ~200 líneas en `internal/spec/spec.go` (`parseLegacy`, `processBlock`, `parseTaskList`, `parseTaskChunk`, `buildTaskFromMap`) sin ningún llamador desde que `Parse` pasó a `yaml.go`. `go vet` no lo señala. Borrar o justificar.

### 7.2. Motores de Agentes y Modelos
1. **Modo API Directa:** Actualmente la ejecución depende obligatoriamente de los binarios instalados de `claude` (Claude Code) u `opencode`. Falta agregar un adaptador que permita llamadas directas a APIs (Anthropic, OpenAI, DeepSeek) sin requerir los CLIs externos.
2. **Tercer Proveedor de CLI:** Soporte para herramientas adicionales como Aider, Gemini CLI o Codex CLI.
3. **Manejo de Errores de CLI y Timeouts:** Optimizar los diagnósticos cuando Claude Code u OpenCode fallan por problemas de red o cuota del proveedor, evitando que el bucle consuma intentos cuando el fallo es de infraestructura.

### 7.3. Flujos de Tareas y Experiencia de Usuario
1. **Soporte para Forjas Adicionales:** Integrar soporte nativo para GitLab (`glab`) o Bitbucket en el paso de entrega remota de `ship`.
2. **Gestión de Fallos en Cascada en Specs:** Cuando una tarea con muchas dependencias (`depende_de`) se detiene o rechaza, refinar la cancelación limpia de las tareas dependientes en la misma oleada.
3. **El plan no cierra la costura semántica entre `expone` y `usa`:** verificado con 5 agentes construyendo una calculadora (lexer → parser → eval → cli). Las 5 tareas verdes, integración verde, y aun así `-2^2` devolvía `unknown binary operator: ^`: el lexer y el parser tenían `^` en su contrato y el del evaluador nunca lo pidió. Cada `listo_cuando` pasa, el nivel funcional de solapamiento tampoco lo ve (corre los mismos comandos) y nadie prueba la costura. **Mitigado, no resuelto, en v1.1.0:** un `acceptance.command` global atrapa ese caso al integrar, y `ValidatePlan` avisa (`integration_test`) cuando un feature multitarea no declara aceptación. Pero el agujero de fondo sigue: `^` nunca aparece en el `usa` del evaluador, así que `orphan_interface` e `incompatible_interface` no disparan. Falta que el planificador derive los casos que una tarea debe cubrir de lo que sus consumidoras prometen.
4. **Suite oculta dependiente del examinador:** el 30% sellado solo existe si el modelo examinador devuelve el bloque `hidden` en su JSON. Si no lo devuelve, se sella nada y el paso `suite_oculta` de la esclusa se omite en silencio; conviene registrar ese fallo del examinador en lugar de degradar sin dejar rastro.
5. **Pulido del Feedback Loop en `listo_cuando`:** Cuando un comando de pruebas produce volcados de error gigantescos (ej. stack traces masivos en Node/Java), filtrar inteligentemente el error para no saturar la ventana de contexto del modelo en el siguiente intento.

---

## 8. "Gotchas" y Advertencias Críticas para Desarrollar en este Repo

- **Un `listo_cuando` que sale con 0 sin ejecutar pruebas NO es verde (`loop.SinPruebas`):** `go test ./pkg/...` sobre un paquete sin archivos de prueba devuelve 0. Sin ese filtro, una tarea cuyo examen ciego degradó se entregaba "verde" sin que nada la juzgara. Quien decida verde por código de salida tiene que pasar por `loop.SinPruebas` (lo hacen el bucle y `listoPadreVerde` de la recursión).
- **La reversión de alcance (A.3) se mide contra el commit con que arrancó el intento, no contra `git status`:** el agente real commitea por su cuenta dentro del cuarto (la skill `implement` lo hace) y con el árbol limpio la reversión quedaba ciega: sus propias pruebas y los archivos fuera de alcance llegaban al PR. `revertFueraDeAlcance` recibe ese commit (`antes`) y restaura desde él.
- **El examinador ciego sí examina paquetes que todavía no existen:** cuando el directorio no tiene código, el nombre del paquete sale del prefijo de `expone` (`numeros.Media(...)` → `numeros`) y la suite lo declara en su encabezado. Antes renunciaba, y toda tarea de paquete nuevo quedaba sin suite. Sigue sin examinar `package main` (Go no deja importarlo) ni stacks sin parser (rust, node).
- **La veda de rutas de prueba (A.3) se decide por TAREA, no por lenguaje (`examiner.Examinable`):** solo se veda donde hay examen ciego que proteger. Una tarea sobre `cmd/algo` (`package main`) no se puede examinar —Go no deja importar un main— y vedarle las pruebas la dejaba imposible de terminar.
- **La suite que escribe el examinador resuelve sus propios imports:** el modelo se olvida de declarar el paquete hermano que usa (`ast.Number{}` sin importar ast), y esa suite no compila nunca ni la puede arreglar el implementador. `importsFaltantes` los resuelve contra el módulo y la stdlib; lo que no se resuelve descarta la suite (mejor sin examen que con uno roto).
- **La suite oculta solo se quema al aprobar:** quemarla al fallar dejaba el paso omitido en el siguiente `ship` y la tarea frenada salía en un PR con solo repetir el comando. La que falla queda sellada y su salida va a `.devclean/runs/<id>/suite-oculta.log`.
- **`gate.Run` devuelve 6 chequeos:** Búscalos por nombre, nunca por índice en el array.
- **El chequeo 0 siempre es `contrato válido`:** Llama a `Validate()`. Todo contrato nuevo debe incluir `version: 1` (`task.Version`).
- **Hay dos parsers y cada uno tiene su territorio:** el spec humano se parsea con `yaml.v3` en `internal/spec/yaml.go` (`spec.Parse`); el frontmatter de contratos (`internal/task`) y `config` siguen en `internal/kv`. No migres uno al otro sin consensuarlo, y no agregues una tercera librería de parseo. En `kv` sigue viva la trampa de las claves repetidas a distinta profundidad: se pisan.
- **`spec.ValidatePlan` distingue error de advertencia y solo el error aborta:** `Apply` junta los `Level=="error"` y falla con `plan inválido: ...`; las advertencias (`requirement_coverage`, `acceptance_coverage`, `integration_test`) únicamente se imprimen. Búscalos por `Code`, nunca por índice. La cobertura se estima por palabras de más de 4 letras: no la trates como prueba semántica.
- **La aceptación del feature vive en `.devclean/feature.json`, no en los contratos:** `apply` la guarda (`SaveFeatureState`) y `ship --todas` la lee (`LoadFeatureState`). Si ese archivo no existe, el paso `aceptación` simplemente no corre y la entrega sigue: borrar `.devclean/` borra la compuerta global. Solo las aceptaciones **con `command`** llegan ahí como compuerta (`AcceptanceCommands`).
- **`planearRequirements` agrega tareas después de `Parse`, así que `constraints.no_tocar` no las alcanza:** los globs globales solo se inyectan a las `tasks` que venían escritas en el spec. Si toques esa ruta, aplica los globs al IR generado o el spec miente sobre su frontera.
- **El nivel funcional de overlap corre DESPUÉS de la oleada:** Si se corre antes, las ramas de las tareas están vacías y no detecta nada. Requiere `Resultado.Arbol` proveniente de `mergeTree`.
- **`standup.Analizar` requiere latidos EN CRUDO (`LeerLatidosCrudos`):** La diferencia temporal entre latido fresco y rancio separa una tarea atascada (`ATASCO`) de una que murió por kill (`MUERTA`).
- **El ledger de ventanas es global del usuario (`~/.devclean/ventanas.jsonl`):** No se resetea borrando la carpeta `.devclean/` del proyecto.
- **Convenciones de Mensajes de Error (§16.6):** Frases en minúscula, sin punto final, sin disculpas, indicando qué ocurrió y qué hacer para solucionarlo (ej. `tarea rechazada · listo_cuando no ejecutable · edita T-001 y reintenta`).
