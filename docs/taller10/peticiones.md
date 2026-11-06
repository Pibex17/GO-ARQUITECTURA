# Taller 10 · Línea base (Fase 0)

- **Fecha:** 2026-11-05
- **Rama:** `taller10`
- **Entorno:** PostgreSQL 16 en Docker + servidor local (`go run . -reset`)
- **sqlc:** `v1.31.1`

La columna **Antes** registra el código HTTP que devolvió el servidor en esta
ejecución. La columna **Después** queda pendiente para comparar al final del
taller.

| Petición | Antes | Después |
| --- | --- | --- |
| 1 sin token | 401 | |
| 2 login | 200 | |
| 3 con token | 200 | |
| 4 rol sin permiso | 403 | |
| 5 cambio permitido | 200 | |
| 6 transición prohibida | 422 | |

## Preparación 0

Se ejecutó antes de las seis peticiones:

1. `POST /login` con `luis` (administrador) → `200`; guarda el token de admin.
2. `GET /canchas` con el token de admin → `200`; elige la cancha `1`.
3. `POST /login` con `ana` (cliente) → `200`; guarda el token de cliente.
4. `POST /reservas` con el token de cliente → `201`; crea la reserva `R1`
   (`id 3`) en estado `pendiente`.

## Detalle de las seis peticiones

| # | Petición ejecutada | Código | Código de error |
| --- | --- | --- | --- |
| 1 | `GET /reservas` sin token | `401` | `falta_token` |
| 2 | `POST /login` con `ana` | `200` | — |
| 3 | `GET /reservas` con el token de cliente | `200` | — |
| 4 | `PATCH /reservas/3/estado` con el token de cliente y `{"NuevoEstado":"pago_por_verificar"}` | `403` | `permiso_denegado` |
| 5 | `PATCH /reservas/3/estado` con el token de admin y `{"NuevoEstado":"pago_por_verificar","Metodo":"efectivo","ComprobanteURL":"https://ejemplo.invalid/comprobante.jpg"}` | `200` | — |
| 6 | `PATCH /reservas/4/estado` con el token de admin y `{"NuevoEstado":"confirmada"}` | `422` | `transicion_invalida` |

## Notas

- La petición 5 consume `R1`: la reserva pasa de `pendiente` a
  `pago_por_verificar`. Por eso, antes de la petición 6 se repitió la
  preparación 0 y se creó la reserva `R2` (`id 4`) en estado `pendiente`.
- La petición 4 usa el mismo endpoint que la 5, pero con el rol cliente: la
  autorización por rol responde `403` antes de evaluar el cuerpo o la
  transición.
- La petición 6 intenta `pendiente → confirmada`, transición prohibida por la
  máquina de estados; el servidor responde `422 transicion_invalida`.
- Los seis códigos observados coinciden con los esperados en la tabla del
  taller.
