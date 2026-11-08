# Colección Bruno · API asíncrona

Abre esta carpeta con Bruno (`Open Collection`) y selecciona el entorno
`Local` en `environments/Local.bru`.

## Orden de ejecución

1. `01 Login cliente` → guarda `tokenCliente`.
2. `02 Login administrador` → guarda `tokenAdmin`.
3. `03 Canchas/Listar canchas`.
4. `04 Reservas/Crear reserva` → guarda `reservaId` y `sagaId`.
5. `04 Reservas/Declarar seña` (admin) → `202`.
6. `04 Reservas/Consultar saga` → repite hasta ver `pago_por_verificar`.
7. `04 Reservas/Confirmar reserva` (admin) → `202`.
8. `04 Reservas/Consultar saga` → repite hasta ver `reserva_confirmada`.
9. `05 Partidos/Crear partido` (cliente) → `202` sobre la misma saga.
10. `04 Reservas/Consultar saga` → repite hasta `completada`; guarda
    `partidoId` desde `recurso_id`.
11. `05 Partidos/Ver partido`.
12. `05 Partidos/Inscribirse en partido` → usa el `CodigoInvitacion` real del
    paso anterior (edítelo en el cuerpo).
13. `04 Reservas/Cancelar reserva` (admin) → `202`; consulta la saga hasta
    `cancelada` y verifica que la seña quedó `anulado` y el partido
    `cancelado`.

## Notas

- Los scripts de respuesta leen `res.body.datos` con la envoltura `ok/datos`.
- `partidoId` se llena desde `recurso_id` de la saga; el `POST /partidos` no
  devuelve el ID porque responde `202` antes de crear el partido.
- Si un paso devuelve `422`, revise el estado actual de la saga y de la
  reserva; la transición puede no ser válida todavía.
- La colección anterior de la API síncrona está en
  [`../legacy/hito2_bruno/`](../legacy/hito2_bruno/) y no aplica.
