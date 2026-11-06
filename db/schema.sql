-- Esquema de referencia para sqlc. No se ejecuta contra la base:
-- las tablas las sigue creando AutoMigrate de GORM.
--
-- Se declaran NOT NULL las columnas que el negocio siempre llena, aunque
-- GORM las haya creado como NULL en PostgreSQL. Así sqlc genera string e
-- int64 en lugar de sql.NullString/sql.NullInt64.
CREATE TABLE reservas (
    id          bigserial PRIMARY KEY,
    cancha_id   bigint NOT NULL,
    cliente_id  bigint NOT NULL,
    fecha       text NOT NULL,
    hora_inicio text NOT NULL,
    hora_fin    text NOT NULL,
    estado      text NOT NULL
);

CREATE TABLE pagos (
    id              bigserial PRIMARY KEY,
    reserva_id      bigint NOT NULL REFERENCES reservas(id),
    tipo            text NOT NULL,
    monto           numeric NOT NULL CHECK (monto > 0),
    metodo          text NOT NULL,
    comprobante_url text NOT NULL,
    estado          text NOT NULL
);
