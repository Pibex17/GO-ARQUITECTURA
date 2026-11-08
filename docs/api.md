# API

## 1. Convenciones

- Base local: `http://localhost:8080`.
- Cuerpos y respuestas: `application/json; charset=utf-8`.
- Autenticación: `Authorization: Bearer <token>`.
- Envoltura de éxito:

```json
{ "ok": true, "datos": { } }
```

- Envoltura de error:

```json
{
  "ok": false,
  "error": { "codigo": "transicion_invalida", "mensaje": "no se puede pasar de jugada a cancelada" }
}
```

- Los campos de entrada usan PascalCase (`CanchaID`, `NuevoEstado`,
  `ReservaID`, …), salvo `POST /login`, que usa `usuario` y `clave`.
- Los errores síncronos (`400`, `401`, `403`, `404`, `409`, `422`) significan que
  la petición **no** fue aceptada. Un `202` significa aceptada; el resultado
  final se consulta en la saga.

## 2. Autenticación

### `POST /login`

Público.

```json
{ "usuario": "ana", "clave": "ana-2026" }
```

`200`:

```json
{
  "ok": true,
  "datos": { "token": "eyJhbGciOiJIUzI1NiIs..." }
}
```

Errores: `400 cuerpo_malformado`, `401 credenciales_invalidas`,
`500 error_interno`.

## 3. Canchas

### `GET /canchas`

Cualquier sesión. El cliente ve solo canchas activas; el administrador ve las
suyas. Filtro opcional: `?administrador_id=1` (el cliente solo puede reducir
dentro de las activas; el administrador queda limitado a las propias).

`200`:

```json
{
  "ok": true,
  "datos": [
    {
      "ID": 1,
      "Nombre": "La Bombonera",
      "Tipo": "futbol_5",
      "Direccion": "Av. Siempre Viva 123",
      "PrecioHora": 25,
      "Activa": true,
      "AdministradorID": 1,
      "Horarios": []
    }
  ]
}
```

### `POST /canchas`

Solo administrador. El dueño sale del token; un `AdministradorID` en el cuerpo
se ignora.

```json
{
  "Nombre": "La Bombonera",
  "Tipo": "futbol_5",
  "Direccion": "Av. Siempre Viva 123",
  "PrecioHora": 25
}
```

`201` con la cancha creada. Errores: `400 cuerpo_malformado`,
`422 datos_invalidos`, `422 tipo_invalido`, `403 permiso_denegado`.

### `PATCH /canchas/{id}`

Solo el administrador dueño. Edición parcial: los campos ausentes no cambian.
`Activa` es puntero: `false` desactiva.

```json
{ "PrecioHora": 30, "Activa": false }
```

`200` con la cancha actualizada. Errores: `403 permiso_denegado`,
`404 no_encontrado`, `422 tipo_invalido`.

## 4. Reservas

### `POST /reservas`

Solo cliente. El dueño sale del token; `ClienteID` en el cuerpo se ignora.

```json
{
  "CanchaID": 1,
  "Fecha": "2026-10-21",
  "HoraInicio": "18:00",
  "HoraFin": "19:00"
}
```

`201`:

```json
{
  "ok": true,
  "datos": {
    "ID": 7,
    "CanchaID": 1,
    "ClienteID": 2,
    "Fecha": "2026-10-21",
    "HoraInicio": "18:00",
    "HoraFin": "19:00",
    "Estado": "pendiente",
    "Pagos": null
  }
}
```

Errores: `400 cuerpo_malformado`, `422 datos_invalidos`,
`422 cancha_invalida`, `409 horario_ocupado`, `403 cuenta_sin_vinculo`.

### `GET /reservas`

Cualquier sesión. El cliente solo ve las suyas; el administrador solo las de
sus canchas. Filtros: `?estado=confirmada`, `?cliente_id=2` (el cliente no
puede ampliar con este filtro).

### `GET /reservas/{id}`

Dueño o administrador de la cancha. Incluye `Pagos`.

```json
{
  "ok": true,
  "datos": {
    "ID": 7,
    "CanchaID": 1,
    "ClienteID": 2,
    "Estado": "confirmada",
    "Pagos": [
      { "ID": 3, "ReservaID": 7, "Tipo": "sena", "Monto": 7.5, "Estado": "verificado" }
    ]
  }
}
```

Errores: `403 permiso_denegado`, `404 no_encontrado`.

### `PATCH /reservas/{id}/estado`

Solo administrador dueño de la cancha. **Responde `202`**: el cambio real lo
aplica la saga.

```json
{
  "NuevoEstado": "pago_por_verificar",
  "Metodo": "efectivo",
  "ComprobanteURL": "https://ejemplo.invalid/comprobante.jpg"
}
```

`NuevoEstado` permitido según el estado actual:

| Desde | Hacia |
| --- | --- |
| `pendiente` | `pago_por_verificar`, `cancelada` |
| `pago_por_verificar` | `confirmada`, `cancelada` |
| `confirmada` | `jugada`, `cancelada` |
| `jugada` | — |
| `cancelada` | — |

`202`:

```json
{
  "ok": true,
  "datos": {
    "saga_id": "reserva-7",
    "estado": "iniciada",
    "paso": "iniciada"
  }
}
```

Cabecera: `Location: /sagas/reserva-7`.

Errores: `400 cuerpo_malformado`, `400 id_invalido`, `401 falta_token`,
`403 permiso_denegado`, `403 cuenta_sin_vinculo`, `404 no_encontrado`,
`422 estado_invalido`, `422 transicion_invalida`.

## 5. Sagas

### `GET /sagas/{id}`

Cualquier sesión válida. El `id` es determinista: `reserva-{id}`.

```json
{
  "ok": true,
  "datos": {
    "id": "reserva-7",
    "tipo": "ciclo_reserva",
    "agregado_id": 7,
    "estado": "completada",
    "paso": "partido_creado",
    "ultimo_evento_id": "a1b2...",
    "ultimo_evento_en": "2026-11-08T12:00:01Z",
    "recurso_id": 3,
    "creada_en": "2026-11-08T12:00:00Z",
    "actualizada_en": "2026-11-08T12:00:01Z"
  }
}
```

Estados: `iniciada`, `en_proceso`, `completada`, `compensando`, `compensada`,
`cancelada`, `fallida`.

Polling recomendado:

```bash
while true; do
  curl -s http://localhost:8080/sagas/reserva-7 \
    -H "Authorization: Bearer $TOKEN" | jq '.datos | {estado, paso, error}'
  sleep 1
done
```

Errores: `401 falta_token`, `404 no_encontrado`.

## 6. Partidos

### `POST /partidos`

Solo cliente. La reserva debe existir, ser del cliente y estar `confirmada`.
La creación real la hace la saga.

```json
{ "ReservaID": 7, "Nombre": "Amistoso del sábado", "CupoMaximo": 10 }
```

`202`:

```json
{
  "ok": true,
  "datos": { "saga_id": "reserva-7", "estado": "en_proceso", "paso": "reserva_confirmada" }
}
```

Cabecera: `Location: /sagas/reserva-7`.

Errores: `400 cuerpo_malformado`, `403 permiso_denegado`,
`404 no_encontrado`, `409 partido_existente`,
`422 reserva_no_confirmada`, `422 datos_invalidos`.

### `GET /partidos/{id}`

Cualquier sesión.

`200`:

```json
{
  "ok": true,
  "datos": {
    "partido": {
      "ID": 3,
      "ReservaID": 7,
      "Nombre": "Amistoso del sábado",
      "CupoMaximo": 10,
      "CodigoInvitacion": "A1B2C3",
      "CreadoEn": "2026-11-08T12:00:01Z",
      "Estado": "activo"
    },
    "inscriptos": 1,
    "cupos_restantes": 9
  }
}
```

Estados del partido: `activo`, `cancelado`. Errores: `404 no_encontrado`.

### `POST /partidos/{id}/inscripciones`

Solo cliente. El jugador sale del token; un `JugadorID` en el cuerpo se ignora.

```json
{ "CodigoInvitacion": "A1B2C3" }
```

`201` con la inscripción. Errores: `400 cuerpo_malformado`,
`422 datos_invalidos`, `404 no_encontrado`, `422 codigo_invalido`,
`409 ya_inscripto`, `409 partido_lleno`.

## 7. Códigos de error

| HTTP | `codigo` | Cuándo |
| --- | --- | --- |
| `400` | `cuerpo_malformado` | JSON inválido. |
| `400` | `id_invalido` | El id de la ruta no es entero positivo. |
| `401` | `falta_token` | No hay `Authorization`. |
| `401` | `token_invalido` | Token vencido o firma incorrecta. |
| `401` | `credenciales_invalidas` | Login fallido. |
| `403` | `permiso_denegado` | Rol o propiedad incorrectos. |
| `403` | `cuenta_sin_vinculo` | La cuenta no tiene usuario de negocio. |
| `404` | `no_encontrado` | El recurso no existe. |
| `404` | `ruta_inexistente` | Ruta desconocida. |
| `405` | `metodo_no_permitido` | Método incorrecto para la ruta. |
| `409` | `horario_ocupado` | Ya hay reserva para esa cancha/fecha/hora. |
| `409` | `partido_existente` | Ya hay partido para la reserva. |
| `409` | `ya_inscripto` | El jugador ya está inscrito. |
| `409` | `partido_lleno` | No quedan cupos. |
| `422` | `datos_invalidos` | Faltan campos o no son válidos. |
| `422` | `cancha_invalida` | La cancha no existe o está inactiva. |
| `422` | `tipo_invalido` | Tipo de cancha no reconocido. |
| `422` | `estado_invalido` | Estado de reserva no reconocido. |
| `422` | `transicion_invalida` | La máquina de estados no permite el cambio. |
| `422` | `reserva_no_confirmada` | Se pidió partido de una reserva no confirmada. |
| `422` | `codigo_invalido` | Código de invitación incorrecto. |
| `500` | `error_base` | Falla inesperada de base de datos. |
| `500` | `error_interno` | Falla inesperada al iniciar sesión. |

## 8. Ejemplo completo con curl

```bash
BASE=http://localhost:8080

TOKEN=$(curl -s -X POST $BASE/login \
  -H 'Content-Type: application/json' \
  -d '{"usuario":"ana","clave":"ana-2026"}' | jq -r '.datos.token')

ADMIN=$(curl -s -X POST $BASE/login \
  -H 'Content-Type: application/json' \
  -d '{"usuario":"luis","clave":"luis-2026"}' | jq -r '.datos.token')

# Reserva
curl -s -X POST $BASE/reservas -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"CanchaID":1,"Fecha":"2026-10-21","HoraInicio":"18:00","HoraFin":"19:00"}'

# Seña
curl -s -X PATCH $BASE/reservas/1/estado -H "Authorization: Bearer $ADMIN" \
  -H 'Content-Type: application/json' \
  -d '{"NuevoEstado":"pago_por_verificar","Metodo":"efectivo","ComprobanteURL":"https://ejemplo.invalid/1.jpg"}'

# Confirmación
curl -s -X PATCH $BASE/reservas/1/estado -H "Authorization: Bearer $ADMIN" \
  -H 'Content-Type: application/json' \
  -d '{"NuevoEstado":"confirmada"}'

# Partido (después de que la saga llegue a reserva_confirmada)
curl -s -X POST $BASE/partidos -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"ReservaID":1,"Nombre":"Amistoso","CupoMaximo":10}'

# Estado de la saga
curl -s $BASE/sagas/reserva-1 -H "Authorization: Bearer $ADMIN" | jq
```

El contrato formal está en [`openapi.yaml`](openapi.yaml) y la versión
navegable en [`api.html`](api.html).
