# Contexto del Proyecto: devclean

> **Documento maestro de contexto para Claude y agentes de IA.**
> Resume el propósito del proyecto, la arquitectura, el estado actual de implementación (qué está hecho y qué funciona), las decisiones de diseño tomadas, y qué detalles faltan por pulir.
> Para cómo llegó hasta aquí —la intención original, la cronología y lo que se descartó— está `docs/HISTORIA.md`, el único documento histórico del repo.

---

## 1. Visión General y Filosofía

**devclean** es una herramienta de terminal en Go (Go 1.22+, binario estático sin runtime externo) que convierte **requerimientos declarados como código** en cambios verificados: dirige y orquesta a múltiples agentes de IA programando en paralelo sobre un mismo repositorio, garantizando que lo único que llegue a la rama principal sea **código limpio, probado y con historial legible**.

La interfaz del producto es `devclean.spec.yml` (**Requirements as Code**), no el agente. Los agentes son parte de la ejecución; la coordinación ocurre por contratos, dependencias, interfaces, git y pruebas.

```text
Requirements as Code (devclean.spec.yml)
        ↓
Esqueleto (modelo grande: plano en código: estructura, ARCHITECTURE.md, interfaces y stubs con casos)
        ↓
Internal Task Contracts (.devclean/tasks/*.md · una tarea de relleno por stub)
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
  - **Claude Code (`claude`):** Se ejecuta en modo print/headless (`claude -p <prompt> --output-format stream-json --verbose --permission-mode bypassPermissions <contexto limpio> --tools <por rol> [--model <modelo>]`). Herramientas por rol (`executor.Rol`): implementador `Bash,Read,Edit,Write`, planificador `Read,Bash`, examinador/revisor/constitución ninguna. Un 429 espera al `resetsAt` de la respuesta y relanza la misma invocación: no gasta intento ni escala.
  - **OpenCode (`opencode`):** Se ejecuta en modo no interactivo (`opencode run <prompt> --dir <cuarto> --format json --auto --agent devclean-<rol> [--model <modelo>]`). Los agentes llegan por `OPENCODE_CONFIG_CONTENT` sin tocar la config del usuario: el implementador sin webfetch/task/todo/skill, el de texto sin herramientas, el planificador con read y bash. Apagar `skill` saca las skills de `~/.config/opencode/skills`.
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
│   │   └── spec.go           #   Apply (IR -> .devclean/tasks, constraints) y Marshal
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
Con `requirements` y sin `tasks` (y con `up "<petición>"`), `completarSpec` delega en `planearRequirements` → `planearEsqueleto` (`cmd/devclean/esqueleto.go`). Ver 4.2.2.

**Dos formas de `acceptance` con garantías distintas:**
- **Textual:** entra en el prompt del planificador y debe quedar cubierta por el plan. Por sí sola **no** es compuerta determinista; sin cobertura reconocible sale como advertencia.
- **Con `command`/`comando`:** se guarda en `.devclean/feature.json` (`spec.SaveFeatureState`) y corre sobre la rama donde ya se integraron todas las tareas. Salida distinta de cero frena la entrega.

`constraints.no_tocar` se aplica en `Apply`, no en `Parse`: los globs se suman al `no_tocar` de **todo** contrato que se vaya a escribir, venga del YAML o lo haya generado el planificador. La restricción del humano sobrevive el viaje spec → IR.

Campos `architecture` y `delivery` están **reservados**: el parser los acepta y los ignora sin error.

#### Nivel 2: Tasks escritos a mano en el spec (interfaz avanzada, no desapareció)
1. **Modo Rápido:** Una sola línea por tarea (`- enviar magic packet por udp`). El planificador completa `listo_cuando`, `tocar_solo`, dependencias y firmas, respetando siempre lo que el humano haya escrito.
2. **Modo Completo:** Lista detallada con límites globales, reglas comunes, agentes y configuración de `ship`.
3. **Resolución de dependencias relativas:** Si no hay IDs asignados, `depende_de: ["T-001"]` se interpreta relativo a ese spec sin colisionar con tareas preexistentes en el repo.

#### 4.2.2. El Esqueleto (`internal/esqueleto`, `planearEsqueleto`)
El modelo grande no reparte prosa: escribe el **plano en código**, sin lógica ni pruebas (cada línea suya la paga el modelo caro; en closet el arquitecto que además escribía pruebas y cableado fue el 70% del gasto). Corre con el modelo `planificador`, herramientas de escritura, en el cuarto de la primera tarea (`T-00N`), y deja:
- estructura, manifiestos, `.gitignore` y dependencias instaladas;
- `ARCHITECTURE.md` en la raíz: módulos, responsabilidades, dependencias permitidas, flujo de datos. Es la fuente de verdad que el siguiente cambio **evoluciona** (el prompt se lo pasa si ya existe);
- tipos, entidades, interfaces y puertos solo como **declaraciones**;
- todo lo demás como **stub**, también el cableado: firma exacta, comentario de contrato (entradas, salida, errores, a quién llama y quién lo usa, y `Casos:` con 2 a 5 ejemplos) y cuerpo que lanza `devclean: sin implementar` (`esqueleto.Marca`).

**Sobre código existente no hay stubs:** lo que ya funciona se cambia, no se reemplaza. Una tarea que toca archivos que ya estaban en el commit base lleva su contrato con `Casos:` en `como`, su `listo_cuando` corre una prueba nueva del comportamiento nuevo y recibe `notaCambio` en vez de `notaRelleno` (`notaPara` decide por la marca). El prompt manda respetar el stack, las librerías y el sistema de diseño que ya hay.
- **Docker por defecto:** un Dockerfile por servicio, `compose.yaml` en la raíz (`docker compose up --build` levanta todo), `.env.example` y `.dockerignore`. Se apaga escribiendo "sin docker" en el spec (una librería o un CLI no se despliegan).
- **Interfaz web:** el sistema de diseño que el proyecto ya tenga; solo si no hay, Tailwind y la librería estándar de su framework instaladas con su CLI (componentes generados, no tokens).

Las pruebas las escribe **cada tarea de relleno** a partir de los `Casos:` de su contrato, con fakes para las dependencias (así el relleno es una sola ola en paralelo). Una tarea final escribe la prueba de punta a punta: depende de todas y su `listo_cuando` es `integracion`.

Responde un JSON con `verificar` (build/typecheck que pasa con los stubs), `integracion` (la prueba de punta a punta), `pruebas` y `tareas` (una por stub, con el stub y su archivo de prueba en `tocar_solo`). **`esqueleto.Problemas` lo verifica sin modelo:** existe `ARCHITECTURE.md`, `verificar` pasa, cada tarea tiene `listo_cuando` y `tocar_solo`, cada archivo no-prueba de `tocar_solo` existe, trae al menos un stub con la marca y un `Casos:` (`contrato`), y alguna tarea tiene `integracion` como `listo_cuando`. La marca y los `Casos:` del archivo se exigen solo en archivos **nuevos** respecto de `Verificacion.Base` (el commit con que arrancó el cuarto); un cambio a un archivo existente exige `Casos:` en `como`. Con docker: `compose.yaml`, `.dockerignore` y, si docker está instalado, `docker compose config -q`. No corre los `listo_cuando` (las pruebas todavía no existen), salvo `integracion`: si falla hoy, una tarea tiene que tenerla como `listo_cuando`; si ya pasa, es la regresión de un proyecto existente y ninguna tarea puede usarla (la esclusa la rechazaría por "ya pasa"). Un cambio solo visual deja `integracion` como regresión sin tarea final; exigir siempre una tarea final contradecía la regla anterior y el arquitecto no podía cumplir las dos. Si algo falla, el arquitecto recibe la lista con el mismo prompt delante (caché) y corrige en el mismo cuarto, hasta `correccionesEsqueleto` veces. `room.Ensure` reusa el cuarto: un esqueleto cortado se corrige, no se rehace.

Al pasar: commit `wip: T-00N esqueleto`, estado **lista** y contrato con `listo_cuando: <verificar>`. Cada stub es una tarea de relleno con `depende_de: [T-00N]`, **sin `expone`/`usa`** (las firmas las valida el compilador) y la nota `notaRelleno`. `verificar`, `integracion` y (con docker instalado) `docker compose build` entran como `acceptance.command`. `run` integra el esqueleto por `sembrarVerdesPrevias`, igual que cualquier verde previo. Log del arquitecto en `.devclean/runs/T-00N/esqueleto-N.log`.

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
  1. Se inyecta en el prompt, primero lo común y después lo de la tarea: `skills.Base` (cómo trabajar, código limpio resumido, no narrar), Constitución, reglas, arquitectura común; luego las skills que la tarea declara en `skills:` (o las de su rol si el contrato no trae el campo), Contrato, Interfaces expuestas/usadas, árbol y firmas recortados, Presupuesto y el error/diagnóstico del intento anterior.
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

Versión actual: **v1.7.x** (28 de septiembre de 2026). **v1.1.0** (18 sep) alcanzó el estado objetivo de **Requirements as Code**: `devclean.spec.yml` declara requirements, reglas, aceptación y restricciones; los contratos de tarea quedan como IR compatible; el grafo se valida antes de ejecutar y la aceptación global corre sobre el conjunto integrado. **v1.7.0** cambió cómo se genera el plan (esqueleto plano, ver 4.2.2) y cómo se gasta la cuota: presupuesto con caché, modelos gratis de opencode mezclados con claude, tope de 3 agentes pagados, escalera sin opus. **v1.7.1** vuelve incremental el mantenimiento por spec: un `up` sobre el mismo spec no replanea y uno editado solo manda lo que cambió (`spec.IntencionFile`).

**Verificado de punta a punta con agentes reales (17 de septiembre de 2026, v1.0):** `devclean up "<petición>" --agentes 3 --ship` sobre un repo Go vacío planifica 3 tareas, las corre en paralelo con `claude` (haiku), genera la suite ciega de cada una, pasa los 10 pasos de la esclusa de salida y entrega un PR local que mergea verde. También verificados por separado: `init`, `doctor`, `check`, `plan --aprobar`, `run`, `ship T-00N`, `ship --todas`, `board`, `report`, `standup`. El esqueleto (con pruebas del arquitecto) corrió real en closet (monorepo Python + frontend).

**Sin verificar con agentes reales todavía:** el esqueleto plano de v1.7.0 (cada tarea escribe su prueba desde `Casos:`), la mezcla opencode free + claude, y el spec incremental de v1.7.1. Solo tienen pruebas unitarias.

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
1. **Mutation Score para el Examinador Ciego:** Falta integrar análisis de mutación (ej. `go-mutesting`) para verificar que las suites generadas realmente detecten fallos y no sean triviales.
2. **Validación de Firmas por AST:** En el paso `interfaces` de la esclusa de salida, la comparación se hace por nombre de función/símbolo (`task.NombreDeFirma`). Falta implementar análisis sintáctico por AST para validar signaturas completas respetando tipos.
3. **Detección de Duplicación de Código entre Ramas:** Comparación estructural de funciones nuevas entre ramas activas de una misma oleada para alertar si dos agentes están reimplementando la misma utilidad.
4. **Las pruebas las escribe el agente barato que rellena, desde los `Casos:` del contrato:** el mismo que implementa, así que puede escribir una prueba complaciente. El revisor es el contrapeso (sin `expone` no hay examinador ciego ni suite oculta). Falta medir su cobertura real (ver el punto 1 de mutation score).
5. **El presupuesto cuenta la caché (`loop.Tokens.Gasto`):** entrada + salida + leída×0.1 + escrita×1.25. Antes `budget`, `metrics` y el ledger de ventanas sumaban solo `entrada + salida` y con `claude` casi todo llega como caché (T-008: 129 de entrada contra 789k leídos y 42k escritos). Presupuestos ya configurados saltan antes que en v1.1. La salida sigue pesando 1× aunque cueste 5×: el tope compara tokens, no dinero.
   - **Corrida A del benchmark (snake, 13 contratos fijos, 23 sep 2026):** 18 intentos contra 13 del original. El recorte de firmas/árbol **no** causó reintentos: de los 5, **3 fueron 429** (límite de 5h agotado; T-011 quemó dos intentos con sonnet y el de opus sin gastar un token) y **2 fueron bugs reales de haiku que atrapó el revisor** (T-002 encerraba a la serpiente en el nivel CRUZ; T-006 lanzaba con `mapKey(undefined)`), ambos con el requisito escrito en el prompt. Contar esos 429 como intento fallido es el pendiente 7.2.3.
6. **Benchmark de prompts:** `DEVCLEAN_BENCH_DIR=<proyecto> go test -tags bench -run TestBenchPrompts -v ./internal/loop/` mide el tamaño del prompt por tarea, el prefijo común y los intentos por tarea de la última corrida (`DEVCLEAN_BENCH_TAREA=T-00N` vuelca un prompt). Los intentos son de la corrida que ya pasó: para juzgar un cambio en el prompt hay que volver a correr el plan con él.

### 7.2. Motores de Agentes y Modelos
1. **Modo API Directa:** Actualmente la ejecución depende obligatoriamente de los binarios instalados de `claude` (Claude Code) u `opencode`. Falta agregar un adaptador que permita llamadas directas a APIs (Anthropic, OpenAI, DeepSeek) sin requerir los CLIs externos.
2. **Tercer Proveedor de CLI:** Soporte para herramientas adicionales como Aider, Gemini CLI o Codex CLI.
3. **Manejo de Errores de CLI y Timeouts:** Optimizar los diagnósticos cuando Claude Code u OpenCode fallan por problemas de red o cuota del proveedor, evitando que el bucle consuma intentos cuando el fallo es de infraestructura.

### 7.3. Flujos de Tareas y Experiencia de Usuario
1. **Soporte para Forjas Adicionales:** Integrar soporte nativo para GitLab (`glab`) o Bitbucket en el paso de entrega remota de `ship`.
2. **Gestión de Fallos en Cascada en Specs:** Cuando una tarea con muchas dependencias (`depende_de`) se detiene o rechaza, refinar la cancelación limpia de las tareas dependientes en la misma oleada.
3. **La costura semántica entre `expone` y `usa` ya no queda sin probar (v1.2.0), pero su calidad depende de un modelo:** el caso de origen fue una calculadora con 5 agentes (lexer → parser → eval → cli): las 5 tareas verdes, integración verde, y `-2^2` devolvía `unknown binary operator: ^` porque el lexer y el parser tenían `^` en su contrato y el del evaluador nunca lo pidió. Ni los `listo_cuando` ni el nivel funcional del solapamiento lo ven. Con el esqueleto, la prueba de punta a punta la escribe la tarea final (depende de todas) y corre como aceptación al integrar. Lo que falta: **medir** que cubra de verdad.
4. **Suite oculta dependiente del examinador:** el 30% sellado solo existe si el modelo examinador devuelve el bloque `hidden` en su JSON. Si no lo devuelve, se sella nada y el paso `suite_oculta` de la esclusa se omite en silencio; conviene registrar ese fallo del examinador en lugar de degradar sin dejar rastro.
5. **Feedback de `listo_cuando` (resuelto en lo básico):** el error que entra al siguiente prompt son las últimas 8 líneas (800 caracteres) **sin marcos de stack** (`loop.sinMarcos`): en jest o Java los marcos empujaban fuera el `Expected/Received`. No hay filtro por lenguaje más allá de eso.

---

## 8. "Gotchas" y Advertencias Críticas para Desarrollar en este Repo

- **Un `listo_cuando` que sale con 0 sin ejecutar pruebas NO es verde (`loop.SinPruebas`):** `go test ./pkg/...` sobre un paquete sin archivos de prueba devuelve 0. Sin ese filtro, una tarea cuyo examen ciego degradó se entregaba "verde" sin que nada la juzgara. Quien decida verde por código de salida tiene que pasar por `loop.SinPruebas` (lo hacen el bucle y `listoPadreVerde` de la recursión).
- **La reversión de alcance se mide contra el commit con que arrancó el intento, no contra `git status`:** el agente real commitea por su cuenta dentro del cuarto (la skill `implement` lo hace) y con el árbol limpio la reversión quedaba ciega: sus propias pruebas y los archivos fuera de alcance llegaban al PR. `revertFueraDeAlcance` recibe ese commit (`antes`) y restaura desde él.
- **El examinador ciego sí examina paquetes que todavía no existen:** cuando el directorio no tiene código, el nombre del paquete sale del prefijo de `expone` (`numeros.Media(...)` → `numeros`) y la suite lo declara en su encabezado. Antes renunciaba, y toda tarea de paquete nuevo quedaba sin suite. Sigue sin examinar `package main` (Go no deja importarlo) ni stacks sin parser (rust, node).
- **La veda de rutas de prueba se decide por TAREA, no por lenguaje ni por proyecto (`examiner.Examinable`):** solo se veda donde hay examen ciego que proteger. Una tarea sobre `cmd/algo` (`package main`) no se puede examinar —Go no deja importar un main— y vedarle las pruebas la dejaba imposible de terminar. **La esclusa y el bucle tienen que usar la misma regla:** `run.go` calculaba `patronesPruebaTarea` para el bucle pero le pasaba a `gate.Run` los patrones del proyecto, así que la tarea de integración —que no expone nada y solo escribe pruebas— era rechazada por `sin rutas de prueba` antes de correr. `check.go` tenía la misma asimetría.
- **La suite que escribe el examinador resuelve sus propios imports:** el modelo se olvida de declarar el paquete hermano que usa (`ast.Number{}` sin importar ast), y esa suite no compila nunca ni la puede arreglar el implementador. `importsFaltantes` los resuelve contra el módulo y la stdlib; lo que no se resuelve descarta la suite (mejor sin examen que con uno roto).
- **La suite oculta solo se quema al aprobar:** quemarla al fallar dejaba el paso omitido en el siguiente `ship` y la tarea frenada salía en un PR con solo repetir el comando. La que falla queda sellada y su salida va a `.devclean/runs/<id>/suite-oculta.log`.
- **El prompt del agente empieza por lo común del plan, byte a byte (`loop.promptPara`):** constitución, reglas y arquitectura sin árbol ni firmas; después skills del rol, el contrato, y el árbol y las firmas recortados a `tocar_solo`/`usa`/`expone` (`plan.RecortarArquitectura`). Nada con id, ruta de cuarto o fecha puede entrar antes de `Tarea T-00N`: rompe el caché compartido (`TestPromptsDeUnPlanCompartenElPrefijoComun`). Las notas siguen guardándose con la arquitectura entera en cada contrato; se separan al armar el prompt por las marcas `plan.Marca*`.
- **Las skills llegan como texto, nunca por el mecanismo de skills de Claude Code:** medido en el snake, `frontend-design` estuvo disponible como skill en 13 agentes y se invocó 0 veces; la única invocada fue `code-review`, y solo porque el texto de `implement` lo pedía. `skills:` en el contrato (nil = las del rol, `[]` = ninguna) elige qué texto entra; el catálogo es `.agents/skills` menos `implement` y `caveman`, que reemplaza `skills.Base`.
- **Lo visual lo juzga quien ve capturas, no las pruebas (`internal/capturas`):** en closet, 9 tareas "liquid glass" salieron verdes en su primer intento porque sus pruebas solo miraban que existieran `.glass` y `backdrop-blur`; en la pantalla no había vidrio. El arquitecto declara `pantallas` (`levantar` sirve la app en `$PORT`, `url`, `rutas`) y se guardan en `config.yml` (`pantallas_*`). Con eso: (1) el arquitecto recibe capturas de cómo se ve hoy antes de planear (`Pedido.Capturas`, `runs/T-00N/antes/`); (2) cada tarea que toca UI, con código verde y aprobado, pasa por `revisorVisualEnBucle` (rol `revisor_visual` o el modelo pesado, `executor.RolVisual` = solo `Read`), que juzga las capturas del cuarto; su crítica entra al siguiente intento, como mucho `loop.TopeVisual` (2) veces por tarea; (3) `ship --todas` pone capturas del resultado integrado en el PR (`.devclean/pr/capturas/`). Todo degrada en abierto: sin navegador (`capturas.Navegador`: chrome-headless-shell de Playwright o chromium/chrome; Brave se cuelga en headless) o si la app no levanta, se avisa y se sigue. `tomarCapturas` instala chrome-headless-shell con npx la primera vez.
- **En un pedido visual el arquitecto reescribe las pruebas que fijan clases:** en closet hizo el liquid glass "aditivo" para no romper pruebas viejas que fijaban `rounded-xl`/`bg-card`, y el resultado apenas cambió. Ahora el prompt le pide reescribirlas o borrarlas en su esqueleto, y la guía de UI dice que las pruebas verifican lo que el usuario ve y hace, no clases.
- **La guía de UI entra por archivo, no por rol (`skills.UI`, `skills.TocaUI`):** toda tarea con un archivo de interfaz en `tocar_solo` (`.tsx/.jsx/.vue/.svelte/.astro/.html/.css/.scss` y plantillas de servidor: `.erb`, `.blade.php`, `.twig`, `.jinja`, `.hbs`, `.razor`…) recibe ~1 KB de reglas neutrales al stack: usar el sistema de diseño y los componentes del proyecto, plantilla de página con estados cargando/vacío/error, mobile-first, formularios, accesibilidad. Las tareas del esqueleto no traen `agente`, así que por rol no recibían nada. `frontend-design` salió del catálogo por defecto y del rol `frontend`: eran 9.4 KB por intento pidiendo un diseño "distintivo" y huir de lo estándar.
- **claude corre con contexto limpio (`executor.contextoLimpio`):** sin settings de usuario (hooks, plugins, skills), sin MCP y con las secciones dinámicas fuera del prompt de sistema. No agregues `--disable-slash-commands`: también apaga las skills de `.claude/skills` del proyecto.
- **La esclusa lee `listo_cuando` como `sh -c`, igual que el bucle (`gate.programas`):** revisa el primer programa de cada comando simple y salta builtins (`cd`, `export`, …). Mirar solo el primer token rechazó las 18 tareas de un monorepo (`cd backend && …`) con "comando no encontrado: cd", y dejaba pasar un programa inexistente detrás del `&&`.
- **opencode topa la salida en 32000 tokens aunque el modelo dé más (`entornoOpenCode`):** `min(limit.output, OPENCODE_EXPERIMENTAL_OUTPUT_TOKEN_MAX ?? 32000)`. `deepseek-v4-flash` gastó los 32000 razonando el plan y cortó con `finish: "length"` sin escribir el JSON; opencode sale con 0 y el error llegaba como "no devolvió un plan JSON". devclean sube el tope a 128000. Para diagnosticar un plan fallido: `opencode session list` y `opencode export <sesión>` en el repo, mirar `finish` y `tokens`.
- **El planificador numera con los ids reales (`plan.Contexto.PrimerID`) y ve el `expone` previo (`Expuestas`):** sin eso, con T-001..T-018 en el repo el modelo numeró su plan desde T-002 y `traducirDependencias` lo leyó por posición: `T-005` pasó a ser la quinta tarea y el plan salió lleno de ciclos. Además reescribió `get_db() -> Iterator[Session]` de T-001 como `Generator[Session]` y `ValidatePlan` lo rechazó por incompatible. La traducción por posición solo aplica a ids que no son del plan ni previos.
- **`plan.Parse` escapa saltos de línea crudos dentro de strings (`EscaparControles`):** los modelos escriben `arquitectura` con párrafos reales en vez de `\n` y el decoder tiraba un plan de 49k tokens. Fixture real en `internal/plan/testdata/plan-saltos-crudos.txt`.
- **Un plan pagado no se tira por un fallo posterior (`planGuardado`):** la respuesta cruda queda en `.devclean/plan-crudo.json` con el sha256 del prompt. Mientras no se escriba ningún contrato el prompt sale idéntico y se reusa sin llamar al modelo; al escribir uno, `PrimerID`/`Expuestas` cambian y el caché deja de pegar solo. Envuelve únicamente al planificador (`plan`, `planearRequirements`, completar): el revisor y la recursión usan `generadorPlan` directo.
- **`ValidatePlan` compara `usa` contra `expone` en forma canónica (`firmaCanonica`):** sin `def`/`func` al inicio y sin la lista de bases de una clase. El modelo expuso `class Garment(Base): id, ...` y en `usa` copió `class Garment: id, ...`: 9 `incompatible_interface` tiraron un plan de 31 tareas. Los tipos sí cuentan: `Iterator[Session]` contra `Generator[Session]` sigue siendo error.
- **El planificador reporta avances mientras corre (`executor.Request.Avance`):** cada adaptador traduce su stream (`avanceOpenCode`, `avanceClaude`) a líneas cortas —`$ comando`, `lee archivo`, `turno N · X tokens escritos`— y `esperarPlan` las muestra bajo el spinner con el tiempo transcurrido, o como líneas en `--plain`. Un plan largo no está colgado: el de closet fueron 19 minutos de un solo turno escribiendo 73k tokens a ~60/s.
- **Sin saldo en el proveedor es de la corrida, no de la tarea (`executor.SinSaldo`):** un 402 `Insufficient account funds` no se arregla reintentando ni escalando de modelo por la misma cuenta. En closet dejó 5 tareas detenidas tras 36 invocaciones y todas sus dependientes "bloqueadas". Ahora `correrUno` guarda la tarea como **pendiente** sin escalar, `correr` deja de lanzar las que siguen, `ejecutarOlas` integra las verdes de esa ola y corta, y `emitirResultados` resume en una línea. Al recargar, `up` retoma sin `--reintentar`. Prueba de punta a punta: `TestSinSaldoDejaTodoPendienteSinEscalar`.
- **Sin examinador, el archivo de prueba del `listo_cuando` entra al alcance (`loop.conPruebasPropias`):** el prompt le dice al agente "la suite la escribes TÚ" cuando `PatronesPrueba` está vacío, pero si ese archivo no estaba en `tocar_solo` la reversión lo borraba: tarea imposible por diseño. En closet (monorepo con `backend/` y `frontend/`, `DetectLanguage` da `""`) las tareas quemaron 7-15 intentos cada una y la escalera a Qwen Max hizo el 91% del gasto. Se agrega en `loop.Run`, el único punto por donde pasan todas, con la ruta tal cual y con `**/` delante porque puede ser relativa a un `--prefix` o a un `cd`. El extractor vive en `task.ArchivosDePrueba`.
- **devclean excluye dependencias y builds por su cuenta (`room.ExcluirArtefactos`):** el bucle indexa con `git add -A` y en un repo nuevo no hay `.gitignore`; en closet T-002 commiteó 1.3M líneas de `node_modules` que viajaron a cada cuarto, diff y revisor. La lista va a `info/exclude` del repo (compartido por los worktrees, no versionado).
- **Un manifiesto que no instala en el esqueleto es un problema para el arquitecto, no un aborto:** entra a la lista de `Problemas` y se corrige en la siguiente vuelta. Antes abortaba y, como la respuesta queda guardada, cada relanzamiento reusaba el mismo manifiesto roto.
- **Los extras de `pyproject.toml` son opcionales también al instalar:** las `dependencies` son obligatorias; cada extra se intenta aparte y si no instala se salta (closet: extra `jev` con un SDK que no está en PyPI).
- **Las dependencias se instalan por manifiesto hasta dos niveles (`room.InstalarDependencias`):** `package.json` con el gestor de su lockfile (`gestorNode`: pnpm, yarn, bun o npm), `go.mod`, `Cargo.toml`, `Gemfile`, `composer.json` y `pyproject.toml`/`requirements.txt` (a `.venv` con `--system-site-packages`, solo dependencias, no el proyecto). `room.Entorno` pone los `.venv/bin` delante del PATH; `loop.Run` lo agrega al agente y a `listo_cuando`.
- **No se escala contra una prueba que no corrió (`Outcome.NoEscalar`):** `loop.PruebaNoCorrio` (127, pytest 4/5, "no test files", "file or directory not found") y la reversión de lo único que tocó el agente marcan un rojo que un modelo caro no arregla. En closet la escalera a qwen-max pagó el 95% de la corrida contra una prueba que no existía.
- **Sin progreso = un intento (>1) que no deja nada dentro del alcance:** no se compara la salida de la prueba, que cambia por tiempos aunque nada cambie.
- **El tope de intentos es por tarea, no por llamada a `loop.Run`:** `TopeIntentos × limite_intentos` intentos pagados (con tokens) desde la última edición del contrato. Escalar, retomar y `--reintentar` suman; editar el contrato lo reinicia.
- **La prueba que ya existía cuando arrancó la tarea es el oráculo (`loop.pruebasFijas`):** si el archivo de prueba de `listo_cuando` está en el commit con que arrancó el cuarto (lo escribió el humano o un plan previo), se veda como ruta de prueba y no corre el examinador. Se mira ese commit y no el árbol: la prueba que el agente escribió en un intento anterior sigue siendo suya al escalar o retomar.
- **`write_overlap` no aplica entre tareas encadenadas por `depende_de`:** corren en fila y la segunda arranca sobre lo que dejó la primera. El esqueleto crea los stubs que después rellena cada tarea.
- **En la entrega conjunta, `bisectable` se conforma con el `listo_cuando` si la suite falla en la rama de la tarea (`Opciones.SuiteAlIntegrar`, `ship.bisectable`):** esa rama arrancó de una base vieja y no ve lo que se arregló después en la base (closet: un timeout corregido en `main` frenaba a T-048). La suite completa la corre `integradas` sobre todo junto y la base actual. El `ship` de una tarea suelta la sigue exigiendo.
- **`bisectable` exige el `listo_cuando` cuando la suite ya fallaba al empezar la tarea (`ship.suiteYaFallaba`):** con esqueleto, la suite sigue roja hasta rellenar todos los módulos. Se comprueba corriendo la suite sobre el commit base en el mismo cuarto y volviendo a la rama; la suite completa la exige `integradas`.
- **Cada tarea de relleno pasa también el `verificar` del esqueleto (`conVerificar`):** su `listo_cuando` queda `<verificar> && (<prueba>)`, y `verificar` entra además como `acceptance.command` sobre lo integrado. En closet vitest pasó las 29 tareas y la integración, y `npm run build` (tsc) tenía 7 errores de tipos: vitest no revisa tipos y nadie corría el build después del esqueleto.
- **El código comentado avisa, no frena (paso `ruido`):** los comentarios de contrato del esqueleto traen pasos en pseudocódigo (`g = deps.servicio.agregar(...);`) y la heurística los confunde con código. Prints de debug y archivos temporales siguen frenando.
- **La respuesta del arquitecto se guarda (`.devclean/runs/T-00N/esqueleto-respuesta.json`, con el hash del pedido):** si devclean falla después (instalar, parsear, git), al relanzar se reverifica sin volver a pagarlo. En closet fueron 39 minutos y $6.90. El `ARCHITECTURE.md` previo se lee de la rama base, no del cuarto, o el prompt cambiaría al retomar.
- **El arquitecto devuelve `pruebas`, la suite completa:** en un monorepo `DetectLanguage` no la encuentra y sin ella `bisectable` e `integradas` no tienen qué correr. Se guarda en `config.yml` si estaba vacía.
- **La rama de entrega instala dependencias antes de `integradas`:** es un worktree nuevo, sin `.venv` ni `node_modules`. Todo comando de `ship` corre con `room.Entorno` en el PATH.
- **Secretos: sin comillas solo cuenta en archivos de configuración, y en pruebas solo los patrones de proveedor:** `api_key=settings.api_key` es una variable y `api_key="sk-or-secret"` en una prueba es un fixture; ambos frenaban el esqueleto de closet.
- **Cada modelo se valida contra el CLI que lo corre y solo se reemplaza el inválido (`revisarModelos`):** antes todo `modelos:` se validaba contra el catálogo de `cli:` y un solo id desconocido reasignaba los tres pesos: en closet `opencode/…-free` con `cli: claude` convirtió la pesada de sonnet en opus y el arquitecto arrancó con opus.
- **El agente muere si devclean muere (`executor.morirConPadre`, Pdeathsig en Linux):** matar devclean dejaba el `claude -p` del arquitecto corriendo y gastando. Fuera de Linux sigue abierto (marcado `ponytail:`).
- **`modelos:` puede mezclar CLIs (`ejecutorPara`):** un id con proveedor (`opencode/mimo-v2.6-flash-free`) corre en opencode; `claude-*`, `opus`, `sonnet` y `haiku` en claude, sin importar `cli:`. Lo usan la tarea, su escalada, el examinador, el revisor y las hojas de la recursión (`recurse.Agent.EjecutorPara`). `gratis` reconoce `-free` (opencode) y `:free` (OpenRouter).
- **Solo `topePagados` (3, o `agentes_pagados:` en config) tareas con modelo de pago corren a la vez sin `--agentes`:** los modelos `…-free` no cuentan y van primero en la fila; el automático sube a 16 trabajadores. `--agentes N` explícito topa ambos a N. Todas las pagadas gastan la misma ventana de 5 h: más en paralelo solo la vacía antes.
- **La escalera nunca sube a `pesada` (`ModeloEscalado`):** liviana→media y ahí para. Opus solo corre en tareas con `peso: pesada` y en el planificador. Una tarea que el medio no resuelve suele tener el contrato mal hecho.
- **El revisor corre con `Effort: "low"`** y por defecto con el modelo `media`: corre en cada intento verde y, desde el esqueleto plano, es el único contrapeso a una prueba complaciente escrita por el mismo agente.
- **`gate.Run` devuelve 6 chequeos:** Búscalos por nombre, nunca por índice en el array.
- **El chequeo 0 siempre es `contrato válido`:** Llama a `Validate()`. Todo contrato nuevo debe incluir `version: 1` (`task.Version`).
- **Hay dos parsers y cada uno tiene su territorio:** el spec humano se parsea con `yaml.v3` en `internal/spec/yaml.go` (`spec.Parse`); el frontmatter de contratos (`internal/task`) y `config` siguen en `internal/kv`. No migres uno al otro sin consensuarlo, y no agregues una tercera librería de parseo. En `kv` sigue viva la trampa de las claves repetidas a distinta profundidad: se pisan.
- **`spec.ValidatePlan` distingue error de advertencia y solo el error aborta:** `Apply` junta los `Level=="error"` y falla con `plan inválido: ...`; las advertencias (`requirement_coverage`, `acceptance_coverage`, `integration_test`) únicamente se imprimen. Búscalos por `Code`, nunca por índice. La cobertura se estima por palabras de más de 4 letras: no la trates como prueba semántica.
- **El spec de requirements es incremental (`.devclean/intencion.json`):** `runApply` guarda lo que el humano declaró (`spec.Intencion`: feature, requirements, reglas, aceptación, restricciones) tras aplicar. Si el próximo `up` trae lo mismo, no llama al arquitecto y solo corre las pendientes. Si cambió, `deltaRequirements` le manda los requerimientos nuevos como obligatorios, los que siguen como "ya implementados" y los que ya no aparecen con ese texto como "quitados o reescritos": compara por texto exacto, así que una redacción nueva llega como quitado + nuevo, y el prompt pide borrar solo lo que ningún requerimiento vigente pida. Antes cada `up` pagaba al arquitecto otra vez con otro id (el caché de la respuesta no pegaba) y duplicaba tareas. Solo aplica al modo requirements sin `tasks` y no a una petición suelta (`up "<frase>"`), que no toca el archivo. Borrarlo fuerza replanear todo. Un repo planeado antes de v1.7.1 no lo tiene: se usa `feature.json` como plan previo, sin comparar la aceptación (trae la del esqueleto); sin ese respaldo, closet con 29 tareas `lista` se replaneó entero.
- **`ship --todas` salta lo que ya está en la base (`ship.Entregadas`):** una tarea `lista` cuyo trabajo ya se integró (commit con trailer `Tarea: T-00N`, o entrega registrada en `.devclean/entregas.jsonl` cuya punta ya es ancestro de la base) se da por entregada: se libera su cuarto y su rama, que es como `run` (`sembrarVerdesPrevias`) ya reconocía una tarea entregada. Sin esto, mantener un proyecto rompía la segunda entrega: `lista` no distingue "verde sin entregar" de "ya integrada", y las tareas viejas chocaban con las que las cambiaron después. El trailer sobrevive al rebase y al squash de GitHub; el registro cubre un `merge` local.
- **La aceptación del feature vive en `.devclean/feature.json`, no en los contratos:** `apply` la guarda (`SaveFeatureState`) y `ship --todas` la lee (`LoadFeatureState`). Si ese archivo no existe, el paso `aceptación` simplemente no corre y la entrega sigue: borrar `.devclean/` borra la compuerta global. Solo las aceptaciones **con `command`** llegan ahí como compuerta (`AcceptanceCommands`).
- **`constraints.no_tocar` se aplica en `Apply`, nunca en `Parse`:** `planearRequirements` agrega sus contratos **después** de parsear, así que aplicarlos al leer el YAML dejaba sin frontera justo al camino principal (requirements sin tasks). `Apply` es el único punto por donde pasan los dos orígenes de tareas. Si mueves esa lógica de vuelta al parser, el spec vuelve a mentir sobre su frontera; hay prueba que lo caza (`TestApplyPropagaConstraintsAlIRGenerado`).
- **El nivel funcional de overlap corre DESPUÉS de la oleada:** Si se corre antes, las ramas de las tareas están vacías y no detecta nada. Requiere `Resultado.Arbol` proveniente de `mergeTree`.
- **`standup.Analizar` requiere latidos EN CRUDO (`LeerLatidosCrudos`):** La diferencia temporal entre latido fresco y rancio separa una tarea atascada (`ATASCO`) de una que murió por kill (`MUERTA`).
- **El ledger de ventanas es global del usuario (`~/.devclean/ventanas.jsonl`):** No se resetea borrando la carpeta `.devclean/` del proyecto.
- **Convenciones de Mensajes de Error:** Frases en minúscula, sin punto final, sin disculpas, indicando qué ocurrió y qué hacer para solucionarlo (ej. `tarea rechazada · listo_cuando no ejecutable · edita T-001 y reintenta`).
