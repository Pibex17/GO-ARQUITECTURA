# Hito 2 · Hoja de demostración

**ServA · API de reservas de canchas sintéticas**<br>
`main` · Contrato: `docs/openapi.yaml` · Colección: `docs/hito2_bruno/`

## 1. Usuarios de ejemplo

Cuentas ficticias de `internal/sesion/semilla.go`; no son credenciales reales. El JWT dura una hora.

| Cuenta | Clave | Rol | Vínculo en base limpia |
|---|---|---|---|
| `ana` | `ana-2026` | `cliente` | María Pérez, usuario 2 |
| `luis` | `luis-2026` | `administrador` | El Golazo, usuario 1 |

Solo funcionan si la siembra se ejecutó bien: `usuarios.Sembrar` solo actúa con la tabla vacía; `sesion.Sembrar` crea o actualiza las cuentas y las vincula al primer usuario de su rol. Sin vínculo, la API responde `403 cuenta_sin_vinculo`.

## 2. Peticiones de demostración

Preparación **0** (no cuenta entre las seis). Crea una reserva nueva en cada ensayo y deja listos los tokens y los IDs:

| # | Método y ruta | Propósito | Resultado esperado |
|---|---|---|---|
| 0 | `POST /login` de `luis` | Login del administrador. | `200`; guarda `adminToken`. |
| 0.1 | `GET /canchas` con `adminToken` | Elige una cancha propia del administrador. | `200`; guarda `canchaId`. |
| 0.2 | `POST /login` de `ana` | Login del cliente. | `200`; guarda `clienteToken`. |
| 0.3 | `POST /reservas` con `clienteToken` | Crea la reserva nueva del ensayo. | `201`; estado `pendiente`; guarda `reservaId`. |

Las seis peticiones de la demostración van después, en este orden:

| # | Método y ruta | Propósito | Código esperado |
|---|---|---|---|
| 1 | `GET /reservas` | Ruta protegida sin token. | `401 falta_token` |
| 2 | `POST /login` de `ana` | Login exitoso; guarda `clienteToken`. | `200` |
| 3 | `GET /reservas` | Reservas de `ana` con `clienteToken`. | `200` |
| 4 | `PATCH /canchas/{{canchaId}}` con `clienteToken` y `{}` | Misma operación del punto 5 con rol incorrecto. | `403 permiso_denegado` |
| 5 | `PATCH /canchas/{{canchaId}}` con `adminToken` y `{}` | Misma operación autorizada; no cambia campos. | `200` |
| 6 | `PATCH /reservas/{{reservaId}}/estado` con `adminToken` y `{"NuevoEstado":"confirmada"}` | Transición inválida desde `pendiente`. | `422 transicion_invalida` |

La preparación 0.3 usa `Fecha` y `HoraInicio` con la marca de tiempo del ensayo para que dos ejecuciones seguidas no choquen con `409 horario_ocupado`. El servidor no valida el formato de fecha/hora; por eso la hora puede incluir milisegundos. La petición 3 ya no sobrescribe `reservaId` ni `canchaId`: solo los completa si faltan. El `PATCH` de la petición 5 conserva la cancha existente. La petición 6 no guarda porque el `422` ocurre antes del cambio. La transición válida del administrador (`pendiente → pago_por_verificar`, `200`) se prueba en `main_integracion_test.go`, no se cuenta entre las seis.

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

| Requisito | Evidencia |
|---|---|
| Seis peticiones en el orden exigido | `TestAPIExtremoAExtremo` ejecuta 1–6 y la colección Bruno las reproduce. |
| La petición 0 crea una reserva nueva | Preparación 0.3: `POST /reservas` responde `201` y guarda `reservaId`; se repite en cada ensayo con fecha/hora única. |
| Cliente `403`, administrador `200` con transición válida y `422` con transición prohibida | Integración: `403 permiso_denegado`, `422 transicion_invalida` y `200` al pasar `pendiente → pago_por_verificar`; prueba unitaria `TestAdministradorCambiaReservaDeSuCancha`. |
| `go test ./...` y prueba de integración | `go test ./... -count=1` y `go test -tags=integration . -run TestAPIExtremoAExtremo -v -count=1` pasan. |
| Redocly sin errores ni advertencias | `npx @redocly/cli@2.71.0 lint docs/openapi.yaml` termina con `valid`. |
| Redocly sin errores ni advertencias POWERSHELL | `npx.cmd --yes @redocly/cli@2.71.0 lint docs/openapi.yaml` termina con `valid`. POWERSHELL |
| El login no expone la contraseña | `Cuenta.ClaveHash` tiene `json:"-"`; el login solo devuelve `datos.token`. |
| Handlers sin consultas directas y reglas fuera de HTTP | Los handlers usan `Repositorio`/`ResolutorVinculos`; `transicionPermitida` y `NuevoPagoSena` viven en `internal/reservas/modelos.go`. |
| Contrato, hoja, Bruno y capturas | Contrato validado; hoja y colección actualizadas; Bruno CLI ejecutó `10/10` peticiones y `16/16` pruebas; capturas 0–6 en `docs/hito2_bruno/hito2_capturas/`. |
| `docs/decisiones.md` describe la arquitectura real | D3 documenta la prueba de integración y D6 la separación handler → repositorio → GORM. |
| Structs y tablas de todas las entidades | `AutoMigrate` crea usuarios, cuentas, canchas, horarios, reservas, pagos, partidos e inscripciones; verificado en PostgreSQL. |
| Ambos integrantes tienen commits y todo está en `main` | El historial conserva commits de ambos; la entrega se publica en `main`. |
| IA declarada y ambos pueden responder | Declaración en `README.md` y en la ficha; el ensayo de preguntas queda a cargo del equipo. |

