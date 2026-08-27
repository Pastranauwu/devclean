# PRD — devclean

**Versión:** 0.1 (borrador para implementación)
**Fecha:** 26 agosto 2026
**Tipo:** herramienta CLI, código abierto, local-first

---

## 1. En una frase

devclean es una herramienta de terminal que dirige a varios agentes de IA programando en paralelo sobre un mismo repositorio, y garantiza que lo único que llega al proyecto sea código limpio, probado y con historial legible.

**Analogía:** una gerencia de consultoría de software. El humano dice qué quiere; devclean define, reparte, supervisa, prueba y entrega. El humano recibe el resultado, no el desorden.

---

## 2. El problema

Los datos del mercado, no opiniones:

- El tiempo para abrir un pull request cayó **58%** con IA, pero esos PRs pasan **4.6 veces más tiempo** en revisión por falta de control automatizado (Opsera, 2026).
- El *churn* — líneas reescritas o revertidas en menos de dos semanas — subió de **3.31% (2021) a 6.87% (2024)** (GitClear, 211M líneas).
- El código "movido" (señal de refactorización) se desplomó de **15.88% a 3.10%**, mientras subieron el código añadido y el copiado.
- DORA 2024: calidad del código **+3.4%**, estabilidad de entrega **−7.2%**.

**Diagnóstico:** la IA mejora la línea suelta y empeora el sistema. Generar dejó de ser el cuello de botella; ahora el costo está en revisar, depurar y probar a ciegas.

**Causa raíz:** el agente entra a programar sin una definición ejecutable de "listo". Sin criterio de éxito, un agente no puede parar; solo puede seguir intentando. De ahí el ciclo infinito de prueba y error, el código muerto y las soluciones que no encajan con el resto del proyecto.

---

## 3. Qué NO es devclean

Anti-alcance explícito. Si el implementador duda, esto manda.

- **No** es un modelo ni un agente. Usa los que el usuario ya paga.
- **No** es un servicio en la nube. No hay backend, no hay cuenta, no hay servidor.
- **No** es una interfaz web ni un dashboard. Todo en terminal.
- **No** es un reemplazo de CI/CD. Actúa **antes** del PR.
- **No** es "AgentOps" (operar agentes en producción) ni "Agentic DevOps" (agentes manejando el pipeline). Esas categorías ya existen y son otra cosa.
- **No** gestiona despliegues, infraestructura ni monitoreo.

---

## 4. Usuario y escenario

**Usuario:** desarrollador individual o equipo pequeño que ya usa agentes de código y sufre PRs sucios, conflictos y depuración a ciegas.

**Requisito de entrada:** un repositorio git y al menos una API key de un proveedor de modelos.

**Escenario objetivo:** el usuario tiene 5 cosas por hacer, las suelta en lenguaje natural, se va a comer, y al volver tiene 3 PRs limpios listos para revisar y 2 tareas detenidas con una pregunta concreta.

---

## 5. Flujo principal

```
$ devclean init
  ✓ repositorio detectado
  ✓ comando de pruebas detectado: npm test
  ✓ configuración creada en .devclean/

$ devclean plan "necesito exportar clientes a CSV y arreglar el login que falla con tildes"
  Propongo 3 tareas:
  T-001  exportar clientes a CSV       listo cuando: npm test -- export
  T-002  login acepta tildes           listo cuando: npm test -- auth
  T-003  test de regresión de acentos  listo cuando: npm test -- i18n
  ¿Aprobar? [s/n/editar]

$ devclean run --agentes 3
  T-001  ██████░░░░  intento 1   glm-5.2
  T-002  ████████░░  intento 2   deepseek
  T-003  ██████████  verde → esclusa de salida

$ devclean board
  LISTO PARA ENTREGAR   T-003
  EN CURSO              T-001, T-002
  DETENIDO              —

$ devclean ship T-003
  ✓ rebase sobre main
  ✓ 47 guardados internos → 3 commits limpios
  ✓ sin código muerto, sin archivos de prueba, sin secretos
  ✓ cada commit compila y pasa pruebas
  ✓ PR #142 abierto
```

El humano solo ve: el plan, el tablero y el PR. Los intentos, logs y ramas internas quedan ocultos salvo que pida `--verbose` o `devclean logs T-001`.

---

## 6. Conceptos del sistema

### 6.1 Contrato de tarea

Un archivo por tarea en `.devclean/tasks/T-001.md`. Máximo 8 campos. Si crece, la tarea es demasiado grande y debe partirse.

```yaml
---
version: 1
id: T-001
titulo: exportar clientes a CSV
porque: soporte pierde 3h/semana copiando a mano
listo_cuando: npm test -- export.spec.ts     # OBLIGATORIO, ejecutable
tocar_solo: ["src/export/**"]
no_tocar: ["src/auth/**", "migrations/**"]
limite_intentos: 3
limite_lineas: 200
riesgos: archivos grandes pueden agotar memoria
---
Notas libres opcionales.
```

**`version` es obligatorio** (adenda A.1). El parser rechaza campos desconocidos, salvo que la versión del archivo sea mayor a la que soporta el binario: ahí los ignora y avisa `contrato versión 2, binario soporta 1 · actualiza devclean`. Así un contrato de v0.2 no muere en un binario viejo.

**Regla dura:** si `listo_cuando` no es un comando ejecutable que devuelve 0 o distinto de 0, la tarea se rechaza y no se asigna a ningún agente.

El contrato lo **redacta un modelo barato** a partir de la frase del usuario; el usuario solo aprueba o corrige. Nunca lo escribe a mano.

### 6.2 Cuarto (aislamiento)

Cada tarea activa recibe:
- Un `git worktree` propio en `.devclean/rooms/T-001/`, en rama `devclean/T-001`.
- Dependencias instaladas de forma independiente.
- Puertos asignados por devclean, nunca fijos.
- Variables de entorno propias (base de datos de prueba separada si aplica).

Se destruye al terminar.

### 6.2b Estado de tarea

El estado de una tarea (`pendiente`, `en_curso`, `lista`, `descartada`) vive en `.devclean/state/`, **fuera del contrato**. El contrato describe qué hay que lograr y no cambia mientras la tarea corre; el estado cambia todo el tiempo. Mezclarlos obligaría a reescribir el contrato en cada transición.

Es además la base del parte de datos (§6.7) y lo que decide qué tareas entran en el chequeo de cruce: solo se comparan las que están `en_curso`.

### 6.3 Esclusa de entrada

Antes de asignar una tarea, devclean valida:
1. `listo_cuando` existe y se ejecuta.
2. El comando falla **hoy** (si ya pasa, la tarea no tiene sentido).
3. `tocar_solo` no se cruza con el de otra tarea activa. Vacío significa "sin restricción" y solo se permite mientras haya una sola tarea en curso; con dos o más es obligatorio, porque sin alcance declarado no hay cruce que detectar (adenda A.4).
4. `tocar_solo` no incluye zonas prohibidas globales (lockfiles, migraciones, CI, CHANGELOG).
5. `tocar_solo` no apunta a rutas de prueba (adenda A.3). Los patrones viven en `patrones_prueba` de `config.yml`, que `init` siembra con `*_test.go`, `test/**`, `spec/**`, `*.spec.ts` y compañía. Un alcance amplio como `src/export/**` sí se acepta: contiene pruebas pero no las declara, y de las que se editen se encarga la reversión del bucle.

Falla cualquiera → tarea rechazada con motivo legible.

### 6.4 Bucle de trabajo del agente

```
mientras intentos < limite:
    el agente edita dentro de tocar_solo
    devclean guarda un punto de restauración interno
    devclean ejecuta listo_cuando
    verde → salir del bucle
    rojo  → devolver la salida del error al agente, intento++
al agotar intentos:
    detener, generar pregunta concreta para el humano, no seguir inventando
```

Puntos clave:
- Los guardados internos son commits `wip:` en la rama del cuarto. Basura intencional, nunca llegan al PR.
- Un archivo fuera de `tocar_solo` se revierte automáticamente y se avisa al agente.
- **La verificación la hace devclean, no el modelo.** El agente nunca decide si terminó.

### 6.5 Esclusa de salida

Pasos deterministas, en orden, en `devclean ship`:

1. `git fetch` + rebase sobre la rama base. Conflicto en archivo ajeno → abortar y reportar.
2. Aplanar todo el trabajo y reconstruir en 1–5 commits con formato Conventional Commits, con el trailer `Agent: <modelo>`.
3. Escaneo de basura: prints de debug, código comentado, archivos temporales, tests de exploración, reformateo de archivos no relacionados.
4. Escaneo de secretos en el diff y en todos los commits de la rama.
5. Verificación de presupuesto (`limite_lineas`, archivos tocados).
6. Verificación bisectable: cada commit compila y pasa pruebas por separado.
7. Generar handoff: qué cambió, **qué no se hizo**, riesgos, dependencias, cómo verificar.
8. Abrir PR y liberar el cuarto.

Falla cualquier paso → no se abre PR, se reporta la razón exacta.

### 6.6 Cola de integración

Paralelo para trabajar, fila india para integrar. Un merge a la vez; cada merge invalida y revalida a los que siguen. Evita que dos PRs verdes por separado rompan la rama principal juntos.

### 6.7 Parte de datos duros

**No implementado. v0.2** (adenda §6.7). Sustituye cualquier idea de reuniones o reportes entre agentes.

**Principio:** un agente nunca reporta su propio avance. Todo se mide del artefacto.

Evidencia que lo justifica, va también en el README:
- Los agentes se conforman públicamente entre 64% y 94% de las veces pese a oponerse en privado.
- Los modelos débiles corrigen solo el 3.6% de sus sesgos de postura en un debate; abandonan juicios correctos por alinearse con la mayoría.
- El debate multi-agente es una martingala: no aporta ganancia esperada sobre el voto independiente.

**Comando:** `devclean standup`, disparado por evento, nunca por reloj. Eventos: un agente toca un símbolo compartido, termina, agota intentos, o supera el presupuesto de diff.

Contenido, todo derivado de `attempts.jsonl` (§6.4, formato en `docs/attempts-jsonl.md`):

```
PARTE 14:32 · 3 tareas en curso

⚠ COLISIÓN   T-001 cambia la firma de Format(); T-002 la invoca en 3 lugares
⚠ ATASCO     T-003 sin cambio en el conteo de fallos desde hace 11 min
⚠ DESVÍO     T-001 agrega caché en memoria; el contrato no lo pide
             evidencia: src/export/writer.go:88
✓            T-002 dentro de contrato
```

Las dos primeras alertas son deterministas y cuestan cero tokens. La tercera es la única que usa un modelo.

**Reglas del juicio con modelo**, solo para DESVÍO:
1. Modelo distinto al que escribió el código.
2. Juicio emitido **antes** de ver el de cualquier otro agente.
3. Toda objeción cita `archivo:línea`. Sin cita, se descarta automáticamente.
4. Protocolo Disagree-or-Commit: o critica explícitamente, o respalda con evidencia nueva.
5. Cero rondas de debate. Si hay desacuerdo, escala al humano.

Bitácora inmutable en `.devclean/standups/`.

**Prohibido:** rondas de debate, un agente coordinador que lea los reportes de los demás, y pedir a un agente que califique su avance en porcentaje.

### 6.8 Examinador ciego y suite oculta

**No implementado. v0.2** (adenda §6.8).

**Orden obligatorio:**
```
contrato aprobado → examinador escribe la suite → se sella (hash) → arranca el implementador
```

**Qué ve el examinador:** el contrato y la frontera pública (firmas, endpoints, CLI, esquema de datos).
**Qué no ve:** el cuerpo de las funciones.

Regla que simplifica todo: las pruebas corren contra el borde del sistema (CLI, HTTP, base de datos), no contra funciones internas. Ahí el contrato **es** la interfaz.

Esto es lo que hace obligatoria la regla de §6.3.5: si el implementador puede editar las pruebas, el examen no vale nada.

**Suite oculta:**
- El implementador ve el 70% de los casos. Ese es su bucle.
- El 30% restante corre **una sola vez**, en la esclusa de salida.
- Si falla el oculto, la tarea vuelve al **examinador** para una suite nueva, no al implementador para otro intento. La suite oculta se quema al usarse.

**Métrica estrella:**
```
brecha = % suite visible aprobado − % suite oculta aprobado
```
Cercana a cero: resolvió el problema. Grande: ajustó al examen. Se detecta sin leer una línea de código.

**Control del examinador:** mutation testing sobre los archivos del diff (`go-mutesting`). Sin esto, un 100% verde sobre una suite vacía no significa nada.

**Límite honesto, va en el README:** esto no aplica a interfaz gráfica, comportamiento visual ni criterios difusos. devclean sirve para lo que tiene oráculo. Prometer más quema el proyecto.

### 6.9 Detección de solapamiento en tres niveles

**No implementado. Nivel textual y semántico en v0.1, funcional en v0.2** (adenda §6.9).

| Nivel | Método | Costo | Cuándo |
|---|---|---|---|
| Textual | `git merge-tree` entre pares de ramas activas | milisegundos | cada evento |
| Semántico | símbolos exportados modificados en común (de `attempts.jsonl`) | nulo | cada evento |
| Funcional | merge en seco + correr las suites de ambas ramas sobre el resultado | alto | solo si textual o semántico marcaron sospecha |

El nivel funcional es el que atrapa el fallo clásico: dos ramas verdes por separado que rompen juntas.

El chequeo 3 de la esclusa de entrada (§6.3) es la versión barata y estática de esto: compara `tocar_solo` declarados antes de que exista una sola línea de código.

### 6.10 Integración y acoplamiento

**No implementado. v0.2** (adenda §6.10).

Ataca directamente el hallazgo de GitClear: el código "movido", señal de refactorización, se desplomó de 15.88% a 3.10% mientras subían el añadido y el copiado. Los agentes agregan y duplican; no integran.

- **Duplicación entre ramas:** comparar estructura de funciones nuevas entre ramas activas. Determinista y barato.
- **Contratos entre tareas:** si T-001 produce una interfaz que T-002 consume, la firma se congela en ambos contratos y se verifica contra los dos diffs.
- **Reglas de dependencia:** dirección permitida entre módulos declarada en config (`api → dominio → datos`); se verifica el grafo de imports del diff.

### 6.11 Constitución del proyecto

**No implementado. v0.2** (adenda §6.11).

Archivo `.devclean/constitution.md`, inyectado en el contexto de **todos** los agentes: convenciones de estilo, capas, manejo de errores, patrones prohibidos.

Motivo: agentes en paralelo toman decisiones implícitas distintas sobre estilo, casos borde y arquitectura. Dos tareas pueden pasar sus pruebas y aun así elegir abstracciones incompatibles. Ninguna esclusa de las anteriores detecta eso.

Se genera con el agente en modo entrevista la primera vez y se versiona en el repo.

---

## 7. Comandos

| Comando | Qué hace |
|---|---|
| `devclean init` | detecta repo, comando de pruebas, crea `.devclean/` |
| `devclean plan "<texto>"` | convierte lenguaje natural en contratos de tarea, pide aprobación |
| `devclean task add\|edit\|rm` | manejo manual de tareas |
| `devclean run [--agentes N]` | ejecuta tareas en paralelo |
| `devclean board` | tablero de estado |
| `devclean ship <id>` | esclusa de salida y PR |
| `devclean logs <id>` | detalle interno de una tarea |
| `devclean report` | métricas del proyecto |
| `devclean doctor` | verifica configuración, keys, permisos, git |

---

## 8. Motor de agentes

### 8.1 Configuración

`.devclean/config.yml`:

```yaml
base: main
pruebas: npm test
proveedores:
  planificador: { modelo: claude-sonnet, key_env: ANTHROPIC_API_KEY }
  ejecutor:     { modelo: glm-5.2,       key_env: OPENCODE_API_KEY }
  revisor:      { modelo: deepseek-v4-flash, key_env: DEEPSEEK_API_KEY }
zonas_prohibidas: ["package-lock.json", "migrations/**", ".github/**"]
```

Las keys se leen de variables de entorno o de `~/.devclean/keys` con permisos 600. **Nunca se guardan en el repo.**

### 8.2 Roles y por qué

| Rol | Trabajo | Modelo sugerido |
|---|---|---|
| Planificador | convierte petición en contratos, parte tareas grandes | el mejor disponible; un error aquí cuesta horas |
| Ejecutor | escribe el código dentro del cuarto | barato, N en paralelo |
| Revisor | lee el diff contra el handoff, marca incoherencias | barato, tarea acotada |

### 8.3 Cómo se ejecuta el agente — DECIDIDO: opción (A)

devclean **no implementa su propio bucle de herramientas**. Envuelve CLIs de agente ya existentes (`claude`, `opencode`) como subprocesos y hereda gratis la edición de archivos, el manejo de contexto y el uso de herramientas.

Contrato del adaptador en Go:

```go
type Executor interface {
    Name() string
    Available() error                       // verifica binario y versión
    Run(ctx context.Context, req Request) (Result, error)
}

type Request struct {
    RoomPath     string    // cwd del agente, su cuarto
    Prompt       string    // contrato + resultado del intento anterior
    AllowedGlobs []string  // tocar_solo
    Model        string
    Timeout      time.Duration
}

type Result struct {
    FilesChanged []string
    Tokens       Usage
    Stdout       string
    ExitCode     int
}
```

Implementaciones v0.1: `executor/opencode`, `executor/claudecode`.
v0.2 añade `executor/api` (llamada directa al proveedor) sin tocar nada más. Esa es toda la razón de que exista la interfaz.

`devclean doctor` verifica que al menos un ejecutor esté instalado y con versión mínima.

---

## 9. Métricas

Se calculan solas y se muestran con `devclean report`. Son el argumento de venta del proyecto.

| Métrica | Definición | Meta |
|---|---|---|
| **Intentos hasta verde** | vueltas de prueba y error por tarea | ≤ 2 |
| **Ruido** | % de líneas del PR que no sirven al objetivo | < 5% |
| **Roce** | conflictos de merge por cada 10 tareas paralelas | < 1 |
| **Fricción** | minutos entre PR abierto y aprobado | bajar |
| **Rechazo en entrada** | % de tareas rechazadas por mala definición | visible, no se oculta |

Cada tarea guarda además su costo en tokens y dinero.

---

## 10. Requisitos no funcionales

- **La sobrecarga de devclean no debe superar el 10% del tiempo total de la tarea** (adenda A.5). Cuando todo va bien, la salida es una línea por tarea. Cualquier verificación que no quepa en ese presupuesto se corre en segundo plano o se mueve a v0.2. Motivo: el hallazgo de METR fue sobrecarga de validación; una herramienta que agrega fricción visible reproduce el problema que dice resolver. Este requisito debe poder matar features buenas.
- **Local-first.** Cero servidores, cero puertos escuchando, cero telemetría. Sin excepciones.
- **Go 1.22+, un solo binario estático**, sin runtime que instalar. Compilación cruzada para linux/darwin/windows en amd64 y arm64.
- **Instalación de una línea:** script `curl`, Homebrew tap y `go install`. Binarios firmados en GitHub Releases vía GoReleaser.
- Funciona sin conexión salvo las llamadas al proveedor de modelos.
- Arranque en frío en máquina limpia < 2 minutos.
- Todo el estado en archivos de texto dentro de `.devclean/`, legible y versionable.

## 11. Seguridad

Referencia negativa: OpenClaw llegó a 42.000 instancias expuestas en internet y más de 1.100 paquetes maliciosos en su ecosistema. Este proyecto ejecuta código escrito por IA, así que:

- Nada escucha en la red. Nunca.
- El agente solo escribe dentro de su cuarto y dentro de `tocar_solo`.
- Comandos peligrosos (`rm -rf`, `push --force`, `curl | sh`) bloqueados por lista.
- Las keys nunca entran al contexto del modelo ni a logs.
- Escaneo de secretos obligatorio antes de cualquier PR.
- El usuario puede ver el diff completo antes de que se abra el PR (`--dry-run`).

---

## 12. Alcance

### v0.1 — MVP (3 semanas)
- `init`, `plan`, `task`, `run`, `board`, `ship`
- Contrato de tarea + esclusa de entrada
- Cuartos aislados con worktree
- Bucle de intentos con límite
- Esclusa de salida completa
- Las 5 métricas
- Un proveedor de modelos funcionando de punta a punta

### v0.2
- Cola de integración automática
- Examinador ciego y suite oculta, métrica de brecha (§6.8)
- Mutation score como control del examinador (§6.8)
- Detección de solapamiento funcional (§6.9)
- Duplicación entre ramas, contratos entre tareas, reglas de dependencia (§6.10)
- Constitución del proyecto (§6.11)
- Segundo y tercer proveedor
- Modo API directa

### Fuera de alcance (por ahora)
- Debate entre agentes y auto-reportes de avance (§6.7: nunca, no "por ahora")
- Verificación de UX, de rendimiento o de cualquier criterio sin oráculo (§6.8)
- Interfaz web o dashboard
- Multiusuario / equipos
- Integración con Jira, Linear o similares
- Despliegue e infraestructura
- Cualquier cosa que necesite un servidor

---

## 13. Criterios de aceptación

El producto está listo cuando, en un repositorio real:

1. Tres tareas corren en paralelo, terminan y ninguna tocó archivos de otra.
2. Una tarea con `listo_cuando` inválido es rechazada antes de gastar un solo token.
3. Una tarea que agota intentos se detiene y produce una pregunta concreta, no un arreglo inventado.
4. Un PR generado tiene menos de 5 commits, cero rastros de `wip:`, cero código muerto, y cada commit pasa las pruebas por separado.
5. Un revisor humano aprueba el PR sin pedir limpieza.
6. `devclean report` muestra las 5 métricas con datos reales.
7. **devclean se construye a sí mismo con devclean** a partir de la semana 2, y el historial del repo lo demuestra.

---

## 14. Riesgos

| Riesgo | Mitigación |
|---|---|
| Los modelos baratos no logran verde en 3 intentos | límite configurable; detenerse es un resultado válido, no un fallo |
| Reescribir historial de git y perder trabajo | rama de respaldo automática antes de cualquier operación destructiva |
| Categoría concurrida (9+ orquestadores OSS ya existen) | el diferenciador son las dos esclusas y las métricas, no el paralelismo |
| Dependencia de CLIs externos que cambian | adaptador aislado, versión mínima verificada en `doctor` |
| Percepción de "otra capa de burocracia" | contrato de 8 líneas, redactado por IA, aprobado en 10 segundos |

---

## 15. Decisiones tomadas

Cerradas. No reabrir durante v0.1.

| Decisión | Elección |
|---|---|
| Lenguaje | **Go 1.22+**, binario estático único |
| Ejecución de agentes | **Opción (A)**: envolver `claude` y `opencode` como subprocesos, detrás de la interfaz `Executor` |
| Licencia | **MIT** |
| Plataforma de PR | **Solo GitHub** en v0.1, vía `gh` CLI o API REST |

Dependencias externas permitidas: `git` (obligatorio), `gh` (opcional, hay fallback por API), al menos un CLI de agente.

---

## 16. Interfaz de terminal

La interfaz no es decoración: es el producto. Es lo que aparece en el GIF del README y lo que decide si alguien pone una estrella.

### 16.1 Stack

| Pieza | Librería |
|---|---|
| Framework TUI | `charmbracelet/bubbletea` |
| Estilos y layout | `charmbracelet/lipgloss` |
| Componentes (spinner, lista, viewport) | `charmbracelet/bubbles` |
| Formularios de aprobación | `charmbracelet/huh` |
| Render de markdown (handoff, planes) | `charmbracelet/glamour` |

### 16.2 Dirección visual

El tema es el **cuarto limpio industrial**: sala presurizada, esclusas, luces de estado. No es una terminal de hacker ni un dashboard corporativo. Se lee como el panel de control de una sala blanca: casi todo apagado, y la información viva marcada con una sola luz.

**Paleta** (truecolor, con degradado automático a 256 y a monocromo):

```
tinta      #E6E6E1   texto principal
apagado    #6B6F72   texto secundario, bordes
presion    #4FB3A2   verde-aqua: verde, listo, aprobado    ← acento único
alerta     #D96C4A   rojo-arcilla: detenido, rechazado
espera     #C9A227   ámbar: en curso, intento en marcha
fondo      transparente (respeta el tema del usuario)
```

Una sola luz de acento (`presion`). Todo lo demás vive en gris. Si un día todo está verde, la pantalla es casi monocroma y eso significa "todo bien" sin que nadie lo explique.

**Reglas de composición:**
- Bordes de una sola línea, esquinas rectas. Nada redondeado.
- Sin emojis. Los estados son glifos: `·` en espera, `◐` trabajando, `✓` verde, `✗` rojo, `⏸` detenido.
- Un solo nivel de indentación. Si necesita dos, la pantalla está haciendo demasiado.
- Ancho fijo de 80 columnas para que el GIF se vea igual en todos lados; adaptable hacia arriba.
- Sin barras de progreso falsas. Si no se sabe cuánto falta, se muestra un contador de intentos, no una barra inventada.

### 16.3 Elemento firma: la esclusa

Es lo único llamativo del producto y solo aparece en `devclean ship`. Los ocho pasos de la esclusa de salida se dibujan como una compuerta que se presuriza de izquierda a derecha, un paso a la vez, con el nombre del paso debajo:

```
  ╭──────────────────────────────────────────────────────────────╮
  │  ESCLUSA DE SALIDA · T-003                                   │
  ╰──────────────────────────────────────────────────────────────╯

     ✓    ✓    ✓    ✓    ◐    ·    ·    ·
    base  hist  ruido secr  presu bisec hand  pr

    47 guardados internos → 3 commits
    verificando presupuesto: 118 líneas de 200
```

Si un paso falla, la compuerta se detiene ahí, se pinta en `alerta` y debajo aparece exactamente qué encontró y el comando para verlo. No se abre PR. Esa imagen — la compuerta que se frena — es la que explica el producto entero sin texto.

### 16.4 Pantallas

**`plan`** — lista de tareas propuestas, cada una con su `listo_cuando` visible. Formulario `huh` de tres opciones: aprobar, editar, descartar. Editar abre `$EDITOR`.

**`run`** — una línea por tarea activa:

```
  T-001  exportar clientes a CSV      ◐ intento 2/3   glm-5.2      1m14s
  T-002  login acepta tildes          ✓ verde         deepseek     2m03s
  T-003  test de regresión acentos    ⏸ detenido      glm-5.2      —
         └ agotó 3 intentos · falla: unicode NFD en el comparador
```

Los logs de los agentes **no se muestran**. Solo la línea de estado. `devclean logs T-001` los abre aparte.

**`board`** — tres columnas: en curso, listo para entregar, detenido. Sin scroll si caben; sin paginación falsa.

**`report`** — las cinco métricas, valor actual y flecha de tendencia. Nada más.

### 16.5 Modos y accesibilidad

- `--plain`: sin colores, sin animación, una línea por evento. Obligatorio para CI y para logs.
- Detecta `NO_COLOR` y salida no interactiva (pipe) y cae a `--plain` solo.
- `--json`: salida estructurada de todos los comandos, para que otros la consuman.
- Ningún estado se comunica solo por color: siempre hay glifo y palabra.

### 16.6 Voz de la interfaz

- Frases en minúscula, sin punto final, sin signos de exclamación.
- Los errores dicen qué pasó y qué hacer, nunca piden disculpas: `tarea rechazada · listo_cuando no ejecutable · edita T-001 y reintenta`.
- El nombre de una acción no cambia: si el comando es `ship`, el estado dice "entregando" y el resultado dice "entregado".
- Nada de "generando magia", "pensando…" ni antropomorfismos. El agente trabaja, no piensa.
- Las pantallas vacías invitan a actuar: `sin tareas · empieza con devclean plan "lo que necesitas"`.

---

## 17. Plan de implementación

Orden obligatorio. Cada fase termina en algo usable; no se empieza la siguiente sin cerrar la anterior.

### Fase 1 — Núcleo de tareas (días 1-3)
1. Esqueleto del binario, `cobra` para comandos, `--plain` y `--json` desde el inicio.
2. `devclean init`: detectar repo, rama base, comando de pruebas, crear `.devclean/`.
3. Parser y validador del contrato de tarea (§6.1).
4. `task add/edit/rm/list` sobre archivos.
5. Esclusa de entrada (§6.3), incluido el chequeo de que `listo_cuando` falla hoy.
**Cierra cuando:** una tarea mal definida se rechaza con motivo legible, sin gastar tokens.

### Fase 2 — Cuartos y ejecución (días 4-8)
6. Gestor de worktrees: crear, preparar entorno, asignar puertos, destruir.
7. Interfaz `Executor` + adaptador `opencode`.
8. Bucle de intentos con límite, guardados `wip:`, reversión de archivos fuera de `tocar_solo`.
9. `devclean run` con N tareas en paralelo.
**Cierra cuando:** tres tareas corren a la vez sin tocarse y una que agota intentos se detiene con una pregunta concreta.

### Fase 3 — Esclusa de salida (días 9-14)
10. Rebase y detección de conflicto ajeno.
11. Reconstrucción de commits en Conventional Commits con trailer `Agent:`.
12. Escáneres: ruido, secretos, presupuesto.
13. Verificación bisectable.
14. Generación de handoff y apertura de PR en GitHub.
**Cierra cuando:** un PR generado pasa revisión humana sin pedir limpieza.

### Fase 4 — Interfaz y métricas (días 15-19)
15. TUI con bubbletea según §16, incluida la esclusa animada.
16. Las cinco métricas y `devclean report`.
17. `devclean doctor`.
**Cierra cuando:** el GIF de 20 segundos se puede grabar.

### Fase 5 — Lanzamiento (días 20-21)
18. GoReleaser, script de instalación, tap de Homebrew.
19. README con manifiesto, GIF y tabla de métricas del propio repo.
20. Publicar.

### Reglas para el equipo de agentes que construya esto
- Un adaptador de ejecutor, un escáner y un comando son unidades separadas: nunca en el mismo PR.
- Todo lo que la esclusa verifica se implementa como función pura con pruebas propias. Cero llamadas a modelos dentro de la verificación.
- A partir de la fase 3, devclean se usa a sí mismo para construirse.

---

## Apéndice — Manifiesto (para el README)

1. El historial del agente no es el historial del proyecto.
2. Nadie toca lo que no reclamó.
3. Verificar es trabajo de código, no del modelo.
4. Lo que no se hizo, se declara.
5. Paralelo para trabajar, en fila para entregar.
6. Si no puedes escribir el comando que dice "ya está", la tarea no existe.
