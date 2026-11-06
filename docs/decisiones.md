# Decisiones

## D1 · Orden de los middleware
**Decisión:** Registro afuera, Recuperación adentro (`r.Use(middleware.Registro); r.Use(middleware.Recuperacion)`).
**Por qué:** si el orden fuera al revés, un pánico en un handler saltaría por encima de Registro antes de que este alcance a escribir la línea del log, y perderíamos el rastro de qué petición reventó.
**Alternativa descartada:** Recuperación afuera, Registro adentro — el pánico nunca llegaría a Registro y el log nunca mostraría el 500.
**Cómo lo comprobamos:** una ruta que fuerza un pánico (por ejemplo, dividir por índice fuera de rango dentro de un handler de prueba); en el log aparece la línea con el 500 antes de que el proceso siga vivo para la siguiente petición.

## D2 · ¿Qué pasa si se intenta borrar una cancha con reservas?
**Decisión:** no implementamos borrado físico de canchas: solo `Activa = false` desde `PATCH /canchas/{id}`. Una cancha con reservas nunca se puede eliminar de la base.
**Por qué:** en nuestro negocio el historial de reservas y pagos de una cancha tiene valor para el administrador (cuánto facturó, qué días se usa más), y borrar la cancha borraría ese historial o rompería la clave foránea de sus reservas.
**Alternativa descartada:** `DELETE /canchas/{id}` con `db.Delete`, que en PostgreSQL respondería `violates foreign key constraint` en cuanto la cancha tuviera una sola reserva — un error de base de datos disfrazado de operación normal del negocio.

## D3 · Prueba de integración contra PostgreSQL real
**Decisión:** el flujo HTTP completo se prueba en `main_integracion_test.go`, con la etiqueta `//go:build integration`, un `httptest.Server` y PostgreSQL real. La prueba usa `TEST_DATABASE_URL` y una base desechable (`canchas_test`), porque `prepararBase` borra y vuelve a sembrar las tablas.
**Qué cubre:** el arranque, la siembra, el login de `luis` y `ana`, la creación de una reserva nueva con `POST /reservas` (`201`), las seis peticiones de la demo (`401`, `200`, `200`, `403`, `200`, `422`), que el `422` no cambia el estado y la transición válida `pendiente → pago_por_verificar` (`200`) con la seña del 30 %.
**Qué sigue solo con dobles:** el conflicto de horario `409 horario_ocupado` y las validaciones unitarias de cada manejador. La consulta `HayConflicto` no se ejecuta contra PostgreSQL en esta prueba.
**Cómo se ejecuta:**
```bash
docker compose exec -T db createdb -U postgres canchas_test
TEST_DATABASE_URL="host=localhost user=postgres password=postgres dbname=canchas_test port=5432 sslmode=disable" \
  go test -tags=integration . -run TestAPIExtremoAExtremo -v -count=1
```
Sin `TEST_DATABASE_URL` la prueba se omite; por eso `go test ./...` no necesita Docker.

## D4 · Cómo resuelve el servidor la dirección de la base dentro de Docker
**Decisión:** `DATABASE_URL` con `host=db` se define directamente en `docker-compose.yml`, no en el `.env` del repositorio. El `.env` que se usa para correr sin Docker sigue con `host=localhost`.
**Por qué:** el nombre de host de PostgreSQL depende de si el servidor corre dentro de la red de Docker (ahí "localhost" sería el propio contenedor del servidor, no el de la base) o directamente en la máquina. Es un valor de infraestructura, no un secreto ni una regla de negocio, así que vive en el archivo que describe esa infraestructura.
**Alternativa descartada:** un único `.env` con `host=db` siempre, que habría roto `go run .` fuera de Docker para cualquiera que no use Docker Compose.

## D5 · Autorización por propietario de los recursos (J5)
**Decisión:** el `sub` del token identifica la cuenta, no al usuario de negocio. El usuario de negocio se resuelve con `Cuenta.UsuarioID` y solo se acepta si el rol firmado en el token, el rol de la cuenta y el rol del usuario coinciden. Si la cuenta no tiene vínculo, la operación responde `403` con `cuenta_sin_vinculo` y no se atribuye a otra persona.
**Reglas de propiedad:**
- `POST /reservas` deriva `ClienteID` de la sesión; un `ClienteID` en el cuerpo se ignora.
- `GET /reservas` filtra por el cliente autenticado. El administrador solo ve las reservas de sus canchas. `?cliente_id=` no amplía el acceso: en el cliente se ignora y en el administrador solo reduce dentro de sus canchas.
- `GET /reservas/{id}` responde `404` si no existe y `403` si no es del cliente o de las canchas del administrador.
- `PATCH /reservas/{id}/estado` exige administrador y que la reserva sea de una cancha suya. La transición prohibida responde `422`, como exige la rúbrica del Hito 2.
- `POST /partidos` exige que la reserva sea del cliente autenticado.
- `POST /partidos/{id}/inscripciones` deriva `JugadorID` de la sesión; un `JugadorID` en el cuerpo se ignora.
- `GET /canchas` devuelve solo activas al cliente y solo las del administrador autenticado a ese rol; `POST /canchas` asigna `AdministradorID` desde la sesión y `PATCH /canchas/{id}` exige que la cancha sea suya.
**Transiciones del cliente pendientes:** la rúbrica del Hito 2 exige que `PATCH /reservas/{id}/estado` sea de un solo rol, así que se conserva solo para administrador. Las transiciones `pendiente → pago_por_verificar`, `pendiente → cancelada` y `confirmada → cancelada`, que la ficha asigna al cliente, no se pueden ejecutar por esa ruta en esta fase. No se creó una ruta alternativa; quedan documentadas para una decisión posterior.
**Limitación cubierta:** la prueba de integración ya ejercita la siembra, el arranque y el flujo completo; el `409 horario_ocupado` sigue cubierto con dobles.

## D6 · Arquitectura por capas y reglas de negocio
**Decisión:** el proyecto se organiza en tres capas con dependencias hacia dentro: **transporte HTTP → servicio → repositorio**. Los manejadores no ejecutan consultas GORM ni crean repositorios: dependen de una interfaz `ServicioX` y de `sesion.ResolutorVinculos`. Cada servicio contiene las reglas de negocio y declara la interfaz `Repositorio` que necesita; las implementaciones GORM viven en `repositorio.go` o `vinculo.go`. `main.go` solo compone: `prepararBase(db, reset)` migra y siembra, y `nuevoEnrutador(db, tokens)` arma el router. Así `main()` y la prueba de integración ejecutan exactamente el mismo esquema y las mismas rutas.
**Reglas fuera de HTTP:** la máquina de estados (`estadosValidos`, `transicionesValidas`, `transicionPermitida`) y la regla de la seña (`NuevoPagoSena`, 30 % de `PrecioHora`) viven en `internal/reservas/modelos.go`, que no importa `net/http` ni GORM. El servicio aplica la transición y decide los efectos; el manejador solo traduce HTTP. La persistencia del pago, la verificación de la seña y el guardado de la reserva ocurren en una sola transacción dentro de `AplicarCambioEstado`.
**Cómo lo comprobamos:** pruebas de servicio con repositorios falsos (`internal/*/servicio_test.go`), pruebas HTTP de los manejadores, `internal/reservas/modelos_test.go` para las transiciones y la seña, y `main_integracion_test.go` para el flujo HTTP completo con PostgreSQL real.
