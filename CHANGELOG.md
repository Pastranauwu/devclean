# Changelog

Qué trajo cada versión, en una línea. Lo anterior a v1.2.0 está contado en `docs/HISTORIA.md`.

## v1.11.4 (8 oct 2026)
Una entrega conjunta cortada (Ctrl+C, kill) ya no deja las ramas de los cuartos aplanadas: sus puntas se guardan en `.devclean/puntas.json` y el `ship --todas` o `run` siguiente las devuelve (`RestaurarPuntas`); el fallo del conjunto nombra la prueba que falló y `reparar` encuentra sola a la tarea que la escribió, con unittest/Django, pytest y jest/vitest; ningún git de la entrega pregunta credenciales, tampoco por el askpass de un IDE, y el `fetch` tiene tope de tiempo; el paso `ruido` deja imprimir al archivo que declara un `main` aunque no viva en `cmd/` o `scripts/`.

## v1.11.3 (8 oct 2026)
Docker solo se exige en proyectos generados desde cero (sobre código existente se pide con "con docker"); entrega local aunque haya remoto (`--local` en `ship` y `up`, o `entrega: local` en `config.yml`); cuartos y ramas de otra corrida se descartan en vez de reusarse (`soltarSinRastro`); `run` se frena si el trabajo verde del que depende una tarea ya no existe; aviso cuando la rama actual no es la base; la respuesta del arquitecto con prosa antes del JSON ya no cuesta una ronda de corrección (`plan.CuerpoJSON`); Django detecta `python manage.py test`; con claude, la corrección del arquitecto continúa su sesión en vez de releer el repo (`--resume`) y `tope_arquitecto_usd` corta cada invocación (`--max-budget-usd`); los cuartos heredan el entorno de Python del repo y del cuarto del que parten en vez de reinstalar de cero; la suite se corre en la base antes de planear y, si ya falla, se avisa; lo que un agente arregla fuera de su alcance se revierte pero queda como parche aplicable; la base por defecto es la rama actual, no `main`; `claude-sonnet-5-5` y `claude-haiku-5-5` en el catálogo de claude y como preferidos; los `.env*` que nombran un ambiente real (`prod`, `production`, `staging`, `stage`, `live`) no se copian a los cuartos; un cuarto reusado cuyo rebase choca en commits intermedios se aplana sobre la base nueva, y si no puede, avisa; con entrega local la esclusa no toca la red y el `fetch` nunca pregunta credenciales; la rama de entrega hereda el entorno de Python de los cuartos que integra.

## v1.11.2 (1 oct 2026)
`arquitecto_economico` arranca cambios incrementales con el modelo medio; el parser de contratos tolera campos desconocidos (aviso en lugar de fallo); modificadores y genéricos de TypeScript en la verificación de firmas (`NombreDeFirma`, `FirmaVerificable`).

## v1.11.1
Libera los cuartos al archivar (`devclean limpiar`) y rechaza la prueba del agente que pasa sin su implementación.

## v1.11.0
Examinador ciego conectado a las tareas del esqueleto y extendido a TypeScript/JavaScript (y a Python en monorepos); `devclean reparar`; la entrega ya no reescribe las ramas de los cuartos.

## v1.10.0
Codex como tercer CLI; tablero que cabe en la terminal (filtro, detalle, entrega conjunta); `devclean archive` (historial tipo ADR de los features probados); arreglos de entrega: dry-run que reescribía la rama, "ya pasa" falso y pruebas viejas que nadie podía actualizar.

## v1.9.0
Cuartos fuera del árbol del proyecto (`room.Dir`); lo pensado por el arquitecto viaja como `Idea:` en cada stub (una tarea es un módulo entero); menos contexto por turno: prompt de sistema propio para claude y lista de archivos al final del prompt.

## v1.8.0
Mantenimiento incremental por spec (un `up` sobre el mismo spec no replanea; uno editado solo manda lo que cambió, `spec.IntencionFile`; ids de requerimiento con `cubre`); evolución de código existente con docker compose; revisión visual con capturas de flujos.

## v1.7.0
Esqueleto plano (cada tarea escribe su prueba desde `Casos:`); presupuesto que cuenta la caché; modelos gratis de opencode mezclados con claude; tope de 3 agentes pagados; escalera sin opus.

## v1.1.0 (18 sep 2026)
Requirements as Code: `devclean.spec.yml` declara requirements, reglas, aceptación y restricciones; los contratos de tarea quedan como IR; el grafo se valida antes de ejecutar y la aceptación global corre sobre el conjunto integrado.
