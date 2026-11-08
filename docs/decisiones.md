# Decisiones de arquitectura (ADR)

Registro de las decisiones que sostienen la migración a Saga Coreografía. Cada
ADR indica contexto, decisión y consecuencias. Las decisiones anteriores están
en [`legacy/decisiones-v1.md`](legacy/decisiones-v1.md) y no aplican al código
actual.

## ADR-001 · Saga Coreografía, no orquestación

**Contexto.** El flujo de reserva involucra tres contextos: reservas, pagos y
partidos. Un orquestador central concentraría el guion y acoplaría los
contextos a un coordinador.

**Decisión.** Cada contexto reacciona a los hechos que le importan y publica
los suyos. No hay un componente que invoque pasos en orden.

**Consecuencias.**

- `reservas` no invoca casos de uso de `pagos` ni de `partidos`; la
  comunicación de escritura es por eventos. Conserva una relación de lectura
  `Reserva.Pagos` para la respuesta HTTP.
- El flujo emerge de las suscripciones; agregar un participante no cambia a
  los demás.
- El flujo completo no está en un solo archivo; hay que leer el catálogo de
  eventos y los consumidores. Se mitiga con
  [`saga-coreografia.md`](saga-coreografia.md).
- La proyección `sagas` es un observador, no un director.

## ADR-002 · Monolito modular en lugar de microservicios

**Contexto.** El equipo es pequeño, el despliegue es académico y no hay
infraestructura de mensajería. Los contextos ya existían como paquetes.

**Decisión.** Un binario, una base PostgreSQL y paquetes con límites claros.

**Consecuencias.**

- Transacciones locales y despliegue simple.
- Los límites se hacen cumplir por revisión y por reglas de importación, no
  por red.
- Migrar un contexto a servicio implica extraer su tabla y su consumidor; el
  contrato de eventos ya está separado.

**Alternativas descartadas.** Microservicios con HTTP síncrono: introducía
latencia, fallos parciales y operación que no se justificaban.

## ADR-003 · Outbox transaccional en PostgreSQL

**Contexto.** Publicar al bus después de guardar el estado deja una ventana de
pérdida si el proceso cae entre ambos pasos.

**Decisión.** El caso de uso escribe el cambio y el evento en la misma
transacción (`eventos`). El despachador publica después.

**Consecuencias.**

- Garantía at-least-once y posibilidad de reintento.
- Latencia de hasta 100 ms entre la respuesta `202` y el primer paso.
- La tabla crece; requiere retención (véase [`operacion.md`](operacion.md)).
- Los consumidores deben ser idempotentes.

## ADR-004 · Bus en memoria detrás de una interfaz

**Contexto.** No se quiere operar Kafka/RabbitMQ, pero tampoco se quiere que
el dominio dependa del bus concreto.

**Decisión.** `eventos.Bus` con `Publicar` y `Suscribir`; implementación
`BusEnMemoria` para el monolito.

**Consecuencias.**

- Cambiar a un broker no toca a los contextos.
- El bus no persiste: la durabilidad la da el outbox.
- El bus actual no escala entre procesos; es una limitación consciente.

## ADR-005 · Idempotencia por `(evento_id, consumidor)`

**Contexto.** El despachador puede reentregar un evento tras una caída o un
error parcial.

**Decisión.** `ProcesadorGORM.Procesar` inserta la clave en
`eventos_procesados` dentro de la misma transacción que el efecto. Si la clave
existe, no ejecuta el efecto.

**Consecuencias.**

- Cada consumidor tiene su propia marca; el mismo evento puede ser procesado
  por `reservas`, `pagos` y `partidos`.
- La tabla también crece; se puede purgar junto con el outbox.
- Un efecto con llamadas externas no transaccionales requeriría una estrategia
  adicional (clave de idempotencia hacia el proveedor).

## ADR-006 · API asíncrona con `202` y `GET /sagas/{id}`

**Contexto.** Si el `PATCH` esperara a que terminen todos los pasos, volvería
a ser síncrono y acoplaría la latencia del cliente a la de la saga.

**Decisión.** Validar forma, rol, propiedad y transición en el request; si es
válido, publicar el hecho y responder `202` con `saga_id` y `Location`. El
resultado final se consulta en la proyección.

**Consecuencias.**

- Errores síncronos claros (`400/401/403/404/409/422`).
- Fallos de negocio posteriores aparecen en la saga como `fallida`.
- El cliente debe hacer polling; no hay webhooks ni SSE en esta versión.
- El `201` de creación de reserva se mantiene porque la reserva sí queda
  creada en el request; el ciclo posterior es asíncrono.

## ADR-007 · ID de saga determinista

**Contexto.** El manejador y la proyección podrían intentar crear la misma
saga al mismo tiempo.

**Decisión.** `sagas.IDDeReserva(id)` devuelve `reserva-{id}`;
`Coordinador.Asegurar` usa `ON CONFLICT DO NOTHING`.

**Consecuencias.**

- No hay carrera por la creación.
- El cliente puede construir la URL de la saga sin consultar la base.
- Solo hay una saga por reserva; para nuevos ciclos habría que versionar el
  ID.

## ADR-008 · Separar `pagos` de `reservas`

**Contexto.** El paquete `reservas` era dueño de `Reserva`, `Pago` y
`NuevoPagoSena`, y el handler hacía escrituras de ambos.

**Decisión.** `internal/pagos` es dueño de `Pago`, `NuevoPagoSena`, su
repositorio y sus consumidores. `reservas` conserva una relación de lectura
`Reserva.Pagos` para `GET /reservas/{id}`.

**Consecuencias.**

- Cada contexto tiene una razón de cambio.
- La creación de la seña ya no ocurre en el handler de reservas.
- Hay una dependencia de lectura de `reservas` hacia el modelo `pagos`; es
  deliberada y solo para la respuesta HTTP.

## ADR-009 · Compensar con estados, no borrar

**Contexto.** Una cancelación debe deshacer la seña y el partido sin perder
historia ni romper referencias.

**Decisión.** `Pago.estado = anulado`, `Partido.estado = cancelado`,
`Reserva.estado = cancelada`. No se ejecutan `DELETE`.

**Consecuencias.**

- Auditoría completa y claves foráneas estables.
- Las consultas deben filtrar por estado cuando corresponda.
- `Partido` ganó una columna `estado` que antes no existía.

## ADR-010 · Documentación nueva; la anterior pasa a `legacy`

**Contexto.** La documentación de Hito 1/Hito 2 describía la API síncrona y
contradecía al código nuevo.

**Decisión.** Mover la documentación anterior a `docs/legacy/` con `git mv` y
escribir un conjunto nuevo. `docs/legacy/README.md` declara que no es
autoritativa.

**Consecuencias.**

- Se preserva el historial sin confundir a quien implementa.
- El contrato vigente es `docs/openapi.yaml`.
- Cualquier ejemplo de `legacy/` que use `200` para un cambio de estado está
  obsoleto.

## ADR-011 · Validación síncrona mínima antes del `202`

**Contexto.** Aceptar cualquier petición y fallar después en la saga da una
mala experiencia para errores evidentes (rol, propiedad, recurso inexistente).

**Decisión.** El manejador valida lo que puede saber sin ejecutar la saga:
forma, token, rol, propiedad, existencia y transición. La decisión de negocio
definitiva se revalida en el consumidor.

**Consecuencias.**

- Menos sagas fallidas por errores de uso.
- Alguna lógica se repite (por ejemplo, dueño de la reserva en `partidos` y en
  `reservas`); es una duplicación aceptada para dar errores rápidos.
- La fuente de verdad sigue siendo el consumidor, que puede rechazar incluso
  si el manejador aceptó (carrera entre validación y ejecución).

## ADR-012 · Reintentos acotados y fallo visible

**Contexto.** Un reintento infinito puede bloquear el outbox y ocultar un
error permanente.

**Decisión.** Máximo 5 intentos por evento. Al agotarlos, se marca `fallido`
y se publica `ProcesamientoFallido`; la saga queda `fallida`.

**Consecuencias.**

- Un fallo permanente no se reintenta para siempre.
- Requiere intervención manual (volver el evento a `pendiente` tras
  corregir la causa).
- `ProcesamientoFallido` también es un evento; si su consumidor falla, no se
  reintenta indefinidamente.
