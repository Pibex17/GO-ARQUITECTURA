# Taller 10 · Comparación antes/después (Fase 4)

- **Fecha:** 2026-11-05
- **Rama:** `taller10`
- **Antes:** `docs/taller10/peticiones.md` (Fase 0, 2026-11-05)
- **Después:** `docs/taller10/peticiones_despues.md` (Fase 4)
- **Entorno después:** PostgreSQL 16 en Docker + servidor local con
  `ADAPTADOR_RESERVAS=sqlc`
- **Pruebas:** `docs/taller10/go_test.txt`

## Tabla comparativa principal

La tabla sigue la variante documentada literalmente en `peticiones.md`.

| # | Petición | Código antes | Código después | Resultado |
| --- | --- | --- | --- | --- |
| 1 | `GET /reservas` sin token | `401` | `401` | Conservado. Sigue devolviendo `falta_token`. |
| 2 | `POST /login` con `ana` | `200` | `200` | Conservado. Devuelve JWT de cliente. |
| 3 | `GET /reservas` con token de cliente | `200` | `200` | Conservado. Devuelve las reservas del cliente. |
| 4 | `PATCH /reservas/3/estado` con token de cliente | `403` | `403` | Conservado. Sigue devolviendo `permiso_denegado`. |
| 5 | `PATCH /reservas/3/estado` con token de admin | `200` | `200` | Conservado. La reserva pasa a `pago_por_verificar` y se registra la seña. |
| 6 | `PATCH /reservas/4/estado` con token de admin y transición inválida | `422` | `422` | Conservado. Sigue devolviendo `transicion_invalida`. |

## Variante adicional de las peticiones 4–5

El mensaje de la tarea y la colección Bruno describen las peticiones 4 y 5
sobre `PATCH /canchas/1`. También se ejecutaron para verificar que esa variante
conserva su comportamiento:

| # | Petición | Código antes | Código después | Resultado |
| --- | --- | --- | --- | --- |
| 4b | `PATCH /canchas/1` con token de cliente y `{}` | `403` | `403` | Conservado. Sigue devolviendo `permiso_denegado`. |
| 5b | `PATCH /canchas/1` con token de admin y `{}` | `200` | `200` | Conservado. El administrador recibe la cancha. |
| 6b | `PATCH /reservas/4/estado` con token de admin y `{"NuevoEstado":"confirmada"}` | `422` | `422` | Conservado. Sigue devolviendo `transicion_invalida`. |

## ¿Se conservaron los seis códigos HTTP?

**Sí.** Las dos variantes produjeron exactamente la secuencia esperada:

```text
401, 200, 200, 403, 200, 422
```

Ninguna petición devolvió un código distinto del documentado en la Fase 0.

## ¿Se conservaron los comportamientos funcionales?

**Sí.**

- La ruta protegida sin token continúa rechazando con `401 falta_token`.
- El login de `ana` continúa devolviendo `200` y un JWT válido.
- El cliente continúa consultando únicamente sus reservas con `200`.
- El rol cliente continúa recibiendo `403 permiso_denegado` antes de evaluar
  el cuerpo o la transición.
- El administrador continúa cambiando el estado de una reserva propia con
  `200`; en la petición 5 se creó la seña de `7.5` con método `efectivo`.
- La transición `pendiente → confirmada` continúa rechazándose con
  `422 transicion_invalida`.
- La variante de canchas también conserva `403` para cliente y `200` para
  administrador.

## Resultado de `go test ./...`

El comando se ejecutó con salida completa en `docs/taller10/go_test.txt`.

```text
?   	canchas-sinteticas	[no test files]
?   	canchas-sinteticas/internal/almacen	[no test files]
?   	canchas-sinteticas/internal/almacen/filas	[no test files]
ok  	canchas-sinteticas/internal/canchas	1.830s
ok  	canchas-sinteticas/internal/config	(cached)
ok  	canchas-sinteticas/internal/inscripcion	1.893s
?   	canchas-sinteticas/internal/middleware	[no test files]
ok  	canchas-sinteticas/internal/partidos	2.666s
ok  	canchas-sinteticas/internal/reservas	2.022s
?   	canchas-sinteticas/internal/respuesta	[no test files]
ok  	canchas-sinteticas/internal/sesion	2.567s
?   	canchas-sinteticas/internal/usuarios	[no test files]
```

**Resultado:** exit code `0`; no apareció ningún `FAIL`.

## Diferencias encontradas

1. **Diferencia documental, no funcional:** `docs/taller10/peticiones.md`
   describe las peticiones 4 y 5 como `PATCH /reservas/{id}/estado`, mientras
   que el mensaje de la tarea y la colección Bruno las describen como
   `PATCH /canchas/1`. Se ejecutaron ambas variantes y ambas conservaron los
   códigos esperados. No se modificó ninguna de las dos evidencias.
2. **Incidente de herramienta:** el primer script de ejecución completó todas
   las llamadas HTTP, pero se quedó colgado al serializar el resumen final. Los
   códigos y cuerpos se recuperaron de las respuestas guardadas y del log del
   servidor; no se perdió ni se inventó evidencia.
3. **ID de las reservas:** se mantuvieron los IDs `3` y `4` porque la base se
   reinició con `-reset` antes de la preparación, igual que en la Fase 0.

## Conclusión

La API conserva el comportamiento anterior de las seis peticiones de la Fase 0.
Los códigos HTTP, los códigos de error y los efectos funcionales coinciden con
la línea base. La integración del adaptador sqlc y los cambios posteriores no
introdujeron regresiones detectables en este recorrido.

La Fase 4 queda demostrada con ejecución real de las peticiones y con
`go test ./...` sin `FAIL`. No se realizaron cambios de código, commits, push,
merge ni rebase.
