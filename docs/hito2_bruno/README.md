# Colección Bruno · Hito 2

Colección importable para demostrar la autenticación JWT, el control de acceso por roles y la validación de transiciones de estado de la API. Utiliza las rutas y la estructura de respuestas definidas en `docs/openapi.yaml`.

**Estado:** la colección está preparada y revisada estáticamente. La auditoría reportó pruebas Go satisfactorias y peticiones HTTP equivalentes contra un servidor conectado a PostgreSQL. La ejecución completa desde la aplicación Bruno debe confirmarse por separado.

## 1. Estructura

```text
docs/hito2_bruno/
├── 0 - Preparacion - login admin.bru
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
| `clienteToken` | Petición 2 | Token de autenticación del cliente `ana`. |
| `reservaId` | Petición 3 o asignación manual verificada | Reserva pendiente utilizada en la petición 6. |
| `canchaId` | Petición 3 o asignación manual verificada | Cancha del administrador utilizada en las peticiones 4 y 5. |

Los tokens y los identificadores se gestionan mediante las variables y los scripts de la colección. Comprueba que los scripts funcionan con la versión de Bruno instalada.

No asumas que los identificadores son válidos solo porque tengan un valor. Antes de ejecutar las peticiones 4, 5 y 6, verifica que correspondan a los registros requeridos.

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
4. Debe existir una cancha de `luis` y una reserva en estado `pendiente` asociada a esa cancha.
5. Los identificadores deben verificarse contra los datos actuales. No asumas que siguen siendo `1`.

En la base de prueba limpia descrita durante la auditoría se reportaron la cancha `1` y la reserva `1` como registros adecuados para la demostración. En una base con datos previos, comprueba los identificadores antes de utilizarlos.

**No ejecutes `go run . -reset` para preparar la demostración**, porque puede borrar las tablas.

## 5. Orden de ejecución y resultados esperados

La petición 0 es una preparación para obtener el token del administrador. Las seis peticiones restantes son los escenarios principales.

| N.º | Petición | Resultado esperado | Finalidad |
|---|---|---|---|
| 0 | Login de `luis` | `200` | Obtener el token del administrador. |
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

La petición 3 consulta las reservas de `ana`. Si el script correspondiente está configurado y funciona correctamente, puede obtener los identificadores de una reserva pendiente.

Antes de ejecutar las peticiones 4, 5 y 6:

1. Comprueba que `reservaId` identifica una reserva cuyo estado actual sea `pendiente`.
2. Comprueba que el `CanchaID` de esa reserva corresponda a una cancha de `luis`.
3. Utiliza el mismo `CanchaID` como valor de `canchaId`.
4. Si no existe una reserva que cumpla ambas condiciones, detén la secuencia hasta seleccionar o preparar datos válidos.

La petición 3 utiliza el token de `ana`, por lo que no demuestra por sí sola que la cancha pertenezca a `luis`. Para comprobarlo, consulta `GET /canchas` con `adminToken` y verifica que la cancha aparezca entre los registros del administrador.

No ejecutes la petición 6 con un identificador vacío o sin verificar el estado de la reserva.

## 7. Consideraciones sobre las peticiones 5 y 6

### Petición 5: modificación autorizada de cancha

La petición utiliza el cuerpo `{}`. Según el comportamiento descrito para el manejador, la operación debe conservar los campos existentes de la cancha.

Después de ejecutarla, comprueba que no hayan cambiado el nombre, tipo, dirección, precio ni estado. No envíes un cuerpo con cambios para esta demostración.

### Petición 6: transición inválida

La petición intenta cambiar una reserva de `pendiente` a `confirmada`.

Según la máquina de estados documentada, esa transición debe rechazarse con `422 transicion_invalida`. Si la reserva ya se encuentra en otro estado, selecciona una reserva válida antes de ejecutar la petición.

## 8. Capturas y evidencias

Las capturas se encuentran en:

`docs/hito2_bruno/hito2_capturas/`

| Archivo | Escenario que debe documentar |
|---|---|
| `0-preparacion.png` | Preparación o login del administrador. |
| `1-sintoken.png` | Respuesta `401` sin token. |
| `2-logincliente.png` | Login del cliente con respuesta `200`. |
| `3-consultarReservas.png` | Consulta de reservas con respuesta `200`. |
| `4-canchacomocliente.png` | Respuesta `403` al intentar modificar una cancha como cliente. |
| `5-canchacomoadmin.png` | Respuesta `200` al ejecutar la operación como administrador. |
| `6-transicioninvalida.png` | Respuesta `422` por transición inválida. |
| `test.png` | Pendiente de revisar y clasificar. |

Antes de presentar las capturas como evidencia definitiva:

- Verifica visualmente que cada imagen corresponda a la petición indicada.
- Confirma el código HTTP y el cuerpo de respuesta.
- Comprueba que no aparezcan tokens JWT completos ni credenciales.
- Revisa `test.png` para determinar si aporta evidencia adicional.
- No presentes una captura simulada como resultado de una prueba real.

La existencia de las imágenes no demuestra por sí sola que la colección completa se haya ejecutado desde Bruno.

## 9. Validación técnica

Durante la auditoría se reportaron los siguientes comandos con resultado satisfactorio:

```bash
go test ./... -count=1
go build ./...
go vet ./...
go mod verify
```

También se reportó que `docs/openapi.yaml` pasó la validación de Redocly sin errores ni advertencias.

Las pruebas Go existentes utilizan dobles para las dependencias de base de datos. Por tanto, no sustituyen una prueba de integración automatizada contra PostgreSQL.

La auditoría reportó peticiones HTTP equivalentes contra un servidor conectado a PostgreSQL aislado. La ejecución completa desde Bruno debe registrarse por separado si es un requisito de la entrega.

## 10. Estado de la entrega

Al finalizar el Hito 2, registra por separado:

- **Colección preparada:** rutas, cuerpos y orden revisados.
- **Pruebas Go:** resultado de la última ejecución.
- **Integración HTTP/PostgreSQL:** escenarios que se ejecutaron realmente.
- **Ejecución desde Bruno:** confirmar si se ejecutaron las seis peticiones principales desde la aplicación.
- **Evidencias:** capturas verificadas y asociadas a cada escenario.

