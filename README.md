# ServA · Canchas Sintéticas

Servidor HTTP en Go (Chi + GORM/PostgreSQL) para reservar canchas sintéticas,
cobrar la seña y organizar partidos. La versión actual migra el flujo síncrono
a una **Saga Coreografía** con eventos de dominio, outbox transaccional y API
asíncrona (`202 Accepted` + `GET /sagas/{id}`).

## Qué cambió

| Antes (API síncrona) | Ahora (Saga Coreografía) |
| --- | --- |
| `PATCH /reservas/{id}/estado` hacía todo en un handler: creaba el pago, lo verificaba y guardaba la reserva. | El handler valida y publica un hecho (`SenaDeclarada`, `SenaAprobada`, …). Cada contexto reacciona y publica el siguiente hecho. |
| La respuesta era `200` con la reserva ya mutada. | La respuesta es `202 Accepted` con `saga_id` y `Location: /sagas/{id}`. El cliente consulta la saga. |
| `POST /partidos` creaba el partido en el mismo request. | El handler publica `SolicitudPartidoRecibida`; `partidos` crea el partido cuando `reservas` valida. |
| Cancelar solo cambiaba el estado de la reserva. | Cancelar dispara compensaciones: se anula la seña y se cancela el partido. |
| Un paquete `reservas` era dueño de reservas y pagos. | `reservas` y `pagos` son contextos separados; se comunican por eventos. |

La API anterior y su documentación quedaron en [`docs/legacy/`](docs/legacy/)
como historia, **no** como fuente de verdad.

## Arquitectura en una página

- **Monolito modular**: un solo binario y una sola base PostgreSQL, con
  paquetes que representan contextos delimitados.
- **Bus de eventos en proceso**: `internal/eventos` entrega eventos a los
  suscriptores registrados. No hay Kafka ni RabbitMQ.
- **Outbox transaccional**: el cambio de estado y el evento se guardan en la
  misma transacción (`tabla eventos`). Un despachador los publica después.
- **Idempotencia**: cada consumidor registra `(evento_id, consumidor)` en
  `eventos_procesados`; si el evento se reintenta, no se repite el efecto.
- **Coreografía**: no hay orquestador central. `reservas`, `pagos` y
  `partidos` reaccionan a hechos y publican los propios. `sagas` solo proyecta
  y audita.
- **Compensación**: los estados terminales son `cancelada` (reserva),
  `anulado` (pago) y `cancelado` (partido). Nunca se borran filas.

```
Cliente HTTP
    │  POST/PATCH
    ▼
Manejador (validación síncrona: forma, rol, propiedad, transición)
    │  inserta evento en outbox (misma transacción que el cambio)
    ▼
eventos (outbox) ──► Despachador ──► Bus en memoria
                                      ├── reservas
                                      ├── pagos
                                      ├── partidos
                                      └── sagas (proyección)
```

Diagramas y recorridos completos: [`docs/arquitectura.md`](docs/arquitectura.md)
y [`docs/saga-coreografia.md`](docs/saga-coreografia.md).

## Requisitos

- Go 1.22 o superior.
- PostgreSQL 16 (o Docker Desktop con Compose).
- Opcional: Bruno para ejecutar [`docs/bruno/`](docs/bruno/).
- Opcional: Redocly CLI para validar el contrato.

## Arranque rápido

### Con Docker Compose

```bash
cp .env.example .env
# edite .env y defina CLAVE_FIRMA con al menos 32 bytes aleatorios
docker compose up --build
```

La primera vez, o si se borró el volumen, cree el esquema y los datos de
ejemplo:

```bash
docker compose run --rm app -reset
```

El servidor queda en `http://localhost:8080`.

### Sin Docker

```bash
cp .env.example .env
# complete DATABASE_URL y CLAVE_FIRMA
go run . -reset
```

`-reset` borra las tablas, las vuelve a crear y siembra los datos de ejemplo.
Sin `-reset` solo migra y siembra si las tablas están vacías.

### Credenciales de ejemplo

| Usuario | Clave | Rol | Usuario de negocio |
| --- | --- | --- | --- |
| `luis` | `luis-2026` | administrador | El Golazo |
| `ana` | `ana-2026` | cliente | María Pérez |

## Flujo asíncrono en 60 segundos

```bash
# 1. Login
TOKEN=$(curl -s -X POST http://localhost:8080/login \
  -H 'Content-Type: application/json' \
  -d '{"usuario":"ana","clave":"ana-2026"}' | jq -r '.datos.token')

# 2. Crear reserva
curl -s -X POST http://localhost:8080/reservas \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"CanchaID":1,"Fecha":"2026-10-21","HoraInicio":"18:00","HoraFin":"19:00"}'

# 3. Declarar la seña como administrador
TOKEN_ADMIN=$(curl -s -X POST http://localhost:8080/login \
  -H 'Content-Type: application/json' \
  -d '{"usuario":"luis","clave":"luis-2026"}' | jq -r '.datos.token')

curl -s -X PATCH http://localhost:8080/reservas/1/estado \
  -H "Authorization: Bearer $TOKEN_ADMIN" \
  -H 'Content-Type: application/json' \
  -d '{"NuevoEstado":"pago_por_verificar","Metodo":"efectivo","ComprobanteURL":"https://ejemplo.invalid/1.jpg"}'
# → 202 {"ok":true,"datos":{"saga_id":"reserva-1","estado":"iniciada","paso":"iniciada"}}
#    Location: /sagas/reserva-1

# 4. Consultar la saga hasta que avance
curl -s http://localhost:8080/sagas/reserva-1 \
  -H "Authorization: Bearer $TOKEN_ADMIN" | jq
```

La respuesta `202` significa **aceptado, no terminado**. El cliente debe hacer
polling de `GET /sagas/{id}`. Los fallos de negocio posteriores aparecen en la
saga como `estado: "fallida"` y `error` con el motivo.

## Endpoints

Todas las respuestas usan la envoltura `{"ok": ..., "datos": ...}` o
`{"ok": false, "error": {"codigo": ..., "mensaje": ...}}`.

| Método | Ruta | Rol | Éxito | Notas |
| --- | --- | --- | --- | --- |
| `POST` | `/login` | público | `200` | Devuelve token JWT. |
| `GET` | `/canchas` | cualquier sesión | `200` | Catálogo; el cliente ve activas. |
| `POST` | `/canchas` | administrador | `201` | El dueño sale del token. |
| `PATCH` | `/canchas/{id}` | administrador dueño | `200` | Edición parcial. |
| `POST` | `/reservas` | cliente | `201` | Crea la reserva y `ReservaCreada`. |
| `GET` | `/reservas` | cualquier sesión | `200` | Filtra por propiedad. |
| `GET` | `/reservas/{id}` | dueño o admin de la cancha | `200` | Incluye `Pagos`. |
| `PATCH` | `/reservas/{id}/estado` | administrador dueño | `202` | Inicia un paso de la saga. |
| `GET` | `/sagas/{id}` | cualquier sesión | `200` | Estado, paso, error y recurso. |
| `POST` | `/partidos` | cliente | `202` | Solicita crear el partido. |
| `GET` | `/partidos/{id}` | cualquier sesión | `200` | Detalle del partido. |
| `POST` | `/partidos/{id}/inscripciones` | cliente | `201` | Inscripción con cupo. |

Contrato completo: [`docs/openapi.yaml`](docs/openapi.yaml).
Guía con ejemplos y códigos de error: [`docs/api.md`](docs/api.md).

## Estructura del repositorio

```
.
├── main.go                     # composición: DB, bus, outbox, rutas
├── main_integracion_test.go    # E2E con PostgreSQL y polling de sagas
├── internal/
│   ├── eventos/                # bus, outbox, despachador, idempotencia
│   ├── sagas/                  # proyección y GET /sagas/{id}
│   ├── reservas/               # Reserva + consumidores de su ciclo
│   ├── pagos/                  # Pago/seña + consumidores
│   ├── partidos/               # Partido + consumidores
│   ├── canchas/
│   ├── inscripcion/
│   ├── sesion/
│   ├── usuarios/
│   ├── respuesta/
│   ├── middleware/
│   └── config/
├── docs/
│   ├── arquitectura.md
│   ├── saga-coreografia.md
│   ├── modelo-de-datos.md
│   ├── api.md
│   ├── openapi.yaml
│   ├── api.html
│   ├── operacion.md
│   ├── decisiones.md
│   ├── bruno/
│   └── legacy/                 # documentación histórica, no autoritativa
├── docker-compose.yml
└── redocly.yaml
```

Regla de dependencias: `eventos` no conoce ningún contexto; `sagas` solo
conoce `eventos`; los contextos de negocio pueden importar `eventos` y `sagas`,
pero no al revés. `partidos` tiene una lectura de solo lectura de `reservas`
para validar la petición antes de aceptar el `202`; la decisión de negocio
sigue siendo de `reservas`.

## Pruebas

```bash
# Pruebas unitarias (sin base de datos)
go test ./...

# Análisis estático
go vet ./...

# Compilación
go build ./...

# Integración: necesita una base DESECHABLE
TEST_DATABASE_URL="host=localhost user=postgres password=postgres dbname=canchas_test port=5432 sslmode=disable" \
  go test -tags=integration . -run TestAPIExtremoAExtremo -v -count=1
```

La prueba de integración crea una reserva, recorre pago → confirmación →
jugada, crea un partido y verifica la compensación al cancelar (seña anulada y
partido cancelado).

En Windows, si el antivirus bloquea los ejecutables temporales de Go, use:

```powershell
$env:GOTMPDIR="$env:TEMP\gotmp"
go test ./...
```

## Documentación

| Documento | Contenido |
| --- | --- |
| [`docs/arquitectura.md`](docs/arquitectura.md) | Contextos, bus, outbox, dependencias y diagramas. |
| [`docs/saga-coreografia.md`](docs/saga-coreografia.md) | Catálogo de eventos, pasos y compensaciones. |
| [`docs/modelo-de-datos.md`](docs/modelo-de-datos.md) | Tablas, columnas, relaciones y estados. |
| [`docs/api.md`](docs/api.md) | Contrato funcional, ejemplos y errores. |
| [`docs/openapi.yaml`](docs/openapi.yaml) | Contrato OpenAPI 3.1. |
| [`docs/operacion.md`](docs/operacion.md) | Arranque, migraciones, logs y diagnóstico. |
| [`docs/decisiones.md`](docs/decisiones.md) | ADRs de la migración. |
| [`docs/bruno/`](docs/bruno/) | Colección Bruno de la API asíncrona. |
| [`docs/legacy/`](docs/legacy/) | Documentación histórica. |

## Uso de IA

Se usó IA como apoyo para revisar la migración a Saga Coreografía, contrastar
el catálogo de eventos y proponer pruebas y documentación. Cada propuesta se
validó con `go test ./...`, `go vet ./...`, `go build ./...` y la prueba de
integración. Las decisiones de negocio y la responsabilidad de la entrega
siguen siendo del equipo. Los ejemplos y credenciales de este README son
ficticios.
