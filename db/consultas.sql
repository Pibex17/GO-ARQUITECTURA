-- name: BuscarReserva :one
SELECT id, cancha_id, cliente_id, fecha, hora_inicio, hora_fin, estado
FROM reservas
WHERE id = $1;

-- name: GuardarReserva :execrows
UPDATE reservas
SET cancha_id   = $2,
    cliente_id  = $3,
    fecha       = $4,
    hora_inicio = $5,
    hora_fin    = $6,
    estado      = $7
WHERE id = $1;
