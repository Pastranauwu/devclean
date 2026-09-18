# Historia de devclean

Este es el único documento histórico del proyecto: por qué existe, qué se fue
construyendo, qué se decidió y qué se tiró. Sustituye al PRD original, a su
adenda y al traspaso entre sesiones, que describían un producto anterior al
actual; los tres salieron del repo y siguen en el historial de git.

Para usar la herramienta, el `README.md`. Para desarrollarla, `CLAUDE.md`. Para
el detalle de un cambio, `git log`.

---

## 1. La intención

devclean dirige a varios agentes de IA programando en paralelo sobre un mismo
repositorio y garantiza que lo único que llega al proyecto sea código limpio,
probado y con historial legible.

La analogía que lo originó: una gerencia de consultoría de software. El humano
dice qué quiere; devclean define, reparte, supervisa, prueba y entrega. El
humano recibe el resultado, no el desorden.

### El problema que lo justifica

Los datos que se usaron para decidir construirlo, a agosto de 2026:

- El tiempo para abrir un pull request cayó **58%** con IA, pero esos PRs pasan
  **4.6 veces más tiempo** en revisión por falta de control automatizado
  (Opsera, 2026).
- El *churn* —líneas reescritas o revertidas en menos de dos semanas— subió de
  **3.31% (2021) a 6.87% (2024)** (GitClear, 211M líneas).
- El código "movido", señal de refactorización, se desplomó de **15.88% a
  3.10%**, mientras subían el añadido y el copiado.
- DORA 2024: calidad del código **+3.4%**, estabilidad de entrega **−7.2%**.

Diagnóstico: la IA mejora la línea suelta y empeora el sistema. Generar dejó de
ser el cuello de botella; el costo se mudó a revisar, depurar y probar a ciegas.

Causa raíz: el agente entra a programar sin una definición ejecutable de
"listo". Sin criterio de éxito un agente no puede parar, solo puede seguir
intentando. De ahí el ciclo infinito de prueba y error, el código muerto y las
soluciones que no encajan con el resto del proyecto.

### Qué NO es

Anti-alcance explícito, vigente desde el primer día:

- **No** es un modelo ni un agente. Usa los que el usuario ya paga.
- **No** es un servicio en la nube. No hay backend, cuenta ni servidor.
- **No** es interfaz web ni dashboard. Todo en terminal.
- **No** reemplaza CI/CD. Actúa **antes** del PR.
- **No** es "AgentOps" ni "Agentic DevOps": operar agentes en producción o
  ponerlos a manejar el pipeline son otras categorías, que ya existen.
- **No** gestiona despliegues, infraestructura ni monitoreo.

---

## 2. Cómo se construyó

144 commits entre el 26 de agosto y el 18 de septiembre de 2026.

### Los cimientos (26–27 ago, `v0.1.0`)

En orden: el binario con cobra y las salidas `--plain`/`--json`; el parser del
subconjunto yaml y la configuración; `init` detectando repo, rama base y comando
de pruebas; el contrato de tarea con validador y almacén; la esclusa de entrada;
los estados en `.devclean/state`; los cuartos aislados con `git worktree`.

Tres decisiones se tomaron aquí y aguantaron todo lo demás:

- **El parser yaml se compartió** entre config y contrato (`internal/kv`) para
  que no hubiera dos dialectos que se separaran con el tiempo.
- **`version` es obligatoria en el contrato**: sin ella no hay forma de leer
  tolerantemente un archivo del futuro.
- **Las rutas de prueba nunca entran en `tocar_solo`**, y con más de una tarea
  en curso `tocar_solo` es obligatorio. El agente no escribe el examen que lo
  juzga, y nadie trabaja sin declarar dónde.

Luego el motor: los adaptadores `opencode` y `claude` detrás de la interfaz
`Executor`, el bucle de trabajo instrumentado en `attempts.jsonl`, `run` con N
tareas en paralelo, la esclusa de salida, las cinco métricas, `board`, `logs`,
`doctor`, la compuerta animada y el planificador.

### La cara (27 ago, `v0.2.x`)

El TUI dejó de ser texto con color: logo de píxeles con degradado, tarjetas con
borde recto, plasma truecolor animado de fondo, 10fps para matar el parpadeo. La
demo se volvió reproducible con un agente falso y `vhs`, y costó media docena de
commits acertarle a los tiempos del tape.

También entraron `proveedores` con modelo y key por rol, la flecha de tendencia
frente a la corrida anterior, las oleadas con `depende_de`, los pesos, y
`expone`/`usa` congelados y verificados: el primer intento de cerrar la costura
entre tareas que corren ciegas.

### La parte B (28 ago, `v0.3.x`)

De un tirón: el parte de datos determinista, el examinador ciego con suite
oculta, la detección de solapamiento, la verificación del grafo de imports y la
constitución del proyecto. Después, varias rondas de endurecer el examinador
—imports declarados, chequeo de sintaxis, saltar cuando no hay `expone`— y de
recuperar cuartos huérfanos.

Aquí aparece el patrón que domina el resto de la historia: cada feature nueva se
paga con dos o tres fixes de lo que el mundo real le hace.

### Requerimientos como código (31 ago, `v0.4.x`)

El salto conceptual: `devclean.spec.yml`, un archivo declarativo al estilo
docker compose, con `apply`, `up` y `ps`. Con él, el bloque `agentes`, el
catálogo de agentes predefinidos, la experiencia zero-config y el examinador
ciego para python.

### El motor de verdad (1–2 sep, `v0.5.0`–`v0.7.2`)

`v0.5.0` trajo skills reales y ejecución recursiva de subtareas. Justo después,
el commit más incómodo del repo: **el motor nunca invocaba un modelo**. Todo lo
anterior estaba probado contra ejecutores falsos.

De ahí salió una racha de arreglos que solo aparecen cuando la herramienta se usa
de verdad: la entrega conjunta fallaba sin identidad de git configurada; el
presupuesto de líneas lo decide el planificador y no una constante; devclean no
servía en un proyecto existente ni en stacks sin examinador; la esclusa de
interfaces tenía que aceptar archivos como entregable; sin tope, el presupuesto
se declaraba agotado. Terminó en `up`: un solo comando que configura, planifica,
corre y entrega.

### Honestidad de las mediciones (9–14 sep, `v0.8.x`–`v1.0.0`)

Cuatro bugs de la misma familia, todos de "el sistema afirmaba algo que no había
comprobado":

- los escáneres decían "limpio" sin haber mirado;
- la compuerta pintaba el paso equivocado bajo cada etiqueta;
- la portada mostraba pasos de una esclusa que no era la que corre;
- las mediciones mentían y ensuciaban el ledger real de gasto.

Después, lo que faltaba para llamarla 1.0: el latido, que evita que una corrida
muerta deje la tarea atascada para siempre; `--fondo`, para que cerrar la
terminal no se lleve el trabajo; la fricción sacando sus minutos del PR en vez
de devolver null; y el nivel funcional del solapamiento, que atrapa el caso que
nada más atrapa —**dos ramas verdes por separado que rompen al juntarse**—.

`v1.0.0` es del 14 de septiembre de 2026.

### El verde falso (17 sep, `v1.0.1`–`v1.0.3`)

La 1.0 se usó con agentes reales y salieron cinco maneras de entregar algo que
nadie había juzgado. Son las que más forma le dieron al diseño actual:

- `go test ./pkg/...` sobre un paquete sin pruebas **devuelve 0**. Salir con
  cero sin ejecutar pruebas dejó de contar como verde.
- La reversión de alcance no veía lo que el agente commiteaba por su cuenta: con
  el árbol limpio quedaba ciega, y sus propias pruebas llegaban al PR. Ahora se
  mide contra el commit con que arrancó el intento.
- El examen ciego renunciaba en toda tarea de paquete nuevo. Ahora deduce el
  paquete del prefijo de `expone`.
- La suite ciega no compilaba por los imports que el modelo olvidaba declarar.
  Ahora se resuelven contra el módulo y la stdlib; lo que no se resuelve
  descarta la suite, porque es mejor sin examen que con uno roto.
- La suite oculta se quemaba al fallar, así que el siguiente `ship` omitía el
  paso y la tarea frenada salía en un PR con solo repetir el comando. Ahora solo
  se quema al aprobar.

Y una tarea que no se puede examinar era imposible de terminar: la veda de rutas
de prueba se decide por tarea y no por lenguaje, porque solo tiene sentido donde
hay un examen que proteger.

### Requirements as Code, el estado final (18 sep, `v1.1.0`)

La interfaz del producto dejó de ser la tarea y pasó a ser el requerimiento. El
spec humano declara `requirements`, `rules`, `acceptance` y `constraints`; los
contratos de tarea quedan como representación interna que el planificador
genera. Antes de gastar tokens se valida el grafo —ciclos, dependencias
inexistentes, interfaces huérfanas o incompatibles, zonas que se pisan— y al
integrar corren los comandos de aceptación del feature sobre el conjunto, porque
una tarea verde no implica un feature correcto.

Cierra con la restricción del humano sobreviviendo el viaje: `constraints.no_tocar`
se aplica al aplicar el spec, no al leerlo, así que alcanza también a los
contratos que el planificador generó.

---

## 3. Decisiones cerradas

| Decisión | Elección | Por qué |
|---|---|---|
| Lenguaje | Go 1.22+, binario estático único | sin runtime que instalar |
| Ejecución de agentes | envolver los CLI (`claude`, `opencode`) como subprocesos detrás de `Executor` | usa lo que el usuario ya paga |
| Licencia | MIT | |
| Forja | GitHub vía `gh`, con entrega local si no hay remoto | no bloquear a quien trabaja sin origin |
| Distribución | `scripts/install.sh`, releases de GoReleaser y `go install`; sin Homebrew | una fórmula más que mantener |
| Examinador ciego | solo Go y Python | validar sintaxis sin ejecutar exige parsers confiables en stdlib |
| Coordinación entre agentes | ninguna: no debaten ni votan | evita sesgos de conformidad y gasto de tokens; el parte se calcula de los artefactos |
| Verificación | siempre código, nunca modelo | el agente no decide si terminó |

---

## 4. Lo que se fue

- **`internal/executor` versionado**: se sacó del control de versiones dos veces
  en el primer día, hasta que se decidió qué era.
- **La adenda del PRD**: se borró en `704f8b5`. Se recupera con
  `git show 704f8b5^:docs/PRD-adenda.md`.
- **El parser yaml propio para el spec**: `internal/kv` no procesa jerarquías, y
  el spec humano las necesita (`requirements` por categorías, `acceptance` con
  criterio y comando). El spec pasó a `yaml.v3`; `kv` se quedó en el frontmatter
  de contratos y en `config`, que son planos.
- **`parseLegacy`**: 377 líneas del parser anterior del spec, sin un solo
  llamador desde que `Parse` cambió de motor.
- **`ventanas.ErrSonda`**: prometía explicar por qué falló `--sonda` y nadie lo
  devolvía ni lo leía.
- **`docs/PRD-devclean.md`, `docs/ESTADO.md` y `docs/attempts-jsonl.md`**: el
  PRD era la especificación de un producto que entraba por la tarea y no por el
  requerimiento; el traspaso duplicaba lo que `CLAUDE.md` mantiene; y la nota
  del formato abría diciendo "esto no está implementado" sobre algo que el bucle
  escribía desde semanas antes. Este documento recoge lo que valía de los tres.
- **Las referencias a secciones del PRD y de su adenda** que 124 comentarios del
  código arrastraban: apuntaban a documentos que ya no existen, así que cada
  comentario dice ahora lo que quería decir, con palabras.

---

### La costura, cerrada (18 sep, `v1.2.0`)

El último agujero conocido del diseño: una tarea verde no implica un feature
correcto, y nadie probaba la cadena completa. Ahora, cuando el plan tiene varias
tareas encadenadas y el humano no declaró un comando de aceptación, devclean
deriva una tarea final que solo escribe pruebas: depende de todas, consume todo
lo que el plan promete, y su comando queda además como aceptación del feature,
así que se ejercita en su propio cuarto y otra vez sobre el conjunto integrado.

Para que pudiera existir hubo que corregir una asimetría vieja: la veda de rutas
de prueba se decidía por tarea en el bucle pero por proyecto en la esclusa de
entrada, así que la tarea de integración era rechazada antes de correr. Ahora las
dos usan la misma regla.

Con ella, `spec.Marshal` aprendió a escribir `constraints`, que era el último
lugar donde un spec exportado mentía sobre su frontera.

---

## 5. Lo que sigue abierto

Vive en la sección 7 de `CLAUDE.md`, que es lo que se mantiene al día. En
resumen: mutation score para verificar que las suites generadas —y la prueba de
costura derivada— no sean triviales; validación de firmas por AST en vez de por
nombre; modo API directa sin depender de los CLI; más forjas; y la costura sin
probar en los stacks que no tienen un comando de integración conocido.
