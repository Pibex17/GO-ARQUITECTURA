# Documentación histórica (no autoritativa)

Esta carpeta conserva la documentación de las etapas anteriores del proyecto
(Hito 1, Hito 2, contrato síncrono y decisiones ya superadas). Se mueve aquí
con `git mv` para preservar el historial, no para mantenerla vigente.

**La fuente de verdad actual es la documentación de la raíz y de `docs/`:**

- [`../../README.md`](../../README.md)
- [`../arquitectura.md`](../arquitectura.md)
- [`../saga-coreografia.md`](../saga-coreografia.md)
- [`../modelo-de-datos.md`](../modelo-de-datos.md)
- [`../api.md`](../api.md)
- [`../openapi.yaml`](../openapi.yaml)
- [`../operacion.md`](../operacion.md)
- [`../decisiones.md`](../decisiones.md)

Nada de lo que está en `docs/legacy/` debe usarse para implementar, probar o
evaluar el sistema actual. En particular, el contrato `openapi-v1.yaml` y la
hoja `hito2_hoja.md` describen la API síncrona anterior, que ya no existe.

## Contenido

| Archivo | Qué era |
| --- | --- |
| `hito1_ficha_del_negocio.md` | Ficha original del negocio. |
| `hito1_addendum.md` | Addendum técnico de la primera etapa. |
| `hito1_*.png`, `hito1_presentacion.pdf` | Capturas y presentación de la primera etapa. |
| `hito2_hoja.md` | Guion de demostración de la API síncrona. |
| `hito2_bruno/` | Colección Bruno y capturas de la API síncrona. |
| `arquitectura-v1.md` | Arquitectura del monolito síncrono. |
| `arquitectura_capas.md` | Descripción por capas anterior. |
| `decisiones-v1.md` | Decisiones anteriores a la Saga Coreografía. |
| `openapi-v1.yaml` | Contrato OpenAPI de la API síncrona. |
| `api-v1.html` | Documentación HTML generada del contrato anterior. |
