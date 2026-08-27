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

## Manifiesto

1. El historial del agente no es el historial del proyecto.
2. Nadie toca lo que no reclamó.
3. Verificar es trabajo de código, no del modelo.
4. Lo que no se hizo, se declara.
5. Paralelo para trabajar, en fila para entregar.
6. Si no puedes escribir el comando que dice "ya está", la tarea no existe.

## Instalación

```sh
# Homebrew
brew install Pastranauwu/tap/devclean

# Script de una línea
curl -fsSL https://github.com/Pastranauwu/devclean/releases/latest/download/install.sh | sh

# Go
go install github.com/Pastranauwu/devclean/cmd/devclean@latest
```

Binario estático único (Go), sin runtime que instalar. Requiere `git` y al
menos un CLI de agente (`opencode` o `claude`). `devclean doctor` lo verifica.

## Uso

```sh
devclean init                        # detecta repo, rama base y comando de pruebas
devclean plan "lo que necesitas"     # convierte lenguaje natural en contratos
devclean task add "otra cosa"        # o escribe contratos a mano
devclean run --agentes 3             # ejecuta en paralelo
devclean board                       # tablero de estado
devclean ship T-001                  # esclusa de salida y PR
devclean logs T-001                  # detalle interno de una tarea
devclean report                      # métricas
devclean doctor                      # verifica el entorno
```

El contrato de tarea son 8 campos máximo:

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
```

## Métricas

`devclean report` mide del artefacto, no de lo que el agente dice de sí mismo.

| Métrica | Definición | Meta |
|---|---|---|
| Intentos hasta verde | vueltas de prueba y error por tarea | ≤ 2 |
| Ruido | % de líneas del PR que no sirven al objetivo | < 5% |
| Roce | conflictos de merge por cada 10 entregas | < 1 |
| Fricción | minutos entre PR abierto y aprobado | bajar |
| Rechazo en entrada | % de tareas rechazadas por mala definición | visible |

Cada tarea guarda su costo en tokens.

## Por qué no reuniones entre agentes

Un agente nunca reporta su propio avance; todo se mide del artefacto. La
evidencia:

- Los agentes se conforman públicamente entre **64% y 94%** de las veces
  pese a oponerse en privado.
- Los modelos débiles corrigen solo el **3.6%** de sus sesgos de postura en
  un debate; abandonan juicios correctos por alinearse con la mayoría.
- El debate multi-agente es una martingala: no aporta ganancia esperada
  sobre el voto independiente.

## Límite honesto

devclean sirve para lo que tiene oráculo: un comando que decide verde o rojo.
No verifica interfaz gráfica, comportamiento visual ni criterios difusos.
Prometer más quema el proyecto.

## Licencia

MIT.
