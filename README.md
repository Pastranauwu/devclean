# devclean

**Declara qué software debe existir. devclean decide cómo construirlo y no lo
entrega si no pasa sus compuertas.**

La interfaz principal es `devclean.spec.yml`: una descripción versionable del
feature, sus requirements, reglas, restricciones y criterios de aceptación.
devclean inspecciona el repositorio, genera contratos internos de tarea, valida
su grafo, ejecuta el trabajo en aislamiento, integra los cambios y entrega un
pull request verificado.

```text
Requirements as Code
        ↓
Planning / Task Graph
        ↓
Internal Task Contracts
        ↓
Parallel Agents
        ↓
Task Verification
        ↓
Integration / Feature Acceptance
        ↓
PR
```

Los agentes son parte de la ejecución, no la interfaz del producto. La
coordinación ocurre mediante contratos, dependencias, interfaces, Git y pruebas.

## Quick start

En la raíz de tu proyecto, crea `devclean.spec.yml`:

```yaml
feature: recuperación de contraseña

requirements:
  functional:
    - solicitar recuperación por email
    - cambiar contraseña usando el token
  security:
    - token de un solo uso

rules:
  - no modificar sesiones

acceptance:
  - token expirado debe rechazarse
  - criterion: integración completa
    command: go test ./... -run TestPasswordRecovery

constraints:
  no_tocar:
    - internal/session/**

ship: true
```

Después ejecuta:

```bash
devclean up
```

Por detrás, devclean:

1. lee `requirements`, `rules` y `acceptance`;
2. inspecciona el lenguaje, las pruebas y las convenciones del repositorio;
3. genera contratos de tarea en `.devclean/tasks/`;
4. construye y valida el task graph;
5. ejecuta las tareas en `git worktree` aislados;
6. verifica cada tarea con comandos ejecutables y sus compuertas;
7. integra un commit limpio por tarea, respetando dependencias;
8. ejecuta la suite del proyecto sobre el conjunto integrado;
9. ejecuta los comandos globales de feature acceptance;
10. crea el PR —o una entrega local si no existe remoto—.

Normalmente no necesitas definir tareas, archivos, agentes, interfaces ni
comandos por tarea. El planificador genera esos detalles a partir del estado
deseado y del repositorio. Si `ship: true`, `up` llega hasta la entrega; sin él,
termina después de ejecutar las tareas.

![demo](docs/demo.gif)

## Dos niveles de especificación

### Human Spec — `devclean.spec.yml`

Es la interfaz recomendada. Describe principalmente **qué debe conseguir el
sistema**:

- `feature`: nombre del cambio y título predeterminado de la entrega;
- `requirements`: lista simple o categorías YAML anidadas;
- `rules` —también acepta `reglas`—: reglas comunes del feature;
- `acceptance`: criterios textuales y comandos de aceptación global;
- `constraints.no_tocar`: rutas globalmente prohibidas para todos los contratos
  del feature;
- `ship`: entrega al terminar;
- opciones avanzadas como `agentes`, `agente` y `limites`.

El parser admite YAML anidado para requirements y acceptance. Las restricciones
globales implementadas actualmente se limitan a `constraints.no_tocar`, y sus
globs se añaden al `no_tocar` de cada contrato al aplicar el spec: da igual si
la task venía escrita en el YAML o si la generó el planificador desde
requirements.

### Internal Task Contracts — `.devclean/tasks/*.md`

Son el IR generado por el planificador: expresan cómo ejecutar y verificar cada
unidad de trabajo. Incluyen:

- `titulo` y `porque`;
- `listo_cuando`;
- `tocar_solo` y `no_tocar`;
- `depende_de`;
- `expone` y `usa`;
- `riesgos`, `peso` y `agente`;
- límites de intentos y líneas.

Siguen siendo editables y ejecutables como interfaz avanzada. También puedes
escribir tasks completos en el spec o administrar contratos a mano; el formato
anterior no desapareció.

Como analogía práctica:

```text
devclean.spec.yml        → source humano
.devclean/tasks/*.md     → task IR generado
código + commits         → resultado de ejecución
```

No es un compilador formal: el planificador usa un modelo y sus contratos pueden
ser incorrectos. Las verificaciones deterministas deciden qué puede avanzar.

## Progressive disclosure

### Nivel 1 — normal

Solo declara el resultado:

```yaml
feature: notas markdown

requirements:
  - crear notas
  - editar notas
  - buscar notas
```

```bash
devclean up
```

Si el spec contiene requirements pero no `tasks`, devclean genera todo el task
IR automáticamente. En este nivel `up` ejecuta las tareas, pero no crea el PR a
menos que añadas `ship: true` o pases `--ship`.

### Nivel 2 — control

Añade reglas, aceptación, restricciones y delivery cuando importen:

```yaml
feature: notas markdown

requirements:
  functional:
    - crear y editar notas
    - buscar por contenido
  security:
    - impedir rutas fuera del directorio de notas

rules:
  - almacenamiento local
  - mantener compatibilidad con la API actual

acceptance:
  - una búsqueda encuentra una nota recién editada
  - criterion: recorrido completo de notas
    command: go test ./... -run TestNotesWorkflow

constraints:
  no_tocar:
    - internal/auth/**

ship: true
```

### Nivel 3 — avanzado

Decide tareas concretas cuando necesites controlar el IR:

```yaml
feature: wake on lan con alexa
agentes: 3
ship: true

tasks:
  - titulo: enviar magic packet por udp
    listo_cuando: go test ./internal/wol/...
    tocar_solo: ["internal/wol/**"]
    expone: ["wol.Send(mac, addr string) error"]
  - guardar la mac en json
  - titulo: endpoint de la skill alexa
    porque: expone el feature a la automatización
    listo_cuando: go test ./internal/alexa/...
    tocar_solo: ["internal/alexa/**"]
    no_tocar: ["internal/auth/**"]
    depende_de: ["T-001"]
    usa: ["wol.Send(mac, addr string) error"]
```

Una línea como `- enviar magic packet por udp` es una tarea rápida: el
planificador completa el contrato. En una tarea detallada, conserva los campos
que escribiste y completa los huecos necesarios. Los specs con tasks completos,
las tareas rápidas y los contratos manuales continúan soportados, pero el camino
recomendado para empezar es Requirements as Code.

## Generación automática y aceptación

Cuando hay `requirements` y no hay `tasks`, el planificador recibe:

- el feature y todos los requirements;
- las reglas obligatorias;
- los criterios de acceptance;
- el lenguaje, comando de pruebas y constitución detectados;
- las zonas prohibidas y los alcances ya ocupados.

Su salida son contratos con comandos verificables, alcances, dependencias e
interfaces. Después devclean asigna IDs, sanea alcances imposibles y valida el
grafo antes de guardar o ejecutar las tareas.

Hay dos formas de acceptance con garantías distintas:

```yaml
acceptance:
  - token expirado debe rechazarse
  - criterion: recuperación completa
    command: go test ./... -run TestPasswordRecovery
```

- **Acceptance textual:** entra en el razonamiento del planificador y debe quedar
  cubierta por el plan. Por sí sola no es una compuerta determinista.
- **Acceptance con `command` o `comando`:** se guarda en
  `.devclean/feature.json` y se ejecuta sobre la rama donde ya se integraron
  todas las tareas. Un código de salida distinto de cero frena la entrega.

Si necesitas garantizar feature acceptance de forma determinista, declara un
comando ejecutable. El texto explica la intención; el comando actúa como oráculo.

**Si no declaras ninguno, devclean lo deriva.** Cuando el plan tiene varias
tareas encadenadas y el spec no trae `command`, devclean agrega una última tarea
que prueba la costura: depende de todas, consume todo lo que el plan promete, no
implementa nada y escribe pruebas en `test/integracion/`. Su comando queda
también como aceptación del feature, así que corre dos veces: en su propio cuarto
y sobre el conjunto integrado. En la salida aparece como
`· T-00N prueba la costura entre tareas`.

### Task verde no implica feature correcto

Cada contrato conserva su propio `listo_cuando`, pero una tarea puede pasar
aislada y romperse al juntarse con las demás. Por eso la entrega conjunta añade
verificación sobre el resultado integrado:

```text
Task verification + per-task quality/security gates
        ↓
Integration
        ↓
Project test suite
        ↓
Feature acceptance commands
        ↓
PR
```

Primero cada tarea pasa su esclusa individual. Luego devclean integra un commit
por tarea, corre el comando `pruebas` del proyecto sobre el conjunto y finalmente
ejecuta cada `acceptance.command`. El PR solo se crea si todo queda verde.

El hueco que esto tapa: cada `listo_cuando` prueba lo que su propio contrato
pide, así que una tarea puede soportar un caso que su consumidora nunca le pidió
y nadie lo ejercita. Cinco tareas verdes construyendo una calculadora dejaban
`-2^2` devolviendo `unknown binary operator: ^`, porque el operador estaba en el
contrato del lexer y del parser y no en el del evaluador. El nivel funcional del
solapamiento tampoco lo ve: corre esos mismos comandos. Solo lo atrapa una prueba
que entre por la frontera final, y por eso ahora siempre existe una.

## Análisis estático del task graph

Antes de ejecutar agentes, devclean valida el IR siempre que ya dispone del plan.
Los errores estructurales bloquean la aplicación completa, antes de escribir las
tareas o gastar trabajo de implementación:

- dependencias circulares;
- dependencias que no existen;
- interfaces en `usa` que nadie declara en `expone`;
- interfaces incompatibles con una firma expuesta del mismo nombre;
- la misma interfaz expuesta por más de una tarea;
- zonas de `tocar_solo` que pueden solaparse.

También emite advertencias —no bloqueos— cuando:

- no reconoce cobertura de un requirement en los contratos;
- no reconoce cobertura de un criterio textual de acceptance;
- hay varias tareas y no se declaró aceptación global.

La cobertura se estima a partir del texto de títulos, motivos, notas y comandos.
Sirve para señalar huecos evidentes, no para demostrar corrección semántica.

## Qué hace, en concreto

1. Lee el estado deseado desde requirements.
2. Inspecciona el repositorio y genera un plan compatible con su stack.
3. Convierte el plan en contratos de tarea verificables.
4. Analiza dependencias, interfaces y alcances antes de ejecutar.
5. Congela en `expone`/`usa` las fronteras relevantes entre tareas.
6. Ejecuta cada tarea en un worktree aislado.
7. Revierte cambios fuera de `tocar_solo`, incluso si el agente los commiteó.
8. Verifica mediante comandos y códigos de salida, no mediante opinión del
   modelo. Un comando que sale con cero sin ejecutar pruebas no cuenta como verde.
9. Integra las tareas en orden de dependencia y vuelve a probar el conjunto.
10. Ejecuta feature acceptance y entrega el PR solo si pasan las compuertas.

En Go y Python, un examinador ciego puede redactar pruebas desde la frontera
pública antes de la implementación y sellar una parte para la entrega. La
esclusa individual revisa rebase, historial, ruido, secretos, presupuesto,
interfaces, reglas de imports, bisectabilidad, suite oculta y handoff.

## Instalación

```sh
curl -fsSL https://github.com/Pastranauwu/devclean/releases/latest/download/install.sh | sh
```

Deja el binario en `~/.local/bin`. Alternativa:

```sh
go install github.com/Pastranauwu/devclean/cmd/devclean@latest
```

devclean no trae ningún modelo. Dirige un CLI que ya tienes y pagas:
[Claude Code](https://docs.anthropic.com/claude-code) (`claude`) u
[OpenCode](https://opencode.ai) (`opencode`). Necesitas uno instalado y
autenticado, además de `git`. Para PRs en GitHub necesitas `gh`; sin remoto
`origin`, devclean crea la rama `devclean/_entrega` y una descripción local en
`.devclean/pr/`.

`devclean doctor` comprueba las dependencias del entorno.

## Comandos

Flujo principal:

| Comando | Qué hace |
|---|---|
| `devclean up ["<petición>"]` | Sin petición, encuentra y aplica el spec del repo; con texto, genera un plan desde esa petición. Después ejecuta las tareas. `--ship` entrega. |
| `devclean apply [-f archivo]` | Lee un spec, completa/genera contratos y los guarda. `--dry-run` valida sin gastar tokens ni escribir; `--run` ejecuta después. |
| `devclean board` | Muestra tareas listas, en curso, detenidas y pendientes. |
| `devclean run [--reintentar]` | Ejecuta pendientes; `--reintentar` revive detenidas reusando su trabajo. |
| `devclean ship T-001` | Pasa la esclusa y entrega una tarea. |
| `devclean ship --todas` | Verifica e integra todas las tareas listas, corre las pruebas globales y crea un solo PR. |
| `devclean logs T-001` | Muestra intentos y diagnósticos de una tarea. |

Planificación, diagnóstico y operación:

| Comando | Qué hace |
|---|---|
| `devclean plan "<petición>" [--aprobar]` | Genera una propuesta de contratos. `--export-spec archivo.yml` la exporta. |
| `devclean check T-001` | Corre la esclusa de entrada sobre un contrato. |
| `devclean ps` | Muestra tareas y worktrees activos, estilo compose. |
| `devclean standup` | Resume avance, bloqueos y colisiones desde artefactos, sin conversaciones entre agentes. |
| `devclean report` | Muestra las métricas del proyecto y su tendencia. |
| `devclean usage` | Muestra consumo por ventanas de 5h, semanal y mensual contra el presupuesto. |
| `devclean doctor` | Verifica Git, configuración, credenciales y ejecutores. |
| `devclean init` | Crea `.devclean/` explícitamente; `up` también prepara lo necesario. |
| `devclean constitution` | Genera `.devclean/constitution.md`, que se incorpora a planificación y prompts. |
| `devclean skills sync` | Descarga las skills configuradas para los agentes. |
| `devclean task add\|edit\|rm\|list` | Administra contratos manualmente. |
| `devclean task seal T-001` | Sella una suite oculta escrita a mano. |

Todos aceptan `--plain` para salida lineal y `--json` para salida estructurada.
En una terminal, cuando corresponde, usan interfaz interactiva.

Variantes útiles de `up`:

```bash
devclean up --ship                 # spec → tareas → ejecución → entrega
devclean up --agentes 4            # fuerza el paralelismo de ejecución
devclean up --revisar              # crea PR y publica una revisión del diff
devclean up --integrar             # revisa e integra si no se piden cambios
devclean up --fondo                # deja la corrida en segundo plano
devclean up -f specs/auth.yml      # usa un spec concreto
```

`--revisar` y `--integrar` implican la fase de entrega. Los flags de CLI ganan
sobre las opciones equivalentes del spec.

### Trabajar en segundo plano

```text
$ devclean up --ship --fondo
corriendo en segundo plano · pid 41287
registro · .devclean/corridas/2026-09-10T18-02-29.log
parar · kill 41287 · retomar después · devclean run --reintentar
```

Las preguntas ocurren antes de desprender el proceso. Si una corrida muere,
`board` detecta el latido obsoleto y permite retomarla con `run --reintentar`.

## Formato avanzado de contratos

Cada tarea vive en `.devclean/tasks/T-001.md`. `id`, `titulo` y
`listo_cuando` son obligatorios al validar el contrato:

```yaml
---
version: 1
id: T-001
titulo: exportar clientes a CSV
porque: soporte pierde tiempo copiando datos a mano
listo_cuando: npm test -- export.spec.ts
tocar_solo: ["src/export/**"]
no_tocar: ["src/auth/**"]
depende_de: []
expone: ["export.ToCSV(rows []Row) []byte"]
usa: []
riesgos: conservar escapes y codificación UTF-8
peso: liviana
agente: backend
limite_intentos: 3
limite_lineas: 200
---
Enfoque sugerido para la implementación.
```

`listo_cuando` debe ser ejecutable, fallar antes del cambio y pasar cuando la
tarea quede completa. El cuerpo después del frontmatter son notas para quien
implementa; no sustituye al criterio ejecutable.

Un spec avanzado también puede incluir contratos completos:

```yaml
version: 1
feature: exportación de clientes
agentes: 2
ship: true
agente: backend
limites:
  intentos: 3
  lineas: 300
rules:
  - conservar compatibilidad con la API pública

tasks:
  - titulo: serializador CSV
    porque: produce el formato descargable
    listo_cuando: go test ./internal/export/... -run TestCSV
    tocar_solo: ["internal/export/**"]
    no_tocar: ["internal/auth/**"]
    expone: ["export.ToCSV(rows []Row) []byte"]
    riesgos: escapar comas, comillas y saltos de línea
    peso: media
```

Si ninguna tarea trae ID, referencias como `T-001` en `depende_de` se resuelven
por posición dentro de ese spec y luego se traducen a los IDs reales.

## Configuración interna

`up` crea `.devclean/config.yml` cuando hace falta. Es una interfaz avanzada:

```yaml
base: main
pruebas: go test ./...
cli: claude
modelos:
  liviana: haiku
  media: sonnet
  pesada: opus
estrategia: equilibrada
timeout_agente: 1200
timeout_pruebas: 300
presupuesto_tokens: 0
presupuesto:
  claude: { 5h: 40000, semanal: 120000 }
zonas_prohibidas: ["go.sum", "migrations/**", ".github/**"]
patrones_prueba: ["*_test.go", "test/**", "*.spec.ts"]
agentes:
  specialist: { provider: claude, model: sonnet, skills: ["python", "ml"] }
reglas_import: ["api → dominio → datos"]
recursion_max: 0
```

Aquí viven selección de modelos, retries, presupuestos, timeouts, reglas de
imports y arquetipos. El flujo normal no obliga a administrarlos. Los arquetipos
incorporados son `ejecutor`, `backend`, `frontend`, `architect`, `tester` y
`refactor`.

## Cuando algo falla

- **Plan inválido.** `apply` enumera el ciclo, dependencia, interfaz o alcance
  conflictivo y no escribe ningún contrato.
- **Advertencia de cobertura.** El análisis no encontró términos del requirement
  o acceptance en el IR. No bloquea: revisa el plan antes de ejecutarlo.
- **Tarea rechazada en entrada.** Normalmente `listo_cuando` ya pasa o el alcance
  viola una zona protegida. `devclean check T-001` muestra la causa.
- **Tarea detenida.** Agotó intentos. Usa `logs` y después
  `run --reintentar` para continuar desde su worktree.
- **Verde sin pruebas.** Un comando puede salir con cero sin ejecutar ninguna
  prueba. devclean detecta varios casos y detiene la tarea en lugar de aceptarla.
- **Integración roja.** Las tareas pasan aisladas pero el comando `pruebas` falla
  sobre el conjunto. No se crea el PR.
- **Feature acceptance roja.** El `acceptance.command` falló sobre la rama
  integrada. La salida indica el comando y el error.
- **Esclusa de salida frenada.** El primer gate fallido detiene la publicación y
  conserva la razón exacta.
- **Presupuesto excedido.** El mensaje indica líneas y límite. Las pruebas se
  contabilizan aparte del código de solución.

## Seguridad

- El agente trabaja en un worktree propio y los cambios fuera de `tocar_solo`
  se revierten antes de verificar.
- Las credenciales no se incorporan deliberadamente a prompts o logs.
- Cada tarea pasa escaneo de secretos y ruido antes de entrar a la entrega.
- `devclean ship --dry-run` recorre las compuertas sin publicar el PR.
- La única sonda HTTP directa de devclean es la opción de
  `devclean usage --sonda`; los CLIs de agentes y `gh` sí usan red por su cuenta.

## Coste y modelos

El coste depende del plan, el tamaño del contexto, el modelo y los reintentos.
Una tarea detenida puede escalar de modelo y reutilizar el trabajo anterior.
`presupuesto_tokens` y los presupuestos por ventana permiten cortar antes de
agotar cuota; `devclean usage` muestra el estado registrado.

Los detalles de modelo y agente quedan en `.devclean/config.yml` o en el IR. No
son necesarios en el human spec salvo que quieras imponerlos.

## Límite honesto

devclean eleva el nivel de evidencia; no demuestra que el software sea correcto.

- **La calidad de salida depende de los requirements.** Una omisión o ambigüedad
  puede producir un plan incompleto y código verde que no resuelve la intención.
- **Acceptance textual no es una compuerta.** Ayuda al planificador y genera
  advertencias de cobertura, pero solo un `command` ejecutable se corre como
  aceptación global determinista.
- **Un comando incorrecto también puede mentir.** Si comprueba poco, pasa sin
  demostrar el feature; si depende del entorno, puede fallar código correcto.
- **Los contratos son generados por modelos.** Pueden dividir mal el trabajo,
  inventar rutas, elegir pruebas débiles o congelar interfaces equivocadas.
- **El análisis estático es estructural.** Detecta ciclos, referencias rotas,
  firmas inconsistentes y algunos solapamientos; no demuestra compatibilidad
  semántica ni cobertura real del comportamiento.
- **La prueba de costura la escribe un modelo.** Garantiza que exista un examen
  end-to-end y que corra sobre el conjunto integrado, no que sea exhaustivo. Se
  deriva solo en Go, Node y Python, y solo cuando el spec no declara ya un
  `acceptance.command`: si lo declaras, la costura es tuya.
- **`listo_cuando` debe fallar antes del cambio.** Una suite general que ya está
  verde no prueba que una tarea nueva exista.
- **Los hidden tests solo se generan automáticamente para Go y Python**, y
  requieren una frontera importable. `package main`, Node y Rust quedan fuera
  del examinador automático actual.
- **La suite oculta depende de la respuesta del examinador.** Si no produce el
  bloque oculto, ese gate se omite; la suite visible puede seguir existiendo.
- **Las pruebas ocultas están separadas, no blindadas.** Viven fuera del worktree
  y llevan hash contra corrupción accidental, pero no forman un secreto
  criptográfico frente a un atacante con acceso al repositorio.
- **Los worktrees no son un sandbox de seguridad.** Aíslan ramas y directorios;
  no aíslan red, procesos, credenciales del sistema ni permisos del usuario.
- **Los agentes y cuotas son externos.** Un CLI sin login, sin cuota o caído
  detiene la corrida; devclean diagnostica el problema, no lo elimina.
- **La interfaz YAML madura aplica al human spec.** Los contratos y otros
  archivos internos históricos todavía usan el parser reducido de devclean.

## Licencia

MIT.
