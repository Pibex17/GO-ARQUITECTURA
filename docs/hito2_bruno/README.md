# Colección Bruno · Hito 2

Colección importable para demostrar la autenticación JWT, el control de acceso por roles y la validación de transiciones de estado de la API. Utiliza las rutas y la estructura de respuestas definidas en `docs/openapi.yaml`.

**Estado:** la colección crea una reserva nueva en la preparación 0.3 y después ejecuta las seis peticiones principales. La prueba `main_integracion_test.go` cubre el mismo flujo contra PostgreSQL real, incluida la transición válida que no forma parte de las seis. La colección se ejecutó con Bruno CLI 4.3.0 contra PostgreSQL: **10/10 peticiones y 16/16 pruebas en PASS**. Las capturas 0–6 existentes corresponden a los escenarios principales; la preparación nueva se valida con la prueba de integración y con la ejecución de la colección.

## 1. Estructura

```text
docs/hito2_bruno/
├── 0 - Preparacion - login admin.bru
├── 0.1 - Preparacion - canchas admin.bru
├── 0.2 - Preparacion - login cliente.bru
├── 0.3 - Preparacion - nueva reserva.bru
├── 1 - Sin token - 401.bru
├── 2 - Login cliente - 200.bru
├── 3 - Consultar reservas - 200.bru
├── 4 - PATCH cancha como cliente - 403.bru
├── 5 - PATCH cancha como admin - 200.bru
├── 6 - Transicion invalida - 422.bru
├── bruno.json
├── collection.bru
├── README.md
└── hito2_capturas/
    ├── 0-preparacion.png
    ├── 1-sintoken.png
    ├── 2-logincliente.png
    ├── 3-consultarReservas.png
    ├── 4-canchacomocliente.png
    ├── 5-canchacomoadmin.png
    ├── 6-transicioninvalida.png
    └── test.png
```

Para importar la colección, abre Bruno y selecciona **Import Collection → Bruno Collection**. Selecciona la carpeta `docs/hito2_bruno/`.

## 2. Variables

| Variable | Origen | Uso |
|---|---|---|
| `baseUrl` | `collection.bru` | Dirección base del servidor, normalmente `http://127.0.0.1:8080`. |
| `adminToken` | Petición 0 | Token de autenticación del administrador `luis`. |
| `canchaId` | Petición 0.1; se confirma en 0.3 | Primera cancha propia del administrador. La usan las peticiones 4 y 5. |
| `clienteToken` | Peticiones 0.2 y 2 | Token de autenticación del cliente `ana`. |
| `reservaId` | Petición 0.3 | Reserva nueva en estado `pendiente`. La usa la petición 6. |
| `fechaDemo` / `horaDemo` | Script de 0.3 | Fecha y hora únicas de cada ensayo para no chocar con una reserva anterior. |

Los tokens y los identificadores se gestionan mediante variables de runtime y no se guardan en el repositorio. Si un script no corre, asígnalos a mano solo después de verificar los registros.

## 3. Usuarios de prueba

Las cuentas de ejemplo son:

| Usuario | Clave de prueba | Rol |
|---|---|---|
| `ana` | `ana-2026` | `cliente` |
| `luis` | `luis-2026` | `administrador` |

Las cuentas dependen de la siembra de datos del proyecto y de sus vínculos con los usuarios de negocio.

- `usuarios.Sembrar` actúa cuando la tabla de usuarios está vacía.
- `sesion.Sembrar` crea o actualiza las cuentas de prueba y las vincula con los usuarios de negocio correspondientes.
- Las funciones de siembra de canchas, reservas, partidos e inscripciones completan los datos de ejemplo.

Si aparece `403 cuenta_sin_vinculo`, revisa la siembra y los vínculos entre las cuentas y los usuarios de negocio.

## 4. Precondiciones

Antes de ejecutar la colección:

1. El servidor debe estar levantado en la dirección indicada por `baseUrl`.
2. PostgreSQL debe estar disponible y la migración del proyecto debe haberse aplicado.
3. Las cuentas de prueba deben existir y estar correctamente vinculadas.
4. Debe existir al menos una cancha del administrador `luis`; la petición 0.1 la selecciona automáticamente.
5. No hace falta una reserva `pendiente` previa: la petición 0.3 la crea.

**No ejecutes `go run . -reset` para preparar la demostración**, porque puede borrar las tablas. La colección está diseñada para funcionar sobre una base ya sembrada.

## 5. Orden de ejecución y resultados esperados

La preparación **0** no cuenta entre las seis peticiones principales:

| N.º | Petición | Resultado esperado | Finalidad |
|---|---|---|---|
| 0 | Login de `luis` | `200` | Obtener `adminToken`. |
| 0.1 | `GET /canchas` con `adminToken` | `200` | Elegir una cancha propia y guardar `canchaId`. |
| 0.2 | Login de `ana` | `200` | Obtener `clienteToken` para crear la reserva. |
| 0.3 | `POST /reservas` con `clienteToken` | `201` | Crear una reserva nueva en `pendiente` y guardar `reservaId`. |

Después se ejecutan las seis peticiones principales:

| N.º | Petición | Resultado esperado | Finalidad |
|---|---|---|---|
| 1 | `GET /reservas` sin token | `401` | Comprobar la autenticación obligatoria. |
| 2 | Login de `ana` | `200` | Obtener el token del cliente. |
| 3 | `GET /reservas` con `clienteToken` | `200` | Consultar las reservas del cliente. |
| 4 | `PATCH /canchas/{{canchaId}}` con `clienteToken` y `{}` | `403` | Comprobar la restricción por rol. |
| 5 | `PATCH /canchas/{{canchaId}}` con `adminToken` y `{}` | `200` | Comprobar la operación autorizada. |
| 6 | `PATCH /reservas/{{reservaId}}/estado` con `adminToken` y `{"NuevoEstado":"confirmada"}` | `422` | Comprobar el rechazo de una transición inválida. |

Códigos de error esperados:

- Petición 1: `401`, código `falta_token`.
- Petición 4: `403`, código `permiso_denegado`.
- Petición 6: `422`, código `transicion_invalida`.

La estructura general de las respuestas es:

**Respuesta exitosa:**

```json
{
  "ok": true,
  "datos": {}
}
```

**Respuesta de error:**

```json
{
  "ok": false,
  "error": {
    "codigo": "codigo_del_error",
    "mensaje": "Descripción del error"
  }
}
```

Los cuerpos anteriores representan la estructura esperada; los datos concretos dependen de la respuesta real del servidor.

## 6. Verificación de los identificadores

La preparación 0.3 crea la reserva y guarda `reservaId` y `canchaId` desde la respuesta `201`. La petición 3 ya no sobrescribe esos valores: solo busca una reserva pendiente como respaldo si alguien ejecuta la colección sin la preparación.

Antes de ejecutar las peticiones 4, 5 y 6:

1. Comprueba que `reservaId` identifica la reserva recién creada y que su estado es `pendiente`.
2. Comprueba que `canchaId` es una cancha propia de `luis`.
3. Usa el mismo `canchaId` en las peticiones 4 y 5.
4. Si la reserva no está en `pendiente`, vuelve a ejecutar la preparación 0.3.

## 7. Consideraciones sobre las peticiones 5 y 6

### Petición 5: modificación autorizada de cancha

La petición utiliza el cuerpo `{}`. Según el comportamiento descrito para el manejador, la operación conserva los campos existentes de la cancha.

Después de ejecutarla, comprueba que no hayan cambiado el nombre, tipo, dirección, precio ni estado. No envíes un cuerpo con cambios para esta demostración.

### Petición 6: transición inválida

La petición intenta cambiar la reserva nueva de `pendiente` a `confirmada`.

Según la máquina de estados documentada, esa transición debe rechazarse con `422 transicion_invalida`. La reserva sigue `pendiente` después del `422`, así que la misma reserva se puede usar para una transición válida en la prueba de integración.

## 8. Capturas y evidencias

Las capturas se encuentran en:

`docs/hito2_bruno/hito2_capturas/`

| Archivo | Escenario que documenta |
|---|---|
| `0-preparacion.png` | Login del administrador. |
| `1-sintoken.png` | Respuesta `401` sin token. |
| `2-logincliente.png` | Login del cliente con respuesta `200`. |
| `3-consultarReservas.png` | Consulta de reservas con respuesta `200`. |
| `4-canchacomocliente.png` | Respuesta `403` al intentar modificar una cancha como cliente. |
| `5-canchacomoadmin.png` | Respuesta `200` al ejecutar la operación como administrador. |
| `6-transicioninvalida.png` | Respuesta `422` por transición inválida. |
| `test.png` | Captura de la ejecución de `go test ./...`; no es una captura de Bruno. |

La captura 0 muestra un JWT completo. Para una entrega pública, recórtalo o vuelve a tomar la captura mostrando solo el código `200` y el inicio del token.

## 9. Validación técnica

Los comandos que se ejecutan en esta entrega son:

```bash
go test ./... -count=1
go test -tags=integration . -run TestAPIExtremoAExtremo -v -count=1
go build ./...
go vet ./...
go mod verify
npx @redocly/cli@2.71.0 lint docs/openapi.yaml
```

`main_integracion_test.go` usa PostgreSQL real mediante `TEST_DATABASE_URL`, crea una reserva nueva, ejecuta las seis peticiones y comprueba además la transición válida `pendiente → pago_por_verificar` con `200`. Sin `TEST_DATABASE_URL`, la prueba se omite y `go test ./...` sigue funcionando sin Docker.

## 10. Estado de la entrega

- **Colección preparada:** rutas, cuerpos, preparación 0 y orden revisados.
- **Pruebas Go:** `go test ./...` y la prueba de integración pasan.
- **Integración HTTP/PostgreSQL:** reserva nueva, seis peticiones y transición válida cubiertas.
- **Ejecución desde Bruno CLI:** `10/10` peticiones y `16/16` pruebas en `PASS` contra PostgreSQL; la aplicación Bruno usa la misma colección.
- **Evidencias:** capturas 0–6 de los escenarios principales; la preparación 0.3 y la transición válida se evidencian con la ejecución de la colección y la prueba de integración. Si se exige una imagen de la reserva nueva, añadir la de la petición 0.3.
