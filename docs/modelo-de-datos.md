# Modelo de datos

PostgreSQL es la única base. GORM crea las tablas con `AutoMigrate`; los
nombres y columnas son los que se describen abajo. No hay borrados físicos en
los flujos de negocio: se usan estados.

## 1. Diagrama de relaciones

```
usuarios 1 ─── N canchas ─── N horarios
    │                │
    │                │
    │                └── N reservas ─── N pagos
    │                         │
    │                         └── 1 partido ─── N inscripciones
    │                                              │
    └──────────────────────────────────────────────┘

cuentas 1 ─── 0..1 usuarios        (vínculo de sesión)

eventos                 (outbox; no tiene FK)
eventos_procesados      (idempotencia; no tiene FK)
sagas                   (proyección; agregado_id = reserva.id)
```

## 2. Tablas de negocio

### `usuarios`

| Columna | Tipo | Notas |
| --- | --- | --- |
| `id` | `bigserial` | PK. |
| `nombre` | `text` | Nombre visible. |
| `email` | `text` | Contacto. |
| `rol` | `text` | `cliente` o `administrador`. |

### `canchas`

| Columna | Tipo | Notas |
| --- | --- | --- |
| `id` | `bigserial` | PK. |
| `nombre` | `text` | |
| `tipo` | `text` | `futbol_5`, `futbol_7`, `futbol_11`. |
| `direccion` | `text` | |
| `precio_hora` | `numeric` | Base para la seña del 30 %. |
| `activa` | `boolean` | Solo las activas se listan al cliente. |
| `administrador_id` | `bigint` | Dueño; se valida contra `usuarios`. |

### `horarios`

| Columna | Tipo | Notas |
| --- | --- | --- |
| `id` | `bigserial` | PK. |
| `cancha_id` | `bigint` | FK lógica a `canchas`. |
| `dia_semana` | `text` | `lunes` … `domingo`. |
| `hora_inicio` | `text` | `HH:MM`. |
| `hora_fin` | `text` | `HH:MM`. |
| `activo` | `boolean` | |

### `reservas`

| Columna | Tipo | Notas |
| --- | --- | --- |
| `id` | `bigserial` | PK. |
| `cancha_id` | `bigint` | Cancha reservada. |
| `cliente_id` | `bigint` | Usuario con rol cliente. |
| `fecha` | `text` | `YYYY-MM-DD`. |
| `hora_inicio` | `text` | `HH:MM`. |
| `hora_fin` | `text` | `HH:MM`. |
| `estado` | `text` | Ver máquina de estados. |

Máquina de estados:

```
pendiente ──► pago_por_verificar ──► confirmada ──► jugada
    │                 │                   │
    └─────────────────┴───────────────────┴──► cancelada
```

- `jugada` y `cancelada` son terminales.
- `pago_por_verificar` no puede volver a `pendiente`.
- La transición la valida el manejador **y** el consumidor de `reservas`.

### `pagos`

| Columna | Tipo | Notas |
| --- | --- | --- |
| `id` | `bigserial` | PK. |
| `reserva_id` | `bigint` | FK lógica a `reservas`. |
| `tipo` | `text` | `sena` o `saldo`. |
| `monto` | `numeric` | Para `sena`: `precio_hora * 0.30`. |
| `metodo` | `text` | `transferencia`, `efectivo`, `tarjeta`. |
| `comprobante_url` | `text` | Opcional. |
| `estado` | `text` | `pendiente`, `verificado`, `anulado`. |

Regla de negocio: la seña es el **30 %** del precio por hora de la cancha. La
calcula `pagos.NuevoPagoSena`; el manejador HTTP no la calcula.

### `partidos`

| Columna | Tipo | Notas |
| --- | --- | --- |
| `id` | `bigserial` | PK. |
| `reserva_id` | `bigint` | 1 a 1 con `reservas`. |
| `nombre` | `text` | |
| `cupo_maximo` | `integer` | |
| `codigo_invitacion` | `text` | Único por partido (generado por `partidos`). |
| `creado_en` | `timestamptz` | |
| `estado` | `text` | `activo` o `cancelado`. |

`estado` se agregó para compensar sin borrar: al cancelar la reserva, el
partido pasa a `cancelado`.

### `inscripciones`

| Columna | Tipo | Notas |
| --- | --- | --- |
| `id` | `bigserial` | PK. |
| `partido_id` | `bigint` | FK lógica a `partidos`. |
| `jugador_id` | `bigint` | FK lógica a `usuarios`. |
| `creado_en` | `timestamptz` | |

Reglas: no se repite `(partido_id, jugador_id)`; no se supera `cupo_maximo`;
el administrador no se inscribe.

### `cuentas`

| Columna | Tipo | Notas |
| --- | --- | --- |
| `id` | `bigserial` | PK. |
| `usuario` | `text` | Único; nombre de login. |
| `rol` | `text` | `cliente` o `administrador`. |
| `clave_hash` | `text` | bcrypt; nunca la clave. |
| `usuario_id` | `bigint` | Vínculo opcional con `usuarios`. |

## 3. Tablas de la arquitectura de eventos

### `eventos` (outbox)

| Columna | Tipo | Notas |
| --- | --- | --- |
| `id` | `text` (64) | PK; hex de 16 bytes. |
| `tipo` | `text` (64) | Nombre del hecho; indexado. |
| `agregado_id` | `bigint` | ID del agregado (reserva, partido, …). |
| `saga_id` | `text` (64) | `reserva-{id}`; indexado. |
| `correlation_id` | `text` (64) | Traza de la petición original. |
| `causa_id` | `text` (64) | ID del evento que causó este. |
| `payload` | `jsonb` | Datos del hecho. |
| `estado` | `text` (20) | `pendiente`, `publicado`, `fallido`. |
| `intentos` | `integer` | Reintentos de entrega. |
| `error` | `text` | Último error. |
| `ocurrido_en` | `timestamptz` | Orden de despacho. |
| `publicado_en` | `timestamptz` | Cuándo se entregó. |

Índices: `tipo`, `saga_id`, `correlation_id`, `agregado_id`, `estado`.
El despachador consulta `estado = 'pendiente' ORDER BY ocurrido_en`.

### `eventos_procesados` (idempotencia)

| Columna | Tipo | Notas |
| --- | --- | --- |
| `evento_id` | `text` (64) | PK compuesta. |
| `consumidor` | `text` (64) | PK compuesta. |
| `procesado_en` | `timestamptz` | |

La clave `(evento_id, consumidor)` permite que `reservas`, `pagos` y `partidos`
procesen el mismo evento sin pisarse.

### `sagas` (proyección)

| Columna | Tipo | Notas |
| --- | --- | --- |
| `id` | `text` (64) | PK; `reserva-{id}`. |
| `tipo` | `text` (64) | `ciclo_reserva`. |
| `agregado_id` | `bigint` | ID de la reserva. |
| `estado` | `text` (32) | `iniciada`, `en_proceso`, `completada`, `compensando`, `compensada`, `cancelada`, `fallida`. |
| `paso` | `text` (64) | Último hito observado. |
| `ultimo_evento_id` | `text` (64) | Último evento aplicado. |
| `ultimo_evento_en` | `timestamptz` | `ocurrido_en` del último evento aplicado; evita retroceder si un evento viejo se reentrega. |
| `error` | `text` | Motivo del fallo, si aplica. |
| `recurso_id` | `bigint` | Partido creado o agregado. |
| `creada_en` | `timestamptz` | |
| `actualizada_en` | `timestamptz` | |

## 4. Datos de ejemplo

`-reset` siembra:

- Usuarios: El Golazo (administrador), María Pérez (cliente), Juan Torres (cliente).
- Cuentas: `luis` / `luis-2026` (vinculada a El Golazo), `ana` / `ana-2026`
  (vinculada a María Pérez).
- Canchas 1 y 2: activas, precios/hora 25 y 35.
- Reserva 1: `pendiente`.
- Reserva 2: `confirmada`, con seña verificada de 7.50.
- Partido 1: de la reserva 2, activo.
- Inscripciones 1 y 2: en el partido 1.

## 5. Consultas útiles

```sql
-- Reservas con su seña
SELECT r.id, r.estado, p.id AS pago_id, p.monto, p.estado AS pago_estado
FROM reservas r
LEFT JOIN pagos p ON p.reserva_id = r.id
ORDER BY r.id;

-- Eventos pendientes de publicar
SELECT id, tipo, intentos, ocurrido_en
FROM eventos
WHERE estado = 'pendiente'
ORDER BY ocurrido_en;

-- Saga de una reserva
SELECT id, estado, paso, error, recurso_id
FROM sagas
WHERE id = 'reserva-1';

-- Eventos procesados por consumidor
SELECT consumidor, count(*)
FROM eventos_procesados
GROUP BY consumidor;

-- Partidos cancelados por compensación
SELECT id, reserva_id, estado
FROM partidos
WHERE estado = 'cancelado';
```
