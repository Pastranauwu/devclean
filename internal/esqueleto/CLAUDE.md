# internal/esqueleto: advertencias

- **`Problemas` verifica el plano sin modelo y mira la forma, no si el reparto es bueno.** No corre los `listo_cuando`, salvo `integracion`: si falla hoy, una tarea la tiene como `listo_cuando`; si ya pasa, es regresión y ninguna puede usarla. Un cambio solo visual no lleva tarea final.
- **La marca y `Casos:`/`Idea:` se exigen solo en archivos nuevos respecto de `Verificacion.Base`:** un cambio a un archivo existente exige `Casos:` en `como`. `Idea:` y `Casos:` son literales (`Casos (con mocks):` no pasa).
- **`Idea:` lleva lo pensado por el arquitecto (`contrato`):** algoritmo, estructuras y trampas en 1 a 8 líneas, sin código. Con eso una tarea es un módulo entero. Sin verificar con agentes reales.
- **Un plan con tareas de interfaz en un proyecto web exige `pantallas.script`.**
- **Un ciclo de verdad o un manifiesto que no instala es un problema para el arquitecto, no un aborto:** entra a la lista y se corrige en la siguiente vuelta.
