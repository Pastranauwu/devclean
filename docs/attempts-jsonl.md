# Formato de `attempts.jsonl` — nota para quien construya `internal/loop`

Esto **no está implementado**. Es la adenda A.2 transcrita al detalle que hace
falta para escribirlo, y nada más. Lo implementa el agente del bucle (§6.4).

Una línea JSON por intento, append-only, en `.devclean/runs/<id>/attempts.jsonl`:

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

Reglas que no se negocian:

- `tests_pasaron` / `tests_fallaron` salen de parsear la salida de
  `listo_cuando`. Si no se puede parsear, van en `null` y solo cuenta
  `salida_codigo`. **No inventar números.**
- `simbolos_exportados` sale del diff contra la rama base: `go/ast` para Go,
  heurística por lenguaje en el resto, `null` si el lenguaje no se soporta.
- `revertidos_fuera_de_alcance` lista los archivos que el bucle revirtió por
  caer fuera de `tocar_solo` o por ser archivos de prueba (adenda A.3).
- Las marcas de tiempo son UTC en RFC 3339.
- Este archivo es la **única** fuente de las métricas y del parte de datos
  (§6.7). Nada se recalcula después leyendo el repo.
