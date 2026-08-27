# Adenda al PRD de devclean

**Aplica sobre:** `docs/PRD.md` v0.1
**Fecha:** 26 agosto 2026
**Estado del proyecto:** fase 1 cerrada, fase 2 en planificación

Esta adenda tiene tres partes:
- **A.** Cambios urgentes: entran antes de que termine la fase 2 o hay que reescribir código.
- **B.** Secciones nuevas: se agregan al PRD pero se implementan en v0.2.
- **C.** Veredictos sobre las decisiones que tomó el agente donde el PRD callaba.

---

# PARTE A — Cambios urgentes (bloquean la fase 2)

## A.1 Versión en el contrato — CRÍTICO

El parser de la fase 1 **rechaza campos desconocidos**. Correcto, pero significa que cualquier campo que agreguemos en v0.2 rompe todos los contratos existentes.

**Cambio:** agregar campo obligatorio `version: 1` al contrato (§6.1). El parser acepta campos desconocidos **solo** si la versión del archivo es mayor a la que conoce el binario, y en ese caso avisa: `contrato versión 2, binario soporta 1 · actualiza devclean`.

Cinco minutos ahora, un dolor de cabeza evitado en tres meses.

## A.2 Instrumentación del bucle — CRÍTICO

Este es el cambio importante. El bucle de la fase 2 (§6.4) debe **emitir hechos medibles en cada intento**, no solo verde/rojo. Si no se instrumenta ahora, hay que reescribirlo entero para las métricas y para el parte de datos.

Cada intento escribe una línea en `.devclean/runs/T-001/attempts.jsonl`:

```json
{
  "intento": 2,
  "inicio": "2026-08-26T14:31:02Z",
  "fin": "2026-08-26T14:33:47Z",
  "salida_codigo": 1,
  "tests_pasaron": 5,
  "tests_fallaron": 4,
  "archivos_tocados": ["src/export/writer.go"],
  "simbolos_exportados": ["WriteCSV", "csvOptions"],
  "lineas_mas": 118,
  "lineas_menos": 12,
  "revertidos_fuera_de_alcance": [],
  "tokens": {"entrada": 18400, "salida": 3100},
  "modelo": "glm-5.2"
}
```

Notas de implementación:
- `tests_pasaron` / `tests_fallaron` salen de parsear la salida de `listo_cuando`. Si no se puede parsear, se deja `null` y solo se usa el código de salida. **No inventar números.**
- `simbolos_exportados` se obtiene del diff de la rama con `go/ast` para Go y con heurística por lenguaje en el resto. Si el lenguaje no se soporta, `null`.
- Este archivo es la fuente de todas las métricas y del parte de datos. Nada se recalcula después.

## A.3 Las rutas de prueba nunca entran en `tocar_solo`

En v0.2 un examinador ciego escribirá las pruebas. Para que eso sea posible, el implementador **nunca** debe poder editarlas.

**Cambio:** el validador de contrato rechaza si `tocar_solo` incluye rutas que casan con los patrones de prueba detectados en `init` (`*_test.go`, `test/**`, `spec/**`, `*.spec.ts`, etc.). El bucle revierte automáticamente cualquier archivo de prueba modificado y lo registra en `revertidos_fuera_de_alcance`.

Barato ahora, imposible de retrofitear sin romper contratos existentes.

## A.4 Corrección: `tocar_solo` vacío

La decisión del agente fue "vacío = sin restricción (todo el repo menos zonas prohibidas)". Eso contradice la regla 2 del manifiesto y hace inviable la detección de cruces.

**Regla correcta:**
- Una sola tarea en curso → vacío permitido, sin restricción.
- Dos o más tareas en curso → `tocar_solo` obligatorio. La esclusa de entrada rechaza contratos vacíos con: `tocar_solo obligatorio con más de una tarea activa · declara tus rutas`.

## A.5 Requisito de sobrecarga

Nuevo requisito no funcional, con rango de sección 10:

> **La sobrecarga de devclean no debe superar el 10% del tiempo total de la tarea.** Cuando todo va bien, la salida es una línea por tarea. Cualquier verificación que no quepa en ese presupuesto se corre en segundo plano o se mueve a v0.2.

Motivo: el hallazgo de METR fue sobrecarga de validación. Una herramienta que agrega fricción visible reproduce el problema que dice resolver. Este requisito debe poder matar features buenas.

---

# PARTE B — Secciones nuevas (v0.2, se documentan ahora)

## §6.7 Parte de datos duros

Sustituye cualquier idea de reuniones o reportes entre agentes.

**Principio:** un agente nunca reporta su propio avance. Todo se mide del artefacto.

Evidencia que lo justifica (va en el README, no solo aquí):
- Los agentes se conforman públicamente entre 64% y 94% de las veces pese a oponerse en privado.
- Los modelos débiles corrigen solo el 3.6% de sus sesgos de postura en un debate; abandonan juicios correctos por alinearse con la mayoría.
- El debate multi-agente es una martingala: no aporta ganancia esperada sobre el voto independiente.

**Comando:** `devclean standup`, disparado por evento, nunca por reloj. Eventos: un agente toca un símbolo compartido, termina, agota intentos, o supera el presupuesto de diff.

Contenido, todo derivado de `attempts.jsonl`:

```
PARTE 14:32 · 3 tareas en curso

⚠ COLISIÓN   T-001 cambia la firma de Format(); T-002 la invoca en 3 lugares
⚠ ATASCO     T-003 sin cambio en el conteo de fallos desde hace 11 min
⚠ DESVÍO     T-001 agrega caché en memoria; el contrato no lo pide
             evidencia: src/export/writer.go:88
✓            T-002 dentro de contrato
```

Las dos primeras alertas son deterministas, cuestan cero tokens. La tercera es la única que usa un modelo.

**Reglas del juicio con modelo** (solo para DESVÍO):
1. Modelo distinto al que escribió el código.
2. Juicio emitido **antes** de ver el de cualquier otro agente.
3. Toda objeción cita `archivo:línea`. Sin cita, se descarta automáticamente.
4. Protocolo Disagree-or-Commit: o critica explícitamente, o respalda con evidencia nueva.
5. Cero rondas de debate. Si hay desacuerdo, escala al humano.

Bitácora inmutable en `.devclean/standups/`.

**Prohibido:** rondas de debate, un agente coordinador que lea los reportes de los demás, y pedir a un agente que califique su avance en porcentaje.

## §6.8 Examinador ciego y suite oculta

**Orden obligatorio:**
```
contrato aprobado → examinador escribe la suite → se sella (hash) → arranca el implementador
```

**Qué ve el examinador:** el contrato y la frontera pública (firmas, endpoints, CLI, esquema de datos).
**Qué no ve:** el cuerpo de las funciones.

Regla que simplifica todo: las pruebas corren contra el borde del sistema (CLI, HTTP, base de datos), no contra funciones internas. Ahí el contrato **es** la interfaz.

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

## §6.9 Detección de solapamiento en tres niveles

| Nivel | Método | Costo | Cuándo |
|---|---|---|---|
| Textual | `git merge-tree` entre pares de ramas activas | milisegundos | cada evento |
| Semántico | símbolos exportados modificados en común (de `attempts.jsonl`) | nulo | cada evento |
| Funcional | merge en seco + correr las suites de ambas ramas sobre el resultado | alto | solo si textual o semántico marcaron sospecha |

El nivel funcional es el que atrapa el fallo clásico: dos ramas verdes por separado que rompen juntas.

## §6.10 Integración y acoplamiento

Ataca directamente el hallazgo de GitClear: el código "movido" (señal de refactorización) se desplomó de 15.88% a 3.10% mientras subían el añadido y el copiado. Los agentes agregan y duplican; no integran.

- **Duplicación entre ramas:** comparar estructura de funciones nuevas entre ramas activas. Determinista y barato.
- **Contratos entre tareas:** si T-001 produce una interfaz que T-002 consume, la firma se congela en ambos contratos y se verifica contra los dos diffs.
- **Reglas de dependencia:** dirección permitida entre módulos declarada en config (`api → dominio → datos`); se verifica el grafo de imports del diff.

## §6.11 Constitución del proyecto

Archivo `.devclean/constitution.md`, inyectado en el contexto de **todos** los agentes: convenciones de estilo, capas, manejo de errores, patrones prohibidos.

Motivo: agentes en paralelo toman decisiones implícitas distintas sobre estilo, casos borde y arquitectura. Dos tareas pueden pasar sus pruebas y aun así elegir abstracciones incompatibles. Ninguna esclusa de las anteriores detecta eso.

Se genera con el agente en modo entrevista la primera vez y se versiona en el repo.

---

# PARTE C — Veredictos sobre las decisiones del agente

## Fase 1

| # | Decisión | Veredicto |
|---|---|---|
| 1 | Parser YAML a mano, duplicado en config y task | Aceptada, pero **deduplicar** a `internal/kv` en la fase 2. Dos parsers que divergen es un bug garantizado. |
| 2 | Timeout fijo de 5 min | Corregir: configurable, default 5 min. Ya está en su lista. |
| 3 | `task check` como subcomando | Aceptada. El alias `devclean check` es cosmético, va cuando sobre tiempo. |
| 4 | Rama base `origin/HEAD → main/master → actual` | Aceptada. Cae a rama actual solo si no hay remoto; si hay remoto y falla, error, no adivinanza. |
| 5 | `make test` sobre `pytest` si hay ambos | Aceptada, pero `init` debe **mostrar** qué eligió y permitir corregirlo. Detección silenciosa equivocada cuesta horas. |

## Fase 2

| Decisión | Veredicto |
|---|---|
| Orden de 6 bloques | Correcto. No cambiar. |
| Estado de tarea fuera del contrato, en `.devclean/state/` | **Correcta y bien vista.** Formalizar como §6.2b del PRD. Es la base del parte de datos. |
| `tocar_solo` vacío = sin restricción | **Corregir** según A.4. |
| Commits `wip:` por intento, destruidos en `ship` | Correcto. |
| Los límites por prompt, la reversión es el enforcement real | **Correcto y es la decisión más importante que ha tomado.** Esa es la tesis del proyecto: la verificación es código, no confianza en el modelo. |
| Sin TUI, una línea por evento | Correcto. La TUI es fase 4. |
| Implementar ambos adaptadores (opencode y claude) | Aceptable ya que tiene los dos instalados. Si aprieta el tiempo, opencode primero y claude después: dos adaptadores a medias no cierran la fase. |
| Preparación de entorno "auto según manifiesto" | Aceptada, con límite: si el manifiesto no se reconoce, **no adivinar** — avisar y dejar que el usuario declare el comando de preparación en config. |

---

# Resumen de aplicación

**Ahora, antes de cerrar la fase 2:** A.1, A.2, A.3, A.4, A.5 y las correcciones de la Parte C.

**Al PRD como documentación, sin implementar:** §6.7 a §6.11.

**Alcance confirmado:**

| v0.1 (3 semanas) | v0.2 | Nunca |
|---|---|---|
| Contrato, esclusa de entrada | Examinador ciego, suite oculta, brecha | Debate entre agentes |
| Cuartos aislados | Duplicación entre ramas | Auto-reportes de avance |
| Bucle instrumentado | Reglas de dependencia | Interfaz web |
| Esclusa de salida | Contratos entre tareas | Verificación de UX o rendimiento |
| Parte de datos (niveles textual y semántico) | Mutation score | |
| Métricas básicas | Colisión funcional | |
