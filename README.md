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
  ✓ base · ✓ historial · ✓ ruido · ✓ secretos · ✓ presupuesto · ✓ bisectable · ✓ handoff · ✓ pr
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

Dos esclusas y un contrato. Antes de gastar un token, la tarea se define con
un comando ejecutable que dice "ya está" (`listo_cuando`). El agente trabaja
en un cuarto aislado, y **la verificación la hace código, nunca el modelo**.

1. **Esclusa de entrada** — `listo_cuando` debe existir, fallar hoy y no
   pisar el alcance de otra tarea. Una tarea mal definida se rechaza con un
   motivo legible, antes de gastar un solo token.
2. **Bucle de intentos** — el agente edita dentro de su alcance; devclean
   revierte lo que se salió, ejecuta `listo_cuando` y decide. Verde → entregado;
   rojo → se devuelve el error al agente. Agotados los intentos → se detiene
   con una pregunta concreta, no con un arreglo inventado.
3. **Esclusa de salida** — ocho pasos deterministas en `ship`: rebase,
   historial aplanado, sin ruido, sin secretos, dentro de presupuesto,
   bisectable, handoff, y recién entonces el PR.

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

# 2. convierte una petición en tareas (el planificador usa un modelo)
devclean plan "exportar clientes a CSV y arreglar el login con tildes"

# 3. ejecuta en paralelo
devclean run --agentes 3

# 4. revisa y entrega
devclean board
devclean ship T-001
```

`init` detecta la rama base y el comando de pruebas; si se equivoca, corrígelo
con `devclean init --pruebas "mi comando"` o editando `.devclean/config.yml`.

## Adoptarlo en un proyecto real

devclean no exige cambiar tu stack: solo crea una carpeta `.devclean/` y
trabaja en worktrees aislados. No toca tu rama principal hasta `ship`.

1. **Prepara el terreno.** Asegúrate de que el repo compila y que tienes un
   comando de pruebas. `devclean doctor` revisa git, configuración, ejecutores
   y keys.

2. **Inicializa.** `devclean init` detecta la rama base y el comando de
   pruebas. Si tu comando no se detecta, pásalo con `--pruebas`.

3. **Delimita lo que nadie debe tocar.** En `.devclean/config.yml`, revisa
   `zonas_prohibidas` (lockfiles, migraciones, CI, changelog). Por defecto ya
   están los sospechosos habituales.

4. **Define el trabajo.** Con `devclean plan` (modelo) o a mano con
   `devclean task add` + `devclean task edit`. La regla de oro: `listo_cuando`
   debe ser un comando que **hoy falla** y que el agente hará pasar.

5. **Acota cada tarea.** `tocar_solo` declara qué rutas puede tocar el
   agente. Si corres varias tareas a la vez, es obligatorio: sin alcance
   declarado no hay cruce que detectar.

6. **Ejecuta y revisa.** `devclean run --agentes N`, luego `devclean board`,
   `devclean logs T-001` y `devclean report`.

7. **Entrega.** `devclean ship T-001 --dry-run` para ver los ocho pasos sin
   publicar; `devclean ship T-001` para abrir el PR (requiere `gh`).

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
peso: liviana                                # opcional: liviana | media | pesada
limite_intentos: 3
limite_lineas: 200
riesgos: archivos grandes pueden agotar memoria
---
```

### Configuración

`.devclean/config.yml`:

```yaml
base: main                      # rama base del repo
pruebas: go test ./...          # comando de pruebas del proyecto
zonas_prohibidas: ["go.sum", "migrations/**", ".github/**"]
patrones_prueba: ["*_test.go", "test/**", "*.spec.ts"]
timeout_esclusa: 300            # segundos para el chequeo "falla hoy"
proveedores:                    # modelo y key por rol (§8.1)
  planificador: { modelo: claude-sonnet, key_env: ANTHROPIC_API_KEY }
  ejecutor:     { modelo: glm-5.2, key_env: OPENCODE_API_KEY }
estrategia: equilibrada         # ligera | equilibrada | pesada (peso por defecto)
modelos:                        # modelo por peso de tarea (Fase 3)
  liviana: glm-4
  media: glm-5.2
  pesada: claude-sonnet
```

---

## Comandos

| Comando | Qué hace |
|---|---|
| `devclean init` | detecta repo, rama base y comando de pruebas; crea `.devclean/` |
| `devclean plan "<texto>"` | convierte lenguaje natural en contratos de tarea |
| `devclean task add\|edit\|rm\|list` | manejo manual de tareas |
| `devclean check <id>` | corre la esclusa de entrada sobre una tarea |
| `devclean run [--agentes N]` | ejecuta las tareas pendientes en paralelo |
| `devclean board` | tablero de estado |
| `devclean ship <id>` | esclusa de salida y PR |
| `devclean logs <id>` | detalle interno de una tarea |
| `devclean report` | métricas del proyecto |
| `devclean doctor` | verifica configuración, keys, permisos y git |

Todos aceptan `--plain` (una línea por evento, para CI) y `--json` (salida
estructurada). Sin flags, en terminal, algunos comandos usan la interfaz
interactiva.

## Métricas

`devclean report` mide del artefacto, no de lo que el agente dice de sí mismo.

| Métrica | Definición | Meta |
|---|---|---|
| Intentos hasta verde | vueltas de prueba y error por tarea | ≤ 2 |
| Ruido | % de líneas del PR que no sirven al objetivo | < 5% |
| Roce | conflictos de merge por cada 10 entregas | < 1 |
| Fricción | minutos entre PR abierto y aprobado | bajar |
| Rechazo en entrada | % de tareas rechazadas por mala definición | visible |

Cada métrica muestra su flecha de tendencia frente a la corrida anterior
(↑ subió, ↓ bajó, · sin cambio). El historial vive en
`.devclean/historial.jsonl`. Cada tarea guarda su costo en tokens.

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

## Estado

**v0.1 (MVP).** Lo de arriba funciona de punta a punta. Queda pendiente para
v0.2: examinador ciego y suite oculta, detección de solapamiento funcional,
duplicación entre ramas, reglas de dependencia y constitución del proyecto.

> El GIF (`docs/demo.gif`) se graba con `vhs docs/demo.tape`; el tape usa
> `scripts/demo-env.sh` (agente falso para no gastar tokens) y muestra la TUI.

## Límite honesto

devclean sirve para lo que tiene oráculo: un comando que decide verde o rojo.
No verifica interfaz gráfica, comportamiento visual ni criterios difusos.
Prometer más quema el proyecto.

## Licencia

MIT.
