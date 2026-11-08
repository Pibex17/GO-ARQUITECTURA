# Operación

## 1. Configuración

Variables de entorno (`.env` en local, entorno del contenedor en Docker):

| Variable | Obligatoria | Descripción |
| --- | --- | --- |
| `DATABASE_URL` | sí | DSN de PostgreSQL (`host=... user=... password=... dbname=... port=...`). |
| `CLAVE_FIRMA` | sí | Secreto de al menos 32 bytes para firmar JWT. El servidor rechaza marcadores. |
| `PUERTO` | no | Puerto HTTP; por defecto `8080`. |
| `TIEMPO_ESPERA_SEGUNDOS` | no | `ReadTimeout`/`WriteTimeout`; por defecto `5`. |

`config.Cargar` valida que no falten valores y que `CLAVE_FIRMA` no sea un
marcador. No hay valores por defecto para secretos.

## 2. Arranque

### Docker Compose

```bash
cp .env.example .env
# complete CLAVE_FIRMA con, por ejemplo, openssl rand -base64 48
docker compose up --build
```

Servicios:

- `db`: PostgreSQL 16 en `localhost:5432`, base `canchas`, volumen
  `datos_postgres`.
- `app`: el binario Go en `localhost:8080`, con `DATABASE_URL` apuntando al
  servicio `db`.

Para crear el esquema y los datos de ejemplo por primera vez:

```bash
docker compose run --rm app -reset
```

### Local

```bash
cp .env.example .env
# complete DATABASE_URL y CLAVE_FIRMA
go run . -reset
```

Sin `-reset` el servidor hace `AutoMigrate` y siembra solo si faltan datos.

### Apagar

```bash
docker compose down          # conserva el volumen
docker compose down -v       # borra también la base
```

## 3. Migraciones y datos de ejemplo

No hay una herramienta de migraciones versionadas: `prepararBase` usa
`AutoMigrate` al arrancar. Para un cambio de esquema:

1. Modifique el modelo GORM.
2. Ejecute en local con una base desechable y revise el SQL con
   `db.Debug()`.
3. En producción, genere el `ALTER TABLE` equivalente y aplíquelo antes de
   desplegar el binario. `AutoMigrate` no borra columnas ni renombra tablas.

`-reset` **borra** todas las tablas del sistema y vuelve a sembrar. Nunca lo
use contra una base con datos reales.

Datos sembrados: usuarios El Golazo (administrador), María Pérez y Juan Torres
(clientes); cuentas `luis`/`luis-2026` (vinculada a El Golazo) y
`ana`/`ana-2026` (vinculada a María Pérez); canchas 1 y 2; reservas 1 y 2; seña
verificada de la reserva 2; partido 1; inscripciones 1 y 2.

## 4. Logs

El middleware `middleware.Registro` escribe una línea por petición con
método, ruta, estado y duración. El despachador escribe:

- `despachador de eventos: ...` si falla la consulta o la actualización del
  outbox.
- Nada por evento exitoso (evita ruido).

GORM está en modo `Debug` durante `AutoMigrate`; en operación normal no
imprime cada consulta.

Para diagnosticar un `202` que no avanza:

```bash
docker compose logs -f app
```

## 5. Diagnóstico del outbox

Conectarse a la base:

```bash
docker compose exec db psql -U postgres -d canchas
```

Eventos pendientes:

```sql
SELECT id, tipo, agregado_id, saga_id, intentos, ocurrido_en
FROM eventos
WHERE estado = 'pendiente'
ORDER BY ocurrido_en;
```

Eventos fallidos:

```sql
SELECT id, tipo, agregado_id, intentos, error
FROM eventos
WHERE estado = 'fallido'
ORDER BY ocurrido_en DESC;
```

Saga de una reserva:

```sql
SELECT id, estado, paso, error, recurso_id, actualizada_en
FROM sagas
WHERE id = 'reserva-1';
```

Eventos procesados por consumidor:

```sql
SELECT consumidor, count(*)
FROM eventos_procesados
GROUP BY consumidor;
```

### Reintentar un evento fallido

El despachador no vuelve a tomar `fallido` automáticamente. Tras corregir la
causa, devuélvalo a `pendiente`:

```sql
UPDATE eventos
SET estado = 'pendiente', intentos = 0, error = NULL
WHERE id = '...';
```

El despachador lo entregará en el siguiente tick (100 ms). La idempotencia
evita repetir efectos ya aplicados.

### Evento atascado en `pendiente`

Compruebe:

1. ¿El proceso `app` está vivo? Sin él no hay despachador.
2. ¿El evento tiene `intentos >= 5`? Entonces debería estar `fallido`; revise
   el log y el campo `error`.
3. ¿La tabla `eventos_procesados` tiene la clave `(evento_id, consumidor)`?
   Entonces el efecto ya se aplicó; el evento puede marcarse `publicado` a
   mano si quedó pendiente por una caída.

## 6. Consistencia y recuperación

- **Caída del proceso**: los eventos `pendiente` siguen en PostgreSQL. Al
  reiniciar, el despachador los retoma.
- **Caída entre el efecto y el evento derivado**: ambos van en la misma
  transacción; no puede quedar uno sin el otro.
- **Entrega duplicada**: `eventos_procesados` bloquea el segundo efecto.
- **Saga desincronizada**: la proyección es reconstruible. Si necesita
  reconstruirla, borre `sagas` y vuelva a publicar los eventos; no hay un
  comando automático, hágalo en una ventana de mantenimiento.
- **Reserva cancelada**: no se borra. La seña queda `anulado` y el partido
  `cancelado`.

## 7. Pruebas

```bash
go test ./...
go vet ./...
go build ./...
```

Integración (base desechable; el test crea y limpia sus tablas):

```bash
TEST_DATABASE_URL="host=localhost user=postgres password=postgres dbname=canchas_test port=5432 sslmode=disable" \
  go test -tags=integration . -run TestAPIExtremoAExtremo -v -count=1
```

En Windows, si el antivirus bloquea ejecutables temporales:

```powershell
mkdir "$env:TEMP\gotmp" -Force
$env:GOTMPDIR="$env:TEMP\gotmp"
go test ./...
```

## 8. Contrato OpenAPI

```bash
npx --yes @redocly/cli@latest lint docs/openapi.yaml
npx --yes @redocly/cli@latest build-docs docs/openapi.yaml -o docs/api.html
```

El repositorio incluye `redocly.yaml` con `security-defined: error`. Toda ruta
nueva debe declarar `security` (global o propia) y documentar sus respuestas.

## 9. Limitaciones operativas actuales

- **Una sola instancia**: el bus es en memoria y el despachador usa un mutex
  local. Dos instancias contra la misma base podrían entregar el mismo evento
  a sus respectivos buses. La idempotencia protege los efectos, pero el
  diseño actual no está pensado para escalar horizontalmente.
- **Sin broker**: los eventos viven en PostgreSQL y se entregan en el mismo
  proceso. Si el proceso está caído, la API también; no hay consumidor
  independiente.
- **Sin métricas**: solo logs. Para producción faltan métricas de outbox
  (pendientes, fallidos, latencia) y alertas.
- **Sin retención**: la tabla `eventos` crece indefinidamente. Antes de
  producción, defina una política de archivo/borrado de eventos `publicado`
  con antigüedad mayor a la ventana de auditoría.
- **Secretos**: `CLAVE_FIRMA` y `DATABASE_URL` se leen del entorno; no se
  registran en logs. Rote `CLAVE_FIRMA` invalidando tokens existentes.
