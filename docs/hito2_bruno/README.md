# Colección Bruno · Hito 2

Colección importable para demostrar `401`, `200` de login, `200` de consulta,
`403` por rol, `200` de la misma operación autorizada y `422` por transición
inválida. Usa las rutas y la envoltura reales de `docs/openapi.yaml`.

> **Estado de esta entrega:** la colección está preparada y revisada
> estáticamente. **Las peticiones todavía no se ejecutaron** contra PostgreSQL
> real. No se creó `docs/hito2_capturas/` ni se generaron capturas.

## Estructura

```text
docs/hito2_bruno/
├── bruno.json
├── collection.bru
├── README.md
├── 0 - Preparacion - login admin.bru
├── 1 - Sin token - 401.bru
├── 2 - Login cliente - 200.bru
├── 3 - Consultar reservas - 200.bru
├── 4 - PATCH cancha como cliente - 403.bru
├── 5 - PATCH cancha como admin - 200.bru
└── 6 - Transicion invalida - 422.bru
```

La colección se importa en Bruno con **Import Collection → Bruno Collection** y
se selecciona esta carpeta. No requiere un entorno activo: las variables están
en `collection.bru`.

## Variables

| Variable | Origen | Uso |
|---|---|---|
| `baseUrl` | `collection.bru` | `http://127.0.0.1:8080` |
| `adminToken` | Script de la petición 0 | JWT de `luis` |
| `clienteToken` | Script de la petición 2 | JWT de `ana` |
| `reservaId` | Script de la petición 3, o asignación manual verificada | Reserva `pendiente` para la petición 6 |
| `canchaId` | Script de la petición 3, o asignación manual verificada | Cancha de `luis` para las peticiones 4 y 5 |

Los tokens y los IDs que obtienen los scripts son **variables de runtime**: no
se escriben en archivos versionados y se pierden al cerrar Bruno. `reservaId` y
`canchaId` **no tienen valor por defecto**: la petición 3 los limpia y solo los
completa si encuentra una reserva `pendiente` de `ana`. Si no la encuentra,
quedan vacíos y las peticiones 4–6 fallan sin modificar datos. No hay tokens
reales guardados en la colección.

### Si el script no funciona

La estructura se validó con el parser oficial `@usebruno/lang` 0.41.0 y los
scripts se contrastaron con la documentación oficial de Bruno (`bru.setVar` y
`script:post-response`). No hay una instalación local de la aplicación Bruno ni
una colección previa en el repositorio para ejecutarla con el programa real. Si
la versión instalada no ejecuta los scripts:

1. Ejecute la petición 0 y copie `datos.token` de la respuesta en la variable de
   colección `adminToken` (Collection Settings → Variables).
2. Ejecute la petición 2 y copie `datos.token` en `clienteToken`.
3. Ejecute la petición 3; si no se actualizaron solos, copie el `ID` y el
   `CanchaID` de una misma reserva con `"Estado": "pendiente"` en `reservaId` y
   `canchaId`. No asigne esos valores sin comprobar antes el estado y que la
   cancha pertenece a `luis`.
4. Como alternativa, pegue el token directamente después de `Bearer ` en la
   cabecera de cada petición, sin guardar el cambio.

## Precondiciones

1. El servidor debe estar levantado en `baseUrl` con PostgreSQL configurado y la
   migración aplicada. Esta colección no levanta ni borra bases.
2. La siembra debe haber creado los usuarios de negocio y las cuentas:
   - `usuarios.Sembrar` solo actúa con la tabla `usuarios` vacía.
   - `sesion.Sembrar` crea o actualiza `ana` / `ana-2026` (`cliente`) y
     `luis` / `luis-2026` (`administrador`) y las vincula al primer usuario de
     cada rol.
   - `canchas.Sembrar`, `reservas.Sembrar`, `partidos.Sembrar` e
     `inscripcion.Sembrar` completan los datos de ejemplo.
3. En una base limpia, la petición 3 obtiene `canchaId = 1` y `reservaId = 1`:
   la reserva 1 está `pendiente` y la cancha 1 pertenece a `luis`. La reserva 2
   está `confirmada` y ya tiene el partido 1. Las variables no traen esos IDs
   por defecto.
4. Si la base ya tenía datos, verifique que `canchaId` sea una cancha de `luis`
   y que `reservaId` sea una reserva `pendiente` de una cancha de `luis`. No
   suponga que los IDs siguen siendo 1.
5. Las cuentas solo funcionan si están vinculadas. Un `403 cuenta_sin_vinculo`
   indica un problema de siembra, no de la colección.

### Verificación no destructiva

- La petición 3 (`GET /reservas` con `clienteToken`) es la comprobación principal:
  no modifica nada, limpia los IDs anteriores y solo los completa si encuentra
  una reserva `pendiente` de `ana`. Si no hay ninguna, los deja vacíos; las
  peticiones 4–6 fallan de forma segura y se deben detener hasta asignar datos
  verificados. La petición 6 necesita una reserva `pendiente` para producir el
  `422` esperado.
- La petición 3 no puede comprobar con el token de `ana` que la cancha sea de
  `luis`. Para confirmarlo, haga un `GET /canchas` con `adminToken` (petición ad
  hoc, fuera de las seis): el administrador solo ve sus propias canchas. El
  `CanchaID` de la reserva elegida debe estar entre esos `ID`. Si no está, no
  reemplace solo `canchaId`: busque otra reserva `pendiente` de `ana` en una
  cancha de `luis` y asigne `reservaId` y `canchaId` desde esa misma reserva, o
  deténgase.
- La petición 5 usa `PATCH` con cuerpo `{}`. El manejador conserva todos los
  campos existentes y guarda la misma cancha: no cambia nombre, tipo, dirección,
  precio ni estado. No se debe enviar un cuerpo con cambios para esta demo.
- La petición 6 pide `confirmada` desde una reserva `pendiente`. Esa transición
  no está en la máquina de estados (`pendiente → pago_por_verificar | cancelada`),
  así que el servidor responde `422` antes de guardar. Si la reserva ya está en
  `pago_por_verificar`, `confirmada` sí sería válida; en ese caso elija otra
  reserva `pendiente` o cambie el objetivo a un estado sin transición desde el
  estado actual.
- No ejecute la petición 6 si `reservaId` está vacío. Tampoco la ejecute con un
  ID manual sin comprobar antes, con `GET /reservas/{id}` y el token de `luis`,
  que la reserva está `pendiente` y pertenece a una cancha suya.
- No use `go run . -reset` para preparar esta demostración: borra las tablas.

## Orden y resultados esperados

| # | Petición | Resultado esperado | Cuerpo de error esperado |
|---|---|---|---|
| 0 | Login de `luis` | `200` | — |
| 1 | `GET /reservas` sin token | `401` | `falta_token` |
| 2 | Login de `ana` | `200` | — |
| 3 | `GET /reservas` con `clienteToken` | `200` | — |
| 4 | `PATCH /canchas/{{canchaId}}` con `clienteToken` y `{}` | `403` | `permiso_denegado` |
| 5 | `PATCH /canchas/{{canchaId}}` con `adminToken` y `{}` | `200` | — |
| 6 | `PATCH /reservas/{{reservaId}}/estado` con `adminToken` y `{"NuevoEstado":"confirmada"}` | `422` | `transicion_invalida` |

Envoltura real: éxito `{"ok":true,"datos":...}`; error
`{"ok":false,"error":{"codigo":"...","mensaje":"..."}}`.

## Ejecución

1. Importe la carpeta en Bruno.
2. Ejecute la petición **0**. Debe guardar `adminToken`.
3. Ejecute **1**: sin `Authorization` debe dar `401 falta_token`.
4. Ejecute **2**: debe guardar `clienteToken`.
5. Ejecute **3**: debe listar las reservas de `ana` y actualizar `reservaId` /
   `canchaId` si encuentra una pendiente.
6. Ejecute **4**: con el rol cliente debe dar `403 permiso_denegado`.
7. Ejecute **5**: con el rol administrador y cuerpo `{}` debe dar `200`.
8. Ejecute **6**: debe dar `422 transicion_invalida`.

También se pueden ejecutar las siete con el Collection Runner; el campo `seq`
las mantiene en ese orden. Si el token vence (una hora), repita 0 y 2.

## Capturas previstas (no creadas todavía)

Cuando exista una base PostgreSQL real y las peticiones se hayan ejecutado:

1. Capturar la petición 1 con el estado `401` y el cuerpo `falta_token`.
2. Capturar la petición 2 con el estado `200` (sin mostrar el JWT completo).
3. Capturar la petición 3 con el estado `200` y las reservas de `ana`.
4. Capturar la petición 4 con el estado `403` y el cuerpo `permiso_denegado`.
5. Capturar la petición 5 con el estado `200` y el cuerpo de la cancha.
6. Capturar la petición 6 con el estado `422` y el cuerpo `transicion_invalida`.

En las capturas conviene ocultar o sustituir el valor de `Authorization` por
`Bearer •••`; no se deben publicar tokens. Guardar las imágenes en
`docs/hito2_capturas/` solo después de ejecutar y comprobar los resultados
reales. No se debe presentar una captura simulada como evidencia.

## Pendiente de PostgreSQL

Esta colección no demuestra por sí sola que PostgreSQL esté integrado: los
manejadores y la siembra están implementados, pero la ejecución real de las seis
peticiones contra una base levantada **sigue pendiente**. Las pruebas Go
existentes usan dobles para las dependencias de base; no sustituyen esta
comprobación de integración.
