# Saga Coreografía

## 1. Qué es y qué no es

Una **saga** es una transacción de negocio distribuida en pasos locales. En
**coreografía**, cada participante reacciona a los hechos de los demás y
publica los suyos; no hay un coordinador que dicte el siguiente paso.

- **No hay orquestador.** No existe un `SagaOrquestador` que llame a
  `pagos.Crear()` y luego a `reservas.Confirmar()`.
- **Sí hay una proyección.** `sagas` escucha todos los eventos para que el
  cliente pueda consultar `GET /sagas/{id}`. Es un observador, no un director.
- **Cada contexto decide.** `reservas` decide si una seña verificada permite
  confirmar; `pagos` decide cómo verificar; `partidos` decide si puede crear el
  partido.

## 2. Ciclo de vida

```
ReservaCreada
    │
    ▼
SenaDeclarada ──► SenaRegistrada ──► ReservaPasoAPagoPorVerificar
                                            │
                                            ▼
                                      SenaAprobada
                                            │
                                            ▼
                                      SenaVerificada
                                            │
                                            ▼
                                      ReservaConfirmada
                                            │
                    ┌───────────────────────┴───────────────────────┐
                    ▼                                               ▼
             SolicitudPartidoRecibida                        JugadaSolicitada
                    │                                               │
                    ▼                                               ▼
          ReservaValidadaParaPartido                         ReservaJugada
                    │                                         (completada)
                    ▼
              PartidoCreado
              (completada)

Cualquier paso puede derivar en:
    RegistroSenaFallido / VerificacionSenaFallida / ReservaNoActualizada /
    ReservaNoConfirmada / PartidoFallido / ProcesamientoFallido
    → saga fallida o compensada
```

La reserva conserva su máquina de estados:

```
pendiente ──► pago_por_verificar ──► confirmada ──► jugada
    │                 │                   │
    └─────────────────┴───────────────────┴──► cancelada
```

Estados terminales: `jugada` y `cancelada`. No se borra ninguna fila.

## 3. Catálogo de eventos

Todos los eventos comparten:

```json
{
  "id": "hex-32",
  "tipo": "SenaDeclarada",
  "agregado_id": 1,
  "saga_id": "reserva-1",
  "correlation_id": "hex-32",
  "causa_id": "hex-32 o vacío",
  "payload": { },
  "estado": "pendiente",
  "intentos": 0,
  "ocurrido_en": "2026-11-08T12:00:00Z"
}
```

| Evento | Productor | Consumidores | Payload | Efecto |
| --- | --- | --- | --- | --- |
| `ReservaCreada` | `reservas` (POST) | `sagas` | `reserva_id`, `cancha_id`, `cliente_id` | Abre la saga `reserva-{id}`. |
| `SenaDeclarada` | `reservas` (PATCH) | `pagos` | `reserva_id`, `cancha_id`, `precio_hora`, `metodo`, `comprobante_url` | `pagos` crea la seña del 30 %. |
| `SenaRegistrada` | `pagos` | `reservas` | `reserva_id`, `pago_id`, `monto` | `reservas` pasa a `pago_por_verificar`. |
| `RegistroSenaFallido` | `pagos` | `sagas` | `reserva_id`, `motivo` | Saga `fallida`. |
| `ReservaPasoAPagoPorVerificar` | `reservas` | `sagas` | `reserva_id` | Proyección: paso `pago_por_verificar`. |
| `SenaAprobada` | `reservas` (PATCH) | `pagos` | `reserva_id` | `pagos` verifica la seña. |
| `SenaVerificada` | `pagos` | `reservas` | `reserva_id`, `pago_id` | `reservas` pasa a `confirmada`. |
| `VerificacionSenaFallida` | `pagos` | `sagas` | `reserva_id`, `motivo` | Saga `fallida`. |
| `ReservaConfirmada` | `reservas` | `sagas` | `reserva_id` | Proyección: paso `reserva_confirmada`. |
| `ReservaNoActualizada` | `reservas` | `pagos`, `sagas` | `reserva_id`, `motivo` | `pagos` anula la seña; saga `compensando`. |
| `ReservaNoConfirmada` | `reservas` | `pagos`, `sagas` | `reserva_id`, `motivo` | `pagos` revierte la verificación. |
| `PagoAnulado` | `pagos` | `sagas` | `reserva_id`, `pago_id` | Proyección: compensación aplicada. |
| `VerificacionRevertida` | `pagos` | `sagas` | `reserva_id`, `pago_id` | Proyección: verificación revertida. |
| `CancelacionSolicitada` | `reservas` (PATCH) | `reservas` | `reserva_id` | `reservas` pasa a `cancelada`. |
| `ReservaCancelada` | `reservas` | `pagos`, `partidos`, `sagas` | `reserva_id` | Anula seña y cancela partido. |
| `JugadaSolicitada` | `reservas` (PATCH) | `reservas` | `reserva_id` | `reservas` pasa a `jugada`. |
| `ReservaJugada` | `reservas` | `sagas` | `reserva_id` | Saga `completada`. |
| `SolicitudPartidoRecibida` | `partidos` (POST) | `reservas` | `reserva_id`, `cliente_id`, `nombre`, `cupo_maximo` | `reservas` revalida. |
| `ReservaValidadaParaPartido` | `reservas` | `partidos` | igual que la solicitud | `partidos` crea el partido. |
| `PartidoCreado` | `partidos` | `sagas` | `reserva_id`, `partido_id` | Saga `completada`, `recurso_id`. |
| `PartidoFallido` | `reservas` | `sagas` | `reserva_id`, `motivo` | Saga `fallida`. |
| `PartidoCancelado` | `partidos` | `sagas` | `reserva_id`, `partido_id` | Proyección: partido cancelado. |
| `ProcesamientoFallido` | despachador | `pagos`, `sagas` | `evento_id`, `tipo_evento`, `agregado_id`, `saga_id`, `motivo` | Saga `fallida` y compensación si aplica. |

## 4. Pasos de la saga

### 4.1 Nacer: `POST /reservas`

1. El manejador valida cancha activa, fechas, conflicto y propiedad.
2. En una transacción guarda `Reserva(estado=pendiente)` y `ReservaCreada`.
3. Devuelve `201` con la reserva. La saga `reserva-{id}` se crea por la
   proyección (o por `Asegurar` en el primer PATCH).

### 4.2 Declarar la seña: `PATCH ... {"NuevoEstado":"pago_por_verificar"}`

1. El manejador valida rol administrador, dueño de la cancha y transición
   `pendiente → pago_por_verificar`.
2. Asegura la saga y publica `SenaDeclarada` con el precio/hora de la cancha.
3. Devuelve `202`.
4. `pagos` crea `Pago(tipo=sena, monto=precio_hora*0,30, estado=pendiente)` y
   publica `SenaRegistrada`.
5. `reservas` pasa la reserva a `pago_por_verificar` y publica
   `ReservaPasoAPagoPorVerificar`.

### 4.3 Aprobar la seña: `PATCH ... {"NuevoEstado":"confirmada"}`

1. El manejador valida `pago_por_verificar → confirmada` y publica
   `SenaAprobada`.
2. Devuelve `202`.
3. `pagos` marca la seña `verificado` y publica `SenaVerificada`.
4. `reservas` pasa la reserva a `confirmada` y publica `ReservaConfirmada`.

### 4.4 Cerrar la reserva: `PATCH ... {"NuevoEstado":"jugada"}`

1. El manejador valida `confirmada → jugada` y publica `JugadaSolicitada`.
2. Devuelve `202`.
3. `reservas` pasa a `jugada` y publica `ReservaJugada`.
4. La saga queda `completada`.

### 4.5 Crear el partido: `POST /partidos`

1. El manejador hace una lectura de solo lectura de la reserva: debe existir,
   pertenecer al cliente y estar `confirmada`; además no debe existir un
   partido para esa reserva. Los errores son `403/404/409/422`.
2. Asegura la saga y publica `SolicitudPartidoRecibida`. Devuelve `202`.
3. `reservas` revalida y publica `ReservaValidadaParaPartido`.
4. `partidos` crea `Partido(estado=activo)` con código de invitación y publica
   `PartidoCreado`.
5. La saga queda `completada` con `recurso_id = partido_id`.

### 4.6 Cancelar: `PATCH ... {"NuevoEstado":"cancelada"}`

1. El manejador valida que la reserva no esté en un estado terminal y publica
   `CancelacionSolicitada`. Devuelve `202`.
2. `reservas` pasa a `cancelada` y publica `ReservaCancelada`.
3. `pagos` anula la seña (`anulado`) y publica `PagoAnulado`.
4. `partidos` cancela el partido (`cancelado`) y publica `PartidoCancelado`.
5. La saga conserva `estado=cancelada` y refleja las compensaciones en `paso`.

## 5. Compensaciones

| Paso que falla | Evento de fallo | Compensación |
| --- | --- | --- |
| Crear seña | `RegistroSenaFallido` | Saga `fallida`; no hay seña que anular. |
| Guardar `pago_por_verificar` | `ReservaNoActualizada` | `pagos` anula la seña; `PagoAnulado`. |
| Verificar seña | `VerificacionSenaFallida` | Saga `fallida`; la seña queda como estaba. |
| Guardar `confirmada` | `ReservaNoConfirmada` | `pagos` revierte la verificación; `VerificacionRevertida`. |
| Crear partido | `PartidoFallido` | Saga `fallida`; no hay partido que borrar. |
| Consumidor agota reintentos | `ProcesamientoFallido` | `pagos` compensa si el evento era `SenaRegistrada` o `SenaVerificada`. |

Las compensaciones son **semánticas**: cambian estados, no borran filas. Un
`Pago` anulado sigue existiendo para auditoría; un `Partido` cancelado también.

## 6. Idempotencia y reintentos

- El despachador entrega cada evento al bus. Si un consumidor devuelve error,
  el evento se marca con un intento más y sigue `pendiente`.
- `ProcesadorGORM.Procesar` inserta la clave `(evento_id, consumidor)` con
  `ON CONFLICT DO NOTHING` en la misma transacción que el efecto. Si la clave
  ya existe, el efecto no se repite.
- Los eventos derivados se guardan en la misma transacción que el efecto; si
  la transacción se revierte, no queda ni el efecto ni el evento derivado.
- Los eventos llevan una marca `ocurrido_en` estrictamente creciente dentro
  del proceso. La proyección de `sagas` guarda `ultimo_evento_en` y descarta
  cualquier evento anterior; así una reentrega vieja no hace retroceder la
  saga.
- Las compensaciones usan IDs deterministas para que un reintento no cree
  duplicados.
- Tras 5 intentos el evento pasa a `fallido` y se publica
  `ProcesamientoFallido`; la saga queda `fallida` para intervención.

## 7. Estados de la proyección

| Estado | Significado |
| --- | --- |
| `iniciada` | La reserva existe; la saga se abrió. |
| `en_proceso` | Hay un paso en curso o completado sin llegar al final. |
| `completada` | Se creó el partido o la reserva quedó `jugada`. |
| `compensando` | Se solicitó una cancelación o se detectó un fallo compensable. |
| `compensada` | La compensación terminó (por ejemplo, seña anulada). |
| `cancelada` | La reserva quedó cancelada; las compensaciones no borran este estado. |
| `fallida` | Un paso falló y no hay compensación automática posible. |

`paso` es el último hito observado (`reserva_creada`, `sena_declarada`,
`sena_registrada`, `pago_por_verificar`, `sena_aprobada`, `sena_verificada`,
`reserva_confirmada`, `solicitud_partido`, `reserva_validada`,
`partido_creado`, `cancelacion_solicitada`, `reserva_cancelada`,
`pago_anulado`, `partido_cancelado`, `jugada`, …).

## 8. Agregar un participante nuevo

1. Defina el hecho que le interesa y el hecho que publica en
   `internal/eventos/payloads.go`.
2. Cree un paquete con su repositorio y un `Sagas` que se suscriba al bus.
3. Use `eventos.Procesador` para que el efecto sea idempotente y guarde sus
   eventos derivados en la misma transacción.
4. Regístrelo en `construirDependencias` de `main.go`.
5. Agregue el mapeo del evento a `internal/sagas/proyeccion.go` si debe verse
   en `GET /sagas/{id}`.
6. Documente el evento y la compensación en este archivo y en
   `docs/openapi.yaml` si afecta la API.
