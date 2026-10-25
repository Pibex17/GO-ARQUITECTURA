# Hito 2 · Hoja de demostración

**ServA · API de reservas de canchas sintéticas**<br>
`develop-jorge` · Contrato: `docs/openapi.yaml` · Colección: `docs/hito2_bruno/`

## 1. Usuarios de ejemplo

Cuentas ficticias de `internal/sesion/semilla.go`; no son credenciales reales. El JWT dura una hora.

| Cuenta | Clave | Rol | Vínculo en base limpia |
|---|---|---|---|
| `ana` | `ana-2026` | `cliente` | María Pérez, usuario 2 |
| `luis` | `luis-2026` | `administrador` | El Golazo, usuario 1 |

Solo funcionan si la siembra se ejecutó bien: `usuarios.Sembrar` solo actúa con la tabla vacía; `sesion.Sembrar` crea o actualiza las cuentas y las vincula al primer usuario de su rol. Sin vínculo, la API responde `403 cuenta_sin_vinculo`.

## 2. Peticiones de demostración

Preparación **0** (no cuenta entre las seis): `POST /login` de `luis` → `200`; guarda `adminToken`. Las seis peticiones van en este orden:

| # | Método y ruta | Propósito | Código esperado |
|---|---|---|---|
| 1 | `GET /reservas` | Ruta protegida sin token. | `401 falta_token` |
| 2 | `POST /login` de `ana` | Login exitoso; guarda `clienteToken`. | `200` |
| 3 | `GET /reservas` | Reservas de `ana` con `clienteToken`. | `200` |
| 4 | `PATCH /canchas/{{canchaId}}` con `clienteToken` y `{}` | Misma operación del punto 5 con rol incorrecto. | `403 permiso_denegado` |
| 5 | `PATCH /canchas/{{canchaId}}` con `adminToken` y `{}` | Misma operación autorizada; no cambia campos. | `200` |
| 6 | `PATCH /reservas/{{reservaId}}/estado` con `adminToken` y `{"NuevoEstado":"confirmada"}` | Transición inválida desde `pendiente`. | `422 transicion_invalida` |

`canchaId` debe ser una cancha de `luis` y `reservaId` una reserva `pendiente` de una cancha suya. La petición 3, no destructiva, solo completa ambos IDs si encuentra una pendiente de `ana`; si no, quedan vacíos y 4–6 no se ejecutan. Confirme con `GET /canchas` de `luis` que el `CanchaID` de esa misma reserva está entre las suyas; si no, busque otra pendiente en una cancha suya y no mezcle IDs. En base limpia son cancha `1` y reserva `1`. El `PATCH` de la petición 5 conserva la cancha existente. La petición 6 no guarda porque el `422` ocurre antes del cambio.

## 3. Matriz de rutas por rol

Las 11 operaciones de `docs/openapi.yaml`. `401` aplica a toda ruta protegida sin token válido; `404 ruta_inexistente` y `405 metodo_no_permitido` son globales.

| Método y ruta | Acceso | Cliente | Admin | Propiedad / alcance |
|---|---|---|---|---|
| `POST /login` | Pública | — | — | Emite el JWT. |
| `GET /canchas` | Con sesión | Sí | Sí | Cliente: activas; admin: solo las suyas. |
| `POST /canchas` | Solo admin | No · `403` | Sí | Dueño desde la sesión. |
| `PATCH /canchas/{id}` | Solo admin | No · `403` | Sí | Solo una cancha propia. |
| `POST /reservas` | Solo cliente | Sí | No · `403` | Cliente desde la sesión. |
| `GET /reservas` | Con sesión | Sí | Sí | Cliente: las suyas; admin: las de sus canchas. |
| `GET /reservas/{id}` | Sesión + propiedad | Sí | Sí | Si existe pero no es accesible, `403`. |
| `PATCH /reservas/{id}/estado` | Admin + propiedad | No · `403` | Sí | Solo sus canchas; transición inválida, `422`. |
| `POST /partidos` | Solo cliente | Sí | No · `403` | Reserva propia y `confirmada`; una por reserva. |
| `GET /partidos/{id}` | Con sesión | Sí | Sí | Cualquier token; sin propiedad. |
| `POST /partidos/{id}/inscripciones` | Solo cliente | Sí | No · `403` | Jugador desde la sesión; código obligatorio. |

## 4. Cumplimiento del Hito 2

> **Cubierto y verificable:** login JWT con dos roles (0 y 2); `401 falta_token` sin token (1); consulta autenticada `200` (3); `403 permiso_denegado` por rol (4); misma operación autorizada `200` con cuerpo `{}` (5); `422 transicion_invalida` sin cambios (6); contrato de 11 operaciones validado con Redocly; envoltura real `{"ok":true,"datos":...}` / `{"ok":false,"error":{"codigo":"...","mensaje":"..."}}`; pruebas Go, build, vet y módulos correctos.
>
> **Pendiente:** ejecución de las seis peticiones contra PostgreSQL real. Esta hoja y la colección dejan los pasos; no se afirma que se hayan ejecutado ni se crearon capturas (`docs/hito2_capturas/` no existe todavía).
