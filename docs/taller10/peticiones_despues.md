# Taller 10 · Peticiones después (Fase 4)

- **Fecha:** 2026-11-05
- **Rama:** `taller10`
- **Entorno:** PostgreSQL 16 en Docker + servidor local (`go run . -reset`)
- **Adaptador de reservas:** `ADAPTADOR_RESERVAS=sqlc`
- **Base de datos:** `canchas` (PostgreSQL en el contenedor `db`)
- **Servidor:** binario local en `http://127.0.0.1:8080`
- **Evidencia original:** `docs/taller10/peticiones.md` (Fase 0)

## Preparación 0

La base se reinició con `-reset`, por lo que los datos sembrados quedaron con los
IDs habituales. Después se ejecutó la preparación documentada:

| Paso | Petición | Código | Resultado |
| --- | --- | --- | --- |
| 0.1 | `POST /login` con `luis` / `luis-2026` | `200` | Token de administrador obtenido. |
| 0.2 | `GET /canchas` con el token de administrador | `200` | Se eligió la cancha `1`, propiedad del administrador `luis`. |
| 0.3 | `POST /login` con `ana` / `ana-2026` | `200` | Token de cliente obtenido. |
| 0.4 | `POST /reservas` con el token de cliente | `201` | Se creó `R1`, ID `3`, estado `pendiente`. |

Cuerpo usado para crear `R1`:

```json
{
  "CanchaID": 1,
  "Fecha": "2026-11-05",
  "HoraInicio": "23:17:34.447",
  "HoraFin": "23:59:59"
}
```

Respuesta relevante:

```json
{
  "ok": true,
  "datos": {
    "ID": 3,
    "CanchaID": 1,
    "ClienteID": 2,
    "Fecha": "2026-11-05",
    "HoraInicio": "23:17:34.447",
    "HoraFin": "23:59:59",
    "Estado": "pendiente",
    "Pagos": null
  }
}
```

Antes de la petición 6 se repitió la preparación 0 y se creó `R2`, ID `4`,
también en estado `pendiente`, porque la petición 5 consume `R1`.

## Seis peticiones principales

Esta es la variante documentada literalmente en `docs/taller10/peticiones.md`.

| # | Método | Endpoint | Cuerpo / autenticación | Esperado | Obtenido | Resultado funcional | Error |
| --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | `GET` | `/reservas` | Sin token | `401` | `401` | La ruta protegida rechaza la petición. | `falta_token` |
| 2 | `POST` | `/login` | `{"usuario":"ana","clave":"ana-2026"}` | `200` | `200` | Devuelve un JWT de cliente; el token no se registra en la evidencia. | — |
| 3 | `GET` | `/reservas` | `Authorization: Bearer <JWT cliente>` | `200` | `200` | Devuelve las reservas del cliente: IDs 1, 2 y 3. | — |
| 4 | `PATCH` | `/reservas/3/estado` | Cliente + `{"NuevoEstado":"pago_por_verificar"}` | `403` | `403` | El rol cliente no puede cambiar el estado; la autorización ocurre antes de evaluar el cuerpo. | `permiso_denegado` |
| 5 | `PATCH` | `/reservas/3/estado` | Admin + `{"NuevoEstado":"pago_por_verificar","Metodo":"efectivo","ComprobanteURL":"https://ejemplo.invalid/comprobante.jpg"}` | `200` | `200` | `R1` pasa a `pago_por_verificar` y se crea la seña de `7.5` con método `efectivo`. | — |
| 6 | `PATCH` | `/reservas/4/estado` | Admin + `{"NuevoEstado":"confirmada"}` | `422` | `422` | La transición `pendiente → confirmada` se rechaza. | `transicion_invalida` |

### Cuerpos de error obtenidos

Petición 1:

```json
{"ok":false,"error":{"codigo":"falta_token","mensaje":"falta el token: inicie sesión en POST /login"}}
```

Petición 4:

```json
{"ok":false,"error":{"codigo":"permiso_denegado","mensaje":"su rol no tiene permiso para esta acción"}}
```

Petición 6:

```json
{"ok":false,"error":{"codigo":"transicion_invalida","mensaje":"no se puede pasar de pendiente a confirmada"}}
```

### Respuesta relevante de la petición 5

```json
{
  "ok": true,
  "datos": {
    "ID": 3,
    "CanchaID": 1,
    "ClienteID": 2,
    "Fecha": "2026-11-05",
    "HoraInicio": "23:17:34.447",
    "HoraFin": "23:59:59",
    "Estado": "pago_por_verificar",
    "Pagos": [
      {
        "ID": 2,
        "ReservaID": 3,
        "Tipo": "sena",
        "Monto": 7.5,
        "Metodo": "efectivo",
        "ComprobanteURL": "https://ejemplo.invalid/comprobante.jpg",
        "Estado": "pendiente"
      }
    ]
  }
}
```

## Variante adicional: peticiones 4–5 sobre canchas

El mensaje de la tarea y la colección `docs/hito2_bruno/` describen las
peticiones 4 y 5 como `PATCH /canchas/1`, mientras que
`docs/taller10/peticiones.md` las describe como cambios de estado de reservas.
Para no descartar evidencia, se ejecutaron también las tres peticiones de esa
variante. Las peticiones 1–3 son las mismas de la tabla anterior.

| # | Método | Endpoint | Cuerpo / autenticación | Esperado | Obtenido | Resultado funcional | Error |
| --- | --- | --- | --- | --- | --- | --- | --- |
| 4b | `PATCH` | `/canchas/1` | Cliente + `{}` | `403` | `403` | El rol cliente no puede editar la cancha. | `permiso_denegado` |
| 5b | `PATCH` | `/canchas/1` | Admin + `{}` | `200` | `200` | El administrador obtiene la cancha actualizada sin cambiar sus datos. | — |
| 6b | `PATCH` | `/reservas/4/estado` | Admin + `{"NuevoEstado":"confirmada"}` | `422` | `422` | `R2` sigue pendiente y la transición inválida se rechaza. | `transicion_invalida` |

## Comprobación desde el log del servidor

El log del servidor local registró, en orden, las siguientes líneas relevantes:

```text
POST /login -> 200
GET /canchas -> 200
POST /login -> 200
POST /reservas -> 201
GET /reservas -> 401
POST /login -> 200
GET /reservas -> 200
PATCH /reservas/3/estado -> 403
PATCH /reservas/3/estado -> 200
POST /login -> 200
GET /canchas -> 200
POST /login -> 200
POST /reservas -> 201
PATCH /reservas/4/estado -> 422
PATCH /canchas/1 -> 403
PATCH /canchas/1 -> 200
PATCH /reservas/4/estado -> 422
```

Los seis códigos de la variante principal coinciden con los de la Fase 0:
`401`, `200`, `200`, `403`, `200` y `422`.

## Notas

- No se modificó la lógica del backend para obtener estos resultados.
- No se alteró `docs/taller10/peticiones.md`.
- Los JWT reales no se incluyen en esta evidencia; se sustituyeron por `<JWT>`.
- El adaptador sqlc se usó en esta ejecución para comprobar que la integración de
  Fase 3 conserva el comportamiento de la API.
