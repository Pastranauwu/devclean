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
devclean board       # tablero
devclean standup     # parte de datos: colisiones y atascos
devclean ship T-001  # esclusa de salida + PR
```

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

### Configuración

`.devclean/config.yml`:

```yaml
base: main                      # rama base del repo
pruebas: go test ./...          # comando de pruebas del proyecto
cli: claude                     # CLI de agente por defecto: claude | opencode
zonas_prohibidas: ["go.sum", "migrations/**", ".github/**"]
patrones_prueba: ["*_test.go", "test/**", "*.spec.ts"]
timeout_esclusa: 300            # segundos para el chequeo "falla hoy"
agentes:                        # agentes con nombre libre, proveedor y skills (Fase 1)
  architect:   { provider: claude, model: claude-sonnet, skills: ["diseno", "arquitectura"] }
  implementer: { provider: opencode, model: glm-5.2, key_env: OPENCODE_API_KEY, skills: ["go", "refactor"] }
  tester:      { provider: claude, model: claude-haiku, skills: ["tests", "cobertura"] }
estrategia: equilibrada         # ligera | equilibrada | pesada (peso por defecto)
modelos:                        # modelo por peso de tarea (Fase 3)
  liviana: glm-4
  media: glm-5.2
  pesada: claude-sonnet
reglas_import: ["api → dominio → datos"]  # opcional: verifica grafo de imports en ship
```

También se soporta el bloque clásico `proveedores:` por retrocompatibilidad:

```yaml
proveedores:                    # modelo y key por rol (§8.1)
  planificador: { modelo: claude-sonnet, key_env: ANTHROPIC_API_KEY }
  ejecutor:     { modelo: glm-5.2, key_env: OPENCODE_API_KEY }
```

En `agentes:`, cada entrada define un nombre libre (`architect`, `implementer`, `tester`, etc.). `provider` debe ser `claude` u `opencode`. Si conviven `agentes:` y `proveedores:`, `agentes:` gana para los roles que define, y los restantes se resuelven por `proveedores:`. Las `skills` por ahora se inyectan como contexto adicional en el prompt del modelo ("Habilidades de este rol: ..."), no como lógica de comportamiento autónoma.

Sin `cli`, devclean usa el primer CLI que encuentre instalado. Fíjalo
cuando tengas los dos y quieras uno concreto (por ejemplo, si se te
acabó la cuota de uno). `--ejecutor` lo pisa por corrida:

```sh
devclean run --ejecutor claude
```

---

## Comandos

| Comando | Qué hace |
|---|---|
| `devclean init` | detecta repo, rama base y comando de pruebas; crea `.devclean/` |
| `devclean constitution [--forzar]` | genera `.devclean/constitution.md` con un modelo (§6.11) |
| `devclean plan "<texto>"` | convierte lenguaje natural en contratos de tarea (acepta `--export-spec`) |
| `devclean apply [-f <spec.yml>]` | aplica y valida una especificación declarativa (acepta `--dry-run`, `--run`) |
| `devclean up` | aplica la spec y ejecuta tareas en paralelo en cuartos aislados (estilo compose) |
| `devclean ps` | muestra el estado de tareas, cuartos activos y puertos asignados |
| `devclean task add\|edit\|rm\|list` | manejo manual de tareas (acepta `--agente`) |
| `devclean task seal <id> --visible <f> --oculta <f>` | sella una suite escrita a mano, sin gastar modelo (§6.8) |
| `devclean check <id>` | corre la esclusa de entrada sobre una tarea |
| `devclean run [--agentes N]` | ejecuta las tareas pendientes en paralelo (detecta solapamiento) |
| `devclean board` | tablero de estado · en TUI, `s` dispara `ship --dry-run` sobre la tarea lista |
| `devclean standup` | parte de datos: COLISIÓN y ATASCO sin gastar tokens (§6.7) |
| `devclean ship <id>` | esclusa de salida (hasta 11 pasos) y PR |
| `devclean logs <id>` | detalle interno de una tarea |
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

**v0.3.x (Parte B completa).** El MVP (v0.1) más todo lo de v0.2 ya está
implementado y verificado en `main`:

- **v0.3.2** publicada en GitHub con binarios estáticos (linux/darwin/windows
  × amd64/arm64), `checksums.txt` e `install.sh` como asset. `v0.2` y `v0.3`
  anteriores también publicadas. Instalación con una línea vía `curl` o con
  `go install` verificada.
- **Esclusa de entrada blindada:** `plan` ya no genera contratos que la
  propia esclusa rechaza. `sanearAlcance` recorta rutas vedadas de
  `tocar_solo` con aviso; el prompt del planificador recibe la lista de
  vedadas. `gate.AlcanceProhibido` compartida.
- **Cuartos huérfanos recuperados:** `room.Create`/`Destroy` limpian ramas y
  worktrees huérfanos si `.devclean/rooms/` fue borrado (queda gitignored).
  Verifica la rama con `rev-parse`, sin depender del idioma de git.
- **Contratos entre tareas (§6.10):** `expone`/`usa` congelados y verificados
  en `plan` → `run` (rechaza `usa` huérfano) → `ship` (paso `interfaces`) +
  `reglas_import` en `ship` (paso `dependencias`).
- **Examinador ciego y suite oculta (§6.8):** `internal/examiner` genera
  `devclean_visible_test.go` (70%) y sella `devclean_hidden_test.go` (30%)
  con `internal/sealed`; `ship` lo verifica y lo quema (paso
  `suite_oculta`). Declara `imports` por suite, filtra `imported and not
  used`, chequea sintaxis con `go/parser` y se salta si `expone` está vacío
  (sin frontera no hay examen; evita falsos solapamientos).
- **Solapamiento activo (§6.9):** `internal/overlap` — `git merge-tree` +
  símbolos exportados de `attempts.jsonl` — corre en cada oleada de `run`.
- **Parte de datos (§6.7):** `devclean standup` + `internal/standup` —
  COLISIÓN y ATASCO derivados de `attempts.jsonl`, cero tokens.
- **Constitución (§6.11):** `devclean constitution` + `internal/constitution`
  — se genera con el modelo, se guarda en `.devclean/constitution.md` y se
  inyecta en el prompt del planificador y de cada agente.

**Pendiente real:** duplicación entre ramas, solapamiento funcional completo
(merge en seco + correr suites) y mutation testing del examinador. Todo lo
demás del PRD ya está en `main` con pruebas verdes.

> El GIF (`docs/demo.gif`) se graba con `vhs docs/demo.tape`; el tape usa
> `scripts/demo-env.sh` (agente falso para no gastar tokens) y muestra la TUI.

## Límite honesto

devclean sirve para lo que tiene oráculo: un comando que decide verde o rojo.
No verifica interfaz gráfica, comportamiento visual ni criterios difusos. El
examinador ciego es de caja negra sobre la interfaz pública; sin expone no
hay prueba. Prometer más quema el proyecto.

## Licencia

MIT.
