# devclean

Dirige agentes de IA programando en paralelo sobre un mismo repositorio y
garantiza que lo único que llega al proyecto sea **código limpio, probado y
con historial legible**.

El humano dice qué quiere; devclean define, reparte, supervisa, prueba y
entrega. El humano recibe el resultado, no el desorden.

```
$ devclean plan "exportar clientes a CSV y arreglar el login con tildes"
  Propongo 3 tareas:
  T-001  exportar clientes a CSV       listo cuando: npm test -- export
  T-002  login acepta tildes           listo cuando: npm test -- auth
  T-003  test de regresión de acentos  listo cuando: npm test -- i18n
  ¿crear estas tareas? [s/n]

$ devclean run --agentes 3
  T-001  exportar clientes a CSV      ✓ verde en 2 intentos
  T-002  login acepta tildes          ✓ verde en 1 intentos
  T-003  test de regresión acentos    ⏸ detenida · agotó 3 intentos · falla: …

$ devclean ship T-001
  ✓ base · ✓ historial · ✓ ruido · ✓ secretos · ✓ presupuesto · ✓ interfaces · ✓ dependencias · ✓ bisectable · ✓ suite_oculta · ✓ handoff · ✓ pr
  entregado · https://github.com/tu/repo/pull/142
```

![demo](docs/demo.gif)

---

## El problema

Los datos, no opiniones:

- El tiempo para abrir un PR cayó **58%** con IA, pero esos PRs pasan **4.6×
  más tiempo** en revisión por falta de control automatizado (Opsera, 2026).
- El *churn* — líneas reescritas o revertidas en menos de dos semanas — subió
  de **3.31% (2021) a 6.87% (2024)** (GitClear, 211M líneas).
- El código "movido", señal de refactorización, se desplomó de **15.88% a
  3.10%**, mientras subieron el código añadido y el copiado.
- DORA 2024: calidad +3.4%, estabilidad de entrega **−7.2%**.

La IA mejora la línea suelta y empeora el sistema. Generar dejó de ser el
cuello de botella; el costo está en revisar, depurar y probar a ciegas.

**Causa raíz:** el agente entra a programar sin una definición ejecutable de
"listo". Sin criterio de éxito, un agente no puede parar; solo puede seguir
intentando. De ahí el ciclo infinito de prueba y error, el código muerto y
las soluciones que no encajan.

## Cómo lo resuelve

Dos esclusas, un contrato y un examinador que no ve tu código. Antes de gastar
un token, la tarea se define con un comando ejecutable que dice "ya está"
(`listo_cuando`). El agente trabaja en un cuarto aislado, y **la verificación
la hace código, nunca el modelo**.

1. **Esclusa de entrada** — `listo_cuando` debe existir, fallar hoy y no
   pisar el alcance de otra tarea. Una tarea mal definida se rechaza con un
   motivo legible, antes de gastar un solo token. Si el modelo metió una ruta
   vedada (`go.sum`, `*_test.go`…) en `tocar_solo`, se recorta con aviso antes
   de escribir el contrato y no hay callejón sin salida.
2. **Bucle de intentos** — el agente edita dentro de su alcance; devclean
   revierte lo que se salió, ejecuta `listo_cuando` y decide. Verde → entregado;
   rojo → se devuelve el error al agente. Agotados los intentos → se detiene
   con una pregunta concreta, no con un arreglo inventado. Antes del primer
   intento, un **examinador ciego** (§6.8) genera la suite de pruebas contra la
   frontera pública (`expone`) — 70% visible para el implementador, 30% sellada
   con hash — sin ver el cuerpo de las funciones. Si la tarea no declara
   `expone`, no hay examen: no hay frontera que probar. El examinador
   soporta **go** (validado con `go/parser`) y **python** (validado con
   `ast.parse` del `python3` del sistema); rust y node todavía no, y en
   esos stacks el examen se salta en vez de emitir pruebas que no corren.
   También podés saltarte el modelo y sellar tus propias pruebas con
   `devclean task seal` (abajo): la esclusa de salida las trata igual, no
   pregunta quién las escribió.
3. **Esclusa de salida** — hasta once pasos deterministas en `ship`: rebase,
   historial aplanado, sin ruido, sin secretos, dentro de presupuesto,
   interfaces entregadas, dependencias dentro de las reglas, bisectable,
   suite oculta superada, handoff, y recién entonces el PR. Nueve pasos son
   fijos; `dependencias` corre solo si hay `reglas_import` en la config y
   `suite_oculta` solo si el examinador selló pruebas. Cualquiera que falle
   frena la compuerta con la razón exacta.

Arriba del bucle, dos radares sin tokens:
- **Solapamiento activo** (§6.9): `git merge-tree` + símbolos exportados en
  común entre cada par de ramas de la misma oleada. Alerta antes de juntar.
- **Parte de datos** (§6.7): `devclean standup` deriva COLISIÓN (símbolos
  compartidos) y ATASCO (>10 min sin progreso en conteo de tests) de
  `attempts.jsonl`. No hay reuniones entre agentes; todo se mide del artefacto.

Una **constitución** (`.devclean/constitution.md`, §6.11) inyectada en cada
prompt evita que dos tareas paralelas elijan arquitecturas incompatibles.
Se genera una vez con `devclean constitution` y se versiona en git.

## Quién hace qué: plan, tareas y agentes

Tres cosas distintas que se confunden fácil:

- **`devclean plan`** no programa nada. Le pide a un modelo (el rol
  *planificador*, ver `proveedores` en la config) que convierta tu frase en
  contratos de tarea (`.devclean/tasks/T-00N.md`). Vos apruebas o editas.
  Costo: una llamada barata al modelo, cero código escrito todavía.
- **Las tareas** son los archivos que resultan del plan (o que escribís a
  mano con `devclean task add`). Son el contrato: qué hay que lograr y con
  qué comando se sabe que ya está (`listo_cuando`). No corren nada por sí
  solas.
- **`devclean run`** es lo que de verdad programa. Toma las tareas
  pendientes y por cada una lanza un **agente** — un CLI de código que ya
  tenés instalado (`claude` o `opencode`, rol *ejecutor*) — dentro de un
  worktree aislado (`.devclean/rooms/T-00N/`). `--agentes N` controla
  cuántos corren en paralelo. devclean no trae su propio modelo: es un
  director que reparte el trabajo a los agentes que vos ya pagás y verifica
  el resultado con código, nunca con el modelo.

En una frase: `plan` decide **qué** hacer, `run` decide **quién** (qué
agente, en qué cuarto) lo hace y verifica que quedó listo.

## Manifiesto

1. El historial del agente no es el historial del proyecto.
2. Nadie toca lo que no reclamó.
3. Verificar es trabajo de código, no del modelo.
4. Lo que no se hizo, se declara.
5. Paralelo para trabajar, en fila para entregar.
6. Si no puedes escribir el comando que dice "ya está", la tarea no existe.

---

## Requisitos

- **git** (obligatorio).
- Un **CLI de agente**: `opencode` o `claude` (Claude Code).
- **`gh`** (opcional, solo para abrir PRs con `ship`).
- Una **API key** del proveedor en el entorno (`OPENCODE_API_KEY`,
  `ANTHROPIC_API_KEY`, etc.).
- Un **comando de pruebas** ejecutable en el proyecto (`npm test`, `go test ./...`, …).

`devclean doctor` verifica todo lo anterior.

## Instalación

```sh
curl -fsSL https://github.com/Pastranauwu/devclean/releases/latest/download/install.sh | sh
```

Descarga el binario de la release, lo instala en `~/.local/bin` (o
`$DEVCLEAN_INSTALL_DIR`) y ya está: `devclean` queda disponible en la
terminal. Si `~/.local/bin` no está en tu `PATH`, el instalador te avisa —
agrégalo (`export PATH="$HOME/.local/bin:$PATH"` en tu `.bashrc`/`.zshrc`).

Alternativas:

```sh
# con go install
go install github.com/Pastranauwu/devclean/cmd/devclean@latest

# desde el código fuente
git clone https://github.com/Pastranauwu/devclean
cd devclean && go build -o devclean ./cmd/devclean
sudo mv devclean /usr/local/bin/
```

Binario estático único (Go), sin runtime que instalar. Compila para
linux/darwin/windows en amd64 y arm64.

**El binario por sí solo no ejecuta nada.** devclean dirige agentes que ya
tienes instalados (`claude` o `opencode`); no trae ningún modelo dentro.
Corre `devclean doctor` justo después de instalar: te dice exactamente qué
falta (git, un CLI de agente, una API key) antes de que intentes usarlo.

---

## Primeros pasos

El camino corto: una petición entra, un pull request sale.

```sh
# dentro de un repo git con al menos un commit
devclean init
devclean up "exportar clientes a CSV y arreglar el login con tildes" --agentes 3 --ship
```

`up` planea, ejecuta cada tarea en su propio cuarto aislado y, con `--ship`,
entrega todo en **un solo PR** con un commit por tarea, en orden de dependencia.

Si quieres revisar el plan antes de que corra nada, quita `--ship` o usa
`devclean plan`: en terminal te muestra las tareas propuestas con casillas
(`espacio` marca, `a` todas, `n` ninguna, `enter` confirma) y solo crea las que
dejes marcadas. Descartar una tarea arrastra a las que dependían de ella —
crearlas sueltas dejaría un plan que nunca puede correr — y te dice cuáles se
cayeron con ella.

El camino largo, si quieres revisar entre paso y paso:

```sh
# 1. dentro de un repo git con al menos un commit
devclean init

# 1b. opcional pero recomendado: fija las convenciones del proyecto
devclean constitution

# 2. convierte una petición en tareas (el planificador usa un modelo)
devclean plan "exportar clientes a CSV y arreglar el login con tildes"

# 3. ejecuta en paralelo
devclean run --agentes 3

# 4. revisa y entrega
devclean board          # tablero
devclean standup        # parte de datos: colisiones y atascos
devclean ship --todas   # un PR con todas las tareas listas
devclean ship T-001     # o una sola tarea en su propio PR
```

### Cuando algo falla

```sh
devclean logs T-001     # intentos, tokens y el error exacto del agente
devclean doctor         # ¿existe el modelo configurado? ¿hay key? ¿hay ejecutor?
devclean standup        # atascos y colisiones, sin gastar tokens
devclean run --reintentar   # revive las tareas detenidas sin perder su trabajo
```

O desde el tablero, sin recordar comandos:

```sh
devclean board
```

| tecla | qué hace |
|---|---|
| `j` / `k` | mueve el cursor |
| `s` | entrega la tarea (`ship --dry-run`) · solo sobre LISTO |
| `r` | revive la tarea reusando su cuarto y su trabajo parcial · solo sobre DETENIDO |
| `d` | detalle: intentos, tokens, el error del agente y la ruta de su volcado |
| `q` | sale |

El tablero se refresca solo y, bajo cada tarea en curso, muestra qué está
haciendo ahora mismo: `intento 2/3 · agente · opencode/glm-5.3 · 3m12s`.
Si una fase lleva más de diez minutos sin moverse, la marca en rojo como
**ATASCO** — avisa, no la mata; tú decides si cortas.

### Saber qué pasa mientras corre

El bucle escribe el estado vivo en `.devclean/runs/<id>/latido.json` en cada
cambio de fase (`examen`, `agente`, `verificando`), con intento, modelo y
tokens acumulados. `attempts.jsonl` solo se escribe cuando un intento
*termina*, y un intento puede durar veinte minutos: sin el latido, una tarea
colgada era indistinguible de una que trabaja.

Como el estado está en disco, `devclean board` o `devclean standup` desde otra
terminal ven una corrida en curso, aunque la hayas lanzado con `--plain` en un
script o en CI.

Cada invocación del agente deja su volcado completo (prompt, stdout y stderr del
CLI) en `.devclean/runs/<id>/intento-N.log`. Si el agente ni siquiera llegó al
modelo — modelo inexistente, key ausente, rate limit — el bucle lo detecta, corta
sin quemar los intentos restantes y te dice qué pasó.

`init` detecta la rama base y el comando de pruebas; si se equivoca, corrígelo
con `devclean init --pruebas "mi comando"`, `--pruebas-plantilla go|node|python`
o editando `.devclean/config.yml`. Al crear una tarea, `task add` sugiere
ejemplos de `listo_cuando` para el stack detectado (go, node/jest, python/pytest);
si no hay stack conocido, deja el campo vacío con la regla de oro: un comando
que **hoy falla**. En un repo vacío (`greenfield`) `plan` te pregunta stack y
requisitos antes de generar.

## Adoptarlo en un proyecto real

devclean no exige cambiar tu stack: solo crea una carpeta `.devclean/` y
trabaja en worktrees aislados. No toca tu rama principal hasta `ship`.

1. **Prepara el terreno.** Asegúrate de que el repo compila y que tienes un
   comando de pruebas. `devclean doctor` revisa git, configuración, ejecutores
   y keys.

2. **Inicializa.** `devclean init` detecta la rama base y el comando de
   pruebas. Si tu comando no se detecta, pásalo con `--pruebas`.

3. **Fija la constitución.** `devclean constitution` genera
   `.devclean/constitution.md` (capas, convenciones, patrones prohibidos).
   Versiona el archivo. Todos los agentes la reciben en cada prompt.

4. **Delimita lo que nadie debe tocar.** En `.devclean/config.yml`, revisa
   `zonas_prohibidas` (lockfiles, migraciones, CI, changelog) y
   `reglas_import` (ej. `api → dominio → datos`). Por defecto ya están los
   sospechosos habituales.

5. **Define el trabajo.** Con `devclean plan` (modelo) o a mano con
   `devclean task add` + `devclean task edit`. La regla de oro: `listo_cuando`
   debe ser un comando que **hoy falla** y que el agente hará pasar. En
   `greenfield`, `plan` también limpia automáticamente cualquier ruta vedada
   que el modelo haya metido en `tocar_solo`.

   Ejemplo de creación manual:
   ```sh
   devclean task add "exportar clientes a CSV"
   devclean task edit T-001  # completa listo_cuando, tocar_solo, etc.
   ```
   **Nota:** el título va como argumento posicional entre comillas, NO como
   flag (`--titulo` o `--title` truenaría con error). Cobra muestra ejemplos
   en `devclean task add --help`.

6. **Declara las interfaces entre tareas.** Las tareas de una misma oleada
   corren aisladas y **no pueden leerse el código entre sí**. Si una produce
   algo que otra consume, congela la firma en los dos contratos: `expone` en
   la que la produce, `usa` en la que la consume, con el mismo texto. El
   planificador los llena solo; devclean rechaza un `usa` que nadie expone y
   verifica al entregar que el `expone` esté de verdad en el diff.

7. **Acota cada tarea.** `tocar_solo` declara qué rutas puede tocar el
   agente. Si corres varias tareas a la vez, es obligatorio: sin alcance
   declarado no hay cruce que detectar.

8. **Ejecuta y revisa.** `devclean run --agentes N`, luego `devclean board`,
   `devclean standup`, `devclean logs T-001` y `devclean report`. Si ves
   `⚠ SOLAPAMIENTO` durante `run`, dos ramas tocan el mismo símbolo o archivo.

9. **Entrega.** `devclean ship T-001 --dry-run` para ver la esclusa sin
   publicar; `devclean ship T-001` para abrir el PR (requiere `gh`). La suite
   oculta del examinador, si existe, se verifica acá y se quema al usarse.

### El contrato de tarea

Un archivo por tarea en `.devclean/tasks/T-001.md`, máximo 8 campos:

```yaml
---
version: 1
id: T-001
titulo: exportar clientes a CSV
porque: soporte pierde 3h/semana copiando a mano
listo_cuando: npm test -- export.spec.ts     # OBLIGATORIO, ejecutable
tocar_solo: ["src/export/**"]
no_tocar: ["src/auth/**", "migrations/**"]
depende_de: ["T-000"]                        # opcional: ids que deben estar verdes antes
expone: ["wol.Send(mac, addr string) error"] # opcional: firmas que otras tareas consumen
usa: ["config.Cargar(p string) error"]       # opcional: firmas de otras, copiadas igual
peso: liviana                                # opcional: liviana | media | pesada
agente: architect                            # opcional: agente asignado (declarado en config.yml)
limite_intentos: 3
limite_lineas: 200
riesgos: archivos grandes pueden agotar memoria
---
```

Notas libres opcionales debajo del segundo `---`.

### Recursividad: tareas que se reparten solas

Una tarea puede ser demasiado grande para un solo intento de agente. Marcarla
`recursivo: true` no cambia lo que exige el contrato — sigue necesitando un
`listo_cuando` real — pero cambia cómo se resuelve el intento:

```yaml
recursivo: true
limite_subtareas: 5   # tope de subtareas que puede proponer la descomposición
```

Con `recursion_max` en `config.yml` (0 = desactivado, el default), el
intento de esa tarea no lo escribe un solo agente: el rol *planificador*
(modelo caro) la reparte en subtareas reales, cada una con su propio
`listo_cuando`, acotadas al `tocar_solo` que ya tenía la tarea padre —
nunca pueden inventarse alcance nuevo. Cada subtarea corre en un **cuarto
anidado dentro del cuarto padre**: un cuarto ya es un worktree completo del
repo, así que adentro de él, otro `worktree add` es, en la práctica, git
propio para esa subtarea — misma rama, mismo historial, sin nada nuevo que
mantener. Verde → se integra a la rama del cuarto padre; si una subtarea
también es `recursivo` y todavía hay profundidad disponible, recursa de
nuevo. Solo la tarea raíz llega a `ship` y abre PR — las subtareas son
plomería interna, invisible afuera.

```yaml
recursion_max: 2   # en config.yml; 0 = sin recursión (default)
```

**Estado actual:** el bucle, la integración y el árbol funcionan
(`internal/recurse`). Cada subtarea resuelta (verde o roja, con su motivo)
queda en `.devclean/runs/<id>/arbol.json`, que sobrevive a que el cuarto se
destruya. `devclean board` (TUI y `--plain`/`--json`) y `devclean standup`
muestran ese árbol indentado bajo la tarea raíz.

---

## Requerimientos como Código (Declarativo estilo Docker Compose)

Así como `docker-compose.yml` declara servicios y contenedores, o `git` versiona ramas y cambios, **devclean** permite declarar tus épicas, requerimientos y agentes en un archivo declarativo (`devclean.spec.yml`):

### Analogías del Flujo

| Concepto | Docker Compose | Git | Devclean |
|---|---|---|---|
| **Definición** | `docker-compose.yml` | `.git/config` | `devclean.spec.yml` (Requerimientos como Código) |
| **Sincronización** | `docker compose build` | `git add` | `devclean apply [-f spec.yml]` |
| **Orquestación** | `docker compose up` | `git checkout -b` (worktrees) | `devclean up` / `devclean run` |
| **Estado en vivo** | `docker compose ps` | `git status` | `devclean ps` / `devclean board` |
| **Entrega / Release** | `docker compose push` | `git merge / push` | `devclean ship` |

### Ejemplo de `devclean.spec.yml`

```yaml
version: 1
feature: "Autenticación de usuarios y JWT"
agente: backend  # agente por defecto para las tareas

# Límites globales para todas las tareas de esta feature (opcional)
limites:
  intentos: 5    # máximo de intentos del bucle TDD (por defecto, 3)
  lineas: 500    # límite de líneas modificadas por tarea (por defecto, 200)

reglas:
  - "no usar sesiones en memoria, tokens stateless"
  - "validar formato de correo antes de consultar bd"

tasks:
  - id: T-001  # opcional: si se omite, devclean asigna correlativo
    titulo: "modelo de usuario y hash de contraseñas"
    listo_cuando: "go test ./internal/auth/ -run TestPasswordHash"
    tocar_solo: ["internal/auth/**"]
    agente: backend
    peso: liviana

  - id: T-002
    titulo: "endpoint de login con generación de JWT"
    listo_cuando: "go test ./internal/auth/ -run TestLogin"
    tocar_solo: ["internal/auth/**", "internal/api/**"]
    depende_de: ["T-001"]
    expone: ["POST /api/login -> 200 {token}"]
    agente: backend

  - id: T-003
    titulo: "formulario de login en cliente web"
    listo_cuando: "npm test -- LoginForm.test.tsx"
    tocar_solo: ["frontend/src/**"]
    depende_de: ["T-002"]
    usa: ["POST /api/login -> 200 {token}"]
    agente: frontend
```

### ¿Cómo se generan y ejecutan las pruebas?

No necesitas inventar rutas de prueba manuales para cada tarea:

1. **Examinador Ciego Automático (IA)**:
   - Cuando una tarea declara contratos públicos en `expone:` (ej. `"POST /api/login"` o `"wol.Send(mac) error"`), devclean invoca al **examinador ciego** (§6.8) antes de empezar.
   - El examinador genera automáticamente las pruebas unitarias contra la interfaz en rutas estándar dentro del cuarto aislado (`.devclean/rooms/T-xxx/`).
   - El 70% de la suite es **visible** para el agente como criterio de aceptación; el 30% restante se **sella con hash en una suite oculta** que solo se evalúa en la esclusa de salida (`devclean ship`), evitando que el agente haga trampa (*test cheating*).
2. **Pruebas Escritas a Mano (`devclean task seal`)**:
   - Si prefieres escribir tus propios tests o usar una suite predefinida sin gastar tokens de examinador, puedes sellarla con:
     ```sh
     devclean task seal T-001 --visible pruebas/visible_test.go --oculta pruebas/oculta_test.go
     ```
   - La esclusa de salida verificará la suite sellada exactamente igual, sin distinguir si la escribió un humano o la IA.
3. **Criterio Determinista (`listo_cuando`)**:
   - Cada tarea incluye su comando real de verificación (ej. `go test ./internal/auth/...`), garantizando que la compuerta de éxito la decida código determinista y nunca la opinión del modelo.

### Comandos Declarativos

- **`devclean apply`**: Lee `devclean.spec.yml`, valida los contratos y genera o sincroniza `.devclean/tasks/`.
  - `devclean apply --dry-run` para validar sin tocar el disco.
  - `devclean apply -f specs/modulo.yml` para aplicar un archivo específico.
  - `devclean apply --run` para aplicar y ejecutar inmediatamente.
- **`devclean up`**: Atajo estilo compose que aplica la especificación declarativa (si existe) y corre las tareas pendientes en paralelo en sus cuartos aislados.
- **`devclean ps`**: Resumen del estado de cada tarea, agente asignado, cuarto y puerto activo.
- **`devclean plan "..." --export-spec devclean.spec.yml`**: Le pide al planificador IA que redacte la propuesta y la guarde como archivo declarativo para que la puedas editar antes de aplicarla.

---

### Arquetipos de Agentes Predefinidos (Zero-Config)

devclean incluye un **catálogo de agentes estándar listos para usar out-of-the-box**. No necesitas configurar nada para usar roles comunes en tus tareas (`devclean.spec.yml` o `--agente`):

| Agente | Provider | Habilidades (Skills) Inyectadas |
|---|---|---|
| **`backend`** | `cli` (`claude` / `opencode`) | `["backend", "api", "database", "sql", "performance"]` |
| **`frontend`** | `cli` (`claude` / `opencode`) | `["frontend", "ui", "ux", "components", "css", "state"]` |
| **`architect`** | `cli` (`claude` / `opencode`) | `["arquitectura", "diseno", "contratos", "clean-code"]` |
| **`tester`** | `cli` (`claude` / `opencode`) | `["testing", "cobertura", "edge-cases", "examinador"]` |
| **`refactor`** | `cli` (`claude` / `opencode`) | `["refactoring", "simplificacion", "deuda-tecnica"]` |
| **`ejecutor`** | `cli` (`claude` / `opencode`) | `["implementacion", "tdd", "refactor"]` |

Ningún arquetipo trae modelo propio: el modelo sale del `peso` de la tarea
contra `modelos:` en `config.yml`, que `devclean init` rellena con el catálogo
real del CLI. Un arquetipo con un id fijo escrito a mano es un id que tarde o
temprano deja de existir, y una corrida que muere sin gastar un token.

`Skills` es solo la etiqueta que ve el prompt. Además, cada arquetipo trae
`SkillPackages`: nombres de **skills reales** (`SKILL.md`, formato
skills.sh/Claude Code) cuyo contenido completo se inyecta en el prompt del
agente, no solo el nombre. Todo arquetipo recibe la base — `implement` y
`clean-code` — y además: `backend` → `create-a-backend`,
`frontend` → `frontend-design`, `tester` → `test-driven-development`.

La base se paga entera en **cada intento de cada tarea**: va como texto al
principio del prompt. Por eso es corta a propósito. Si agregás paquetes en
`config.yml`, contá lo que cuestan: ocho paquetes son unos 59 KB (~15k tokens)
por invocación, antes de que el agente lea una sola línea de tu código.

Traelas una vez con:

```sh
devclean skills sync
```

`devclean init` ya lo corre solo al terminar (usa `npx skills add` contra
cada repo de origen); pasá `--sin-skills` para saltarlo y traerlas después
a mano. El fetch corre siempre contra la raíz del repo, nunca dentro de un
cuarto — los cuartos son worktrees separados y no ven archivos sin
commitear del worktree principal — así que el contenido se inyecta como
texto en el prompt, no como archivo que el agente deba encontrar. Una skill
que falta (sin red, o el repo no la tiene) se salta en silencio: nunca
bloquea al agente, mismo criterio que el examinador y la constitución.
Corre `git status` tras el primer `sync`: `.agents/skills/` y
`skills-lock.json` quedan sin trackear — decidí si versionarlos (build
reproducible) o gitignorarlos (se regeneran con `devclean skills sync`).

### Configuración Avanzada y Personalización

Si deseas sobreescribir modelos, agregar API keys específicas o definir agentes personalizados, puedes hacerlo en `.devclean/config.yml`:

```yaml
base: main                      # rama base del repo
pruebas: go test ./...          # comando de pruebas del proyecto
cli: claude                     # CLI de agente por defecto: claude | opencode
zonas_prohibidas: ["go.sum", "migrations/**", ".github/**"]
patrones_prueba: ["*_test.go", "test/**", "*.spec.ts"]
timeout_esclusa: 300            # segundos para el chequeo "falla hoy"
timeout_agente: 1200             # segundos por invocación del agente (bucle real, no la esclusa de entrada)
timeout_pruebas: 300             # segundos por corrida de listo_cuando y del paso bisectable en ship
recursion_max: 0                 # profundidad de recursión de tareas `recursivo: true`; 0 = desactivada
agentes:                        # sobreescribe arquetipos o agrega agentes con nombres propios
  backend:     { provider: opencode, model: opencode/muse-spark-1.2-contributor-free, key_env: OPENCODE_API_KEY, skills: ["go", "sql"] }
  specialist:  { provider: claude, model: sonnet, skills: ["machine-learning", "python"] }
estrategia: equilibrada         # ligera | equilibrada | pesada (peso por defecto)
modelos:                        # modelo por peso de tarea · lo rellena `devclean init`
  liviana: opencode/ling-3.0-flash-fin-free
  media: opencode-go/glm-5.3
  pesada: opencode-go/qwen3.8-max
reglas_import: ["api → dominio → datos"]  # opcional: verifica grafo de imports en ship
```

#### Los ids de modelo son los del CLI, no inventados

`devclean init` le pregunta al ejecutor qué modelos acepta de verdad
(`opencode models`; para `claude`, los alias `opus`/`sonnet`/`haiku`) y reparte
tres por peso de tarea en `modelos:`. En terminal te enseña lo que eligió y te
deja quedarte con eso o elegir cada peso a mano sobre el catálogo real. Cambialos
después a gusto — mandan sobre todo lo demás — y verificalos antes de gastar un
token:

```sh
devclean doctor     # avisa si algún modelo de config.yml no existe en el CLI
opencode models     # el catálogo real de tu cuenta
```

Un id mal escrito no falla a mitad de la corrida: falla en `doctor`. Y si
`modelos:` está vacío, cada invocación usa el modelo por defecto del CLI, que
siempre existe.

`agente` (por tarea) elige *quién* hace el trabajo; `peso` elige *con qué
modelo*. Un plan típico manda las tareas de andamiaje a `liviana` y las de
diseño a `pesada`, y así el gasto no es plano.

### Ejecutores soportados: Claude Code y OpenCode

devclean soporta de forma nativa tanto **Claude Code** (`claude`) como **OpenCode** (`opencode`):

- **Claude Code**:
  - Requiere `ANTHROPIC_API_KEY`.
  - Alias de modelo: `opus`, `sonnet`, `haiku`.
- **OpenCode**:
  - Requiere `OPENCODE_API_KEY`.
  - Los ids llevan **siempre** el prefijo del proveedor (`provider/modelo`);
    sin él, el servidor rechaza la invocación. Mirá el catálogo de tu cuenta
    con `opencode models`.
  - Soporta modelos premium (`opencode-go/glm-5.2`, `opencode-go/deepseek-v4-pro`, `opencode-go/qwen3.7-max`) y **modelos gratuitos** como:
    - `opencode/muse-spark-1.2-contributor-free`
    - `opencode/mimo-v2.5-free`
    - `opencode/nemotron-3-ultra-free`
    - `opencode/ling-3.0-flash-fin-free`

Puedes alternar el ejecutor y modelo dinámicamente con flags:

```sh
# Usando Claude
devclean run --ejecutor claude --modelo sonnet

# Usando OpenCode con modelo gratuito
devclean run --ejecutor opencode --modelo "opencode/muse-spark-1.2-contributor-free"

# O aplicando y corriendo todo en un solo paso
devclean up --ejecutor opencode --modelo "opencode/muse-spark-1.2-contributor-free"
```

---

## Comandos

| Comando | Qué hace |
|---|---|
| `devclean init [--sin-skills]` | detecta repo, rama base y comando de pruebas; crea `.devclean/`; trae las skills por defecto |
| `devclean skills sync` | trae con `npx skills add` los paquetes de skill que usan los agentes del catálogo |
| `devclean constitution [--forzar]` | genera `.devclean/constitution.md` con un modelo (§6.11) |
| `devclean plan "<texto>"` | convierte lenguaje natural en contratos de tarea (acepta `--export-spec`) |
| `devclean apply [-f <spec.yml>]` | aplica y valida una especificación declarativa (acepta `--dry-run`, `--run`) |
| `devclean up "<petición>" [--ship]` | **de una petición a un PR limpio en un solo comando**: planea, ejecuta en paralelo y entrega |
| `devclean up [-f <spec.yml>]` | sin petición: aplica la spec y ejecuta tareas en paralelo (estilo compose) |
| `devclean ps` | muestra el estado de tareas, cuartos activos y puertos asignados |
| `devclean task add\|edit\|rm\|list` | manejo manual de tareas (acepta `--agente`) |
| `devclean task seal <id> --visible <f> --oculta <f>` | sella una suite escrita a mano, sin gastar modelo (§6.8) |
| `devclean check <id>` | corre la esclusa de entrada sobre una tarea |
| `devclean run [--agentes N]` | ejecuta las tareas pendientes en paralelo (detecta solapamiento) |
| `devclean run --reintentar` | vuelve a correr las tareas detenidas, reusando su cuarto y su trabajo parcial |
| `devclean board` | tablero en vivo · se refresca solo y muestra intento, fase, modelo y atascos |
| `devclean standup` | parte de datos: COLISIÓN y ATASCO sin gastar tokens (§6.7) |
| `devclean ship <id>` | esclusa de salida (hasta 11 pasos) y PR de esa tarea |
| `devclean ship --todas` | la esclusa de cada tarea lista y **un solo PR** con un commit por tarea |
| `devclean logs <id>` | intentos, tokens, y el error del agente si la invocación falló |
| `devclean report` | métricas del proyecto |
| `devclean doctor` | verifica configuración, keys, permisos y git |

Todos aceptan `--plain` (una línea por evento, para CI) y `--json` (salida
estructurada). Sin flags, en terminal, algunos comandos usan la interfaz
interactiva. `run` recupera automáticamente cuartos huérfanos si
`.devclean/rooms/` fue borrado (p. ej. `git clean -fdx`).

## Métricas

`devclean report` mide del artefacto, no de lo que el agente dice de sí mismo.

| Métrica | Definición | Meta |
|---|---|---|
| Intentos hasta verde | vueltas de prueba y error por tarea | ≤ 2 |
| Ruido | % de líneas del PR que no sirven al objetivo | < 5% |
| Roce | conflictos de merge por cada 10 entregas | < 1 |
| Fricción | minutos entre PR abierto y aprobado | bajar |
| Rechazo en entrada | % de tareas rechazadas por mala definición | visible |
| Brecha (§6.8) | % suite visible − % suite oculta | cercana a 0 |

Cada métrica muestra su flecha de tendencia frente a la corrida anterior
(↑ subió, ↓ bajó, · sin cambio). El historial vive en
`.devclean/historial.jsonl`. Cada tarea guarda su costo en tokens. La
**brecha** solo aparece cuando el examinador selló una suite oculta; si la
suite oculta falló, la entrega guarda la brecha igualmente para el reporte.

## Seguridad

- Nada escucha en la red. Nunca.
- El agente solo escribe dentro de su cuarto y dentro de `tocar_solo`.
- Las keys nunca entran al contexto del modelo ni a logs.
- Escaneo de secretos obligatorio antes de cualquier PR.
- `devclean ship --dry-run` muestra todo antes de publicar.

## Por qué no reuniones entre agentes

Un agente nunca reporta su propio avance; todo se mide del artefacto. La
evidencia:

- Los agentes se conforman públicamente entre **64% y 94%** de las veces
  pese a oponerse en privado.
- Los modelos débiles corrigen solo el **3.6%** de sus sesgos de postura en
  un debate; abandonan juicios correctos por alinearse con la mayoría.
- El debate multi-agente es una martingala: no aporta ganancia esperada
  sobre el voto independiente.

`devclean standup` y el chequeo de solapamiento de `run` reemplazan ese
debate con dos detectores deterministas que cuestan cero tokens.

## Estado

**v0.6.2.** MVP (v0.1) + Parte B (v0.2/v0.3) + zero-config y OpenCode (v0.4)
+ skills reales, recursividad y fiabilidad de `ship` (v0.5) + de una petición
a un PR limpio (v0.6), todo en `main` con pruebas verdes.

- **v0.6.2:** devclean no servía en un proyecto que ya existe con la suite en
  verde. El planificador copiaba el comando de pruebas del proyecto
  (`npm test`, `go test ./...`) en los `listo_cuando`, y la esclusa de entrada
  rechazaba TODAS las tareas con "listo_cuando ya pasa" antes de gastar un
  token: el prompt nunca le decía que el comando tiene que fallar hoy. Y en los
  stacks sin examinador ciego (node, rust) nadie podía escribir las pruebas —
  el examinador no existe y al implementador se le vedaban esas rutas — así que
  el `listo_cuando` apuntaba a un archivo que jamás se creaba.
- **v0.6.1:** el presupuesto de líneas lo estima el planificador por tarea, en
  vez de una constante de 200 igual para todas — en una corrida real de siete
  tareas, tres la reventaban y el trabajo ya estaba hecho cuando la esclusa lo
  descubría. Además, el commit desde el que arranca cada cuarto se anota en su
  estado en vez de deducirse de los mensajes `wip:`: sin eso, las tareas de
  oleadas posteriores medían el cuarto entero contra la rama base, y la medida
  cambiaba entre pasadas de `ship`.
- **v0.6.0:**
  - **El motor invoca modelos de verdad.** Los arquetipos traían ids
    inventados (`glm-5.2`, `claude-sonnet`) que ningún CLI acepta: cada
    invocación moría en dos segundos sin gastar un token y la corrida entera
    caía en cascada. Ahora `init` le pregunta al ejecutor qué modelos existe
    y `doctor` los valida antes de gastar nada.
  - **Se puede saber qué falló.** `stderr` del CLI ya no se descarta; cada
    invocación deja su volcado en `.devclean/runs/<id>/intento-N.log`; el
    bucle distingue "el agente reventó" de "las pruebas fallaron" y corta en
    vez de quemar los tres intentos contra un error de infraestructura.
  - **De una petición a un PR limpio:** `devclean up "<petición>" --ship`
    encadena plan, corrida y entrega; `devclean ship --todas` junta un commit
    por tarea en un solo PR, en orden de dependencia, con la suite del
    proyecto corriendo sobre el conjunto integrado.
  - **Progreso y atascos en todo momento:** el bucle escribe su estado vivo
    en `latido.json` en cada cambio de fase, así `board` (que ahora se
    refresca solo) y `standup` ven qué pasa DENTRO de un intento. Una fase
    parada más de diez minutos se marca como ATASCO.
  - **Elecciones del humano:** `plan` deja marcar qué tareas se crean (y
    arrastra las dependientes de una descartada), `init` deja elegir el
    modelo de cada peso sobre el catálogo real, y el tablero gana `r`
    (reintentar) y `d` (detalle) junto a `s`.
  - **Menos tokens:** la base de skills inyectaba ~59 KB (~15k tokens) en
    cada prompt de cada intento; quedan `implement` y `clean-code`. El gasto
    de OpenCode, que se leía mal, ahora se contabiliza.
  - **Desbloqueos:** `go.sum` acompaña a `go.mod` cuando está en alcance (sin
    eso ningún proyecto Go con dependencias compilaba), el examinador ya no
    emite suites que nadie puede arreglar, `fmt.Println` en un `main.go` dejó
    de contar como ruido, y `run --reintentar` revive tareas detenidas
    reusando su cuarto.
- **v0.5:**
  - **Skills reales por agente:** cada arquetipo trae paquetes de skill de
    verdad (`SKILL.md`, no solo una etiqueta) traídos con `devclean skills
    sync` e inyectados completos en el prompt — ver [Arquetipos de Agentes
    Predefinidos](#arquetipos-de-agentes-predefinidos-zero-config).
  - **Ejecución recursiva de tareas:** una tarea `recursivo: true` se reparte
    en subtareas reales que corren en cuartos anidados dentro de su propio
    cuarto, con vista de árbol en `board`/`standup`/TUI — ver
    [Recursividad](#recursividad-tareas-que-se-reparten-solas).
  - **Fiabilidad de `ship`:** `git push --force` en la rama del cuarto y
    detección de PR duplicado (`gh pr view`) evitan que un reintento tras un
    fallo parcial (p. ej. `gh pr create` caído por red después del push) se
    quede sin poder abrir el PR nunca. `timeout_agente`/`timeout_pruebas`
    configurables — el default de 5 minutos por invocación de agente mataba
    intentos reales a mitad de terminar.
- **v0.4:** soporte nativo de OpenCode (modelos premium y gratuitos) junto a
  Claude Code, con precedencia de ejecutor y auto-approve en modo headless;
  catálogo de agentes predefinidos zero-config (`backend`, `frontend`,
  `architect`, `tester`, `refactor`, `ejecutor`); modelo declarativo
  `devclean.spec.yml` (`apply`, `up`, `ps`) estilo Docker Compose.
- **v0.2/v0.3:** esclusa de entrada blindada (`sanearAlcance` recorta rutas
  vedadas del plan antes de escribir el contrato), cuartos huérfanos
  recuperados, contratos entre tareas (§6.10: `expone`/`usa`), examinador
  ciego y suite oculta (§6.8), solapamiento activo (§6.9), parte de datos
  (§6.7), constitución (§6.11). Binarios estáticos publicados
  (linux/darwin/windows × amd64/arm64) desde v0.2.

**Pendiente real:** duplicación entre ramas, solapamiento funcional completo
(merge en seco + correr suites), mutation testing del examinador, y sumar el
costo en tokens de la recursión a `devclean report`. El camino de `gh pr
create` está probado en su mecánica de git (ramas, commits, suite integrada)
pero todavía no contra un repositorio de GitHub real de punta a punta.

> El GIF (`docs/demo.gif`) se graba con `vhs docs/demo.tape`; el tape usa
> `scripts/demo-env.sh` (agente falso para no gastar tokens) y muestra la TUI.

## Límite honesto

devclean sirve para lo que tiene oráculo: un comando que decide verde o rojo.
No verifica interfaz gráfica, comportamiento visual ni criterios difusos. El
examinador ciego es de caja negra sobre la interfaz pública; sin expone no
hay prueba. Prometer más quema el proyecto.

**El examinador ciego solo existe para Go y Python.** En los demás stacks
(node, rust) las pruebas las escribe quien implementa, y las rutas de prueba
dejan de estar vedadas: la regla de que el implementador no toca el examen
solo tiene sentido si hay alguien más que lo escriba. Ahí devclean sigue
sirviendo — el `listo_cuando` es igual de vinculante — pero sin la garantía de
que las pruebas las redactó alguien que no vio la implementación.

**`listo_cuando` tiene que fallar hoy.** Es la regla que hace que una tarea
signifique algo, y la esclusa de entrada la comprueba ejecutándolo antes de
gastar un token. Por eso el comando de pruebas del proyecto tal cual
(`npm test`, `go test ./...`) no sirve en un repo con la suite verde: hay que
apuntar a lo que la tarea va a crear.

## Licencia

MIT.
