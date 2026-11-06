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

## Semana 10 · transacciones

### D1 · El caso

El caso de uso es `AplicarCambioEstado`, en `internal/reservas/servicio.go`: al pasar una reserva a `pago_por_verificar` se escribe una fila nueva en `pagos` (la seña) y se actualiza `reservas.estado`; al pasarla a `confirmada` se actualiza el pago pendiente a `verificado` y también se actualiza la reserva. Son dos escrituras en dos tablas.

Si falla la segunda escritura sin transacción, la reserva queda en `pago_por_verificar` o `confirmada` sin el pago correspondiente, o el pago queda verificado sin que la reserva haya cambiado. El cliente recibe un error, pero la mitad del trabajo ya quedó guardada. Por ejemplo, un pago con monto `0` viola la restricción `monto > 0`; sin transacción, la reserva ya se habría actualizado y el pago no existiría.

### D2 · Opciones que consideramos

1. **Transacción en el manejador o en el servicio.** El manejador abriría una transacción de GORM y pasaría el `*gorm.DB` transaccional al repositorio. Rompe la restricción 1 (el servicio o el manejador importan GORM), obliga a cambiar los manejadores y hace que las pruebas del servicio dependan de una base.
2. **Unit of Work.** El servicio recibiría una interfaz con `Begin`, `Commit` y `Rollback`, y el repositorio participaría en esa unidad. Mantiene SQL fuera del servicio, pero le entrega el control del ciclo de transacción, complica el repositorio falso y hace más fácil olvidar un `Rollback`.
3. **Un método transaccional del repositorio: `AplicarCambioEstado`.** El servicio expresa una sola intención de negocio y el adaptador decide cómo confirmar o revertir las dos escrituras. El servicio no importa GORM ni `database/sql`, las pruebas siguen usando un repositorio falso y los manejadores no cambian.
4. **Transacción que cruza GORM y sqlc.** El adaptador sqlc usaría la transacción de GORM y metería una consulta sqlc dentro. Funciona en PostgreSQL, pero mezcla las dos herramientas y no cumple la restricción 5: cada adaptador debe resolver su propia transacción.

### D3 · La que elegimos y por qué

Elegimos la opción 3. `AplicarCambioEstado` es el límite transaccional del caso de uso; el servicio no puede olvidar la transacción porque no tiene operaciones separadas para guardar la reserva y crear el pago.

- En `RepositorioGORM`, una `DB.Transaction` guarda primero la reserva (con `Omit("Pagos")` para no tocar la relación desde `Save`) y después crea o verifica el pago. Si el pago falla, GORM revierte el cambio de estado.
- En `ReservasSqlc`, `Conexion.BeginTx` abre una transacción propia de `database/sql`; `q.WithTx(tx)` ejecuta `GuardarReserva`, `CrearPago` o `VerificarPago` dentro de ella y el `Commit` o el `Rollback` cierra el caso. No se usa la transacción de GORM ni se cruzan las herramientas.
- La restricción `monto > 0` en `Pago.Monto` y en `db/schema.sql` es la red de seguridad de la base: un monto inválido no puede quedar guardado.

Con esto se cumplen las cinco restricciones: el servicio no importa GORM ni `database/sql`; las pruebas del servicio siguen con el repositorio falso; los manejadores y las seis peticiones no cambian; el adaptador sqlc se puede seleccionar en `main.go` sin tocar el servicio ni sus pruebas; y cada adaptador revierte sus dos escrituras ante un fallo.

### D4 · La que descartamos y por qué

Descartamos la transacción en el manejador o en el servicio porque habría que cambiar los manejadores, inyectar GORM o `database/sql` en el servicio y renunciar a las pruebas con repositorio falso. Descartamos Unit of Work porque traslada `Begin`/`Commit`/`Rollback` al servicio: el repositorio falso tendría que simular una transacción y el riesgo de olvidar el `Rollback` vuelve a aparecer en la capa equivocada. Descartamos la transacción que cruza GORM y sqlc porque el adaptador sqlc quedaría atado a GORM y no sería un adaptador sqlc independiente; además, cambiar de herramienta obligaría a rehacer la transacción. No implementamos `ConfirmarYCobrar` porque el proyecto ya tiene un caso real equivalente en `AplicarCambioEstado`; inventar otro método habría duplicado la regla de negocio.

### D5 · Con sqlc

Para que sqlc participara en las dos escrituras agregamos la tabla `pagos` a `db/schema.sql`, las consultas `CrearPago :one` y `VerificarPago :exec` a `db/consultas.sql`, y un override en `sqlc.yaml` para que `numeric` se genere como `float64`. Después de `sqlc generate`, `AplicarCambioEstado` dejó de usar la transacción de GORM: ahora abre una transacción con `Conexion.BeginTx`, ejecuta las consultas con `a.q.WithTx(tx)` y confirma o revierte. El cambio de diseño fue mover la transacción al adaptador y hacer que las dos escrituras del caso sean consultas sqlc; el servicio, sus pruebas, los DTO y los manejadores quedaron igual. `ReservaPorID` sigue cargando los pagos con GORM porque esa lectura no forma parte de la transacción de escritura. Para usar este adaptador: `ADAPTADOR_RESERVAS=sqlc go run .`; sin la variable se conserva GORM.

### D6 · Archivos

**Archivos tocados:** `internal/reservas/modelos.go` (restricción `monto > 0`), `internal/reservas/repositorio.go` (orden y transacción GORM), `internal/almacen/reservas_sqlc.go` (transacción propia de sqlc), `db/schema.sql`, `db/consultas.sql`, `sqlc.yaml`, `internal/almacen/filas/models.go` y `internal/almacen/filas/consultas.sql.go` (generados), `main.go` (selector `ADAPTADOR_RESERVAS`), `transacciones_integracion_test.go` (prueba nueva), `docs/taller10/error_forzado.txt` (evidencia) y este archivo.

**Archivos que quedaron igual:** `internal/reservas/servicio.go`, `internal/reservas/manejadores.go`, `internal/reservas/servicio_test.go`, `internal/reservas/manejadores_test.go`, los DTO de la fase 2 y las rutas HTTP. La prueba de integración existente `main_integracion_test.go` tampoco cambió.

**Evidencia:** `docs/taller10/error_forzado.txt` guarda la salida de `TestTransaccionReservaPagoEsAtomica` con los dos adaptadores. En ambos casos el error es `chk_pagos_monto`, la reserva queda en `pendiente` y `pagos = 0`:

```text
adaptador=gorm error=no se pudo registrar el pago: ERROR: new row for relation "pagos" violates check constraint "chk_pagos_monto" (SQLSTATE 23514)
adaptador=gorm reserva.estado="pendiente" pagos=0
adaptador=sqlc error=no se pudo registrar el pago: ERROR: new row for relation "pagos" violates check constraint "chk_pagos_monto" (SQLSTATE 23514)
adaptador=sqlc reserva.estado="pendiente" pagos=0
```
