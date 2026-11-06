# Arquitectura en capas

Explicación corta y técnica de cómo está organizado el código del servidor: qué
capa hace cada cosa, cómo se comunican y dónde vive cada regla. Complementa a
`docs/arquitectura.md`, que describe la estructura de carpetas y el detalle de
cada paquete.

## 1. La idea en una frase

El proyecto es un **monolito modular en capas**: la petición entra por HTTP
(`main.go` → router), atraviesa los *middleware* (log, pánico, autenticación y
rol), llega al **manejador** del recurso, que solo traduce HTTP; el **servicio**
aplica las reglas de negocio con **funciones puras del dominio** (sin base de
datos) y accede a PostgreSQL **solo a través de la interfaz `Repositorio`**.
Las dependencias apuntan siempre hacia dentro: transporte → servicio →
repositorio; el servicio no conoce HTTP ni GORM.

## 2. Diagrama de capas

```text
┌──────────────────────────────────────────────────────────────┐
│ 0. ARRANQUE          main.go                                 │
│    config → conexión → migrate/siembra → composición         │
│    repositorio → servicio → manejador                        │
├──────────────────────────────────────────────────────────────┤
│ 1. MIDDLEWARE        internal/middleware                     │
│    Registro · Recuperacion · NotFound/MethodNotAllowed       │
├──────────────────────────────────────────────────────────────┤
│ 2. SEGURIDAD         internal/sesion                         │
│    Autenticar (401) → SoloRol (403) → De(r) vía context      │
├──────────────────────────────────────────────────────────────┤
│ 3. TRANSPORTE HTTP   internal/*/manejadores.go               │
│    decodifica el cuerpo, resuelve la sesión, elige el código │
├──────────────────────────────────────────────────────────────┤
│ 4. NEGOCIO           internal/*/servicio.go                  │
│    casos de uso, reglas y autorización de propiedad;         │
│    depende de la interfaz Repositorio, no de GORM            │
├──────────────────────────────────────────────────────────────┤
│ 5. DOMINIO           internal/*/modelos.go                   │
│    entidades + reglas puras (transiciones, seña 30 %)        │
├──────────────────────────────────────────────────────────────┤
│ 6. DATOS             interfaz Repositorio                    │
│    ←→ adaptadores GORM en internal/almacen y repositorio.go  │
├──────────────────────────────────────────────────────────────┤
│ 7. TRANSVERSAL       respuesta, config, semilla              │
│    envoltura {ok,datos|error} · entorno · datos demo         │
└──────────────────────────────────────────────────────────────┘
```

## 3. Responsabilidad de cada capa

| Capa | Archivos | Qué hace | Qué **no** hace |
|---|---|---|---|
| **0. Arranque** | `main.go` | Lee el entorno, conecta con hasta 10 reintentos, ejecuta `AutoMigrate`, siembra datos, compone repositorio → servicio → manejador y arranca el `http.Server` con *timeouts* | No define rutas ni reglas de negocio |
| **1. Middleware** | `internal/middleware/middleware.go` | `Registro` (método, ruta, código y duración con el envoltorio `grabador`) y `Recuperacion` (`recover()` → `500` sin tumbar el proceso) | No valida datos de negocio |
| **2. Seguridad** | `internal/sesion/guardia.go`, `tokens.go` | `Autenticar`: lee `Bearer`, valida el JWT y responde `401` (`falta_token`, `token_vencido`, `token_invalido`) dejando la `Sesion` en el `context`. `SoloRol`: responde `403 permiso_denegado` | No decide la propiedad del recurso |
| **3. Transporte HTTP** | `internal/*/manejadores.go` | Decodifica el JSON, resuelve la sesión y el vínculo de negocio, llama al servicio, traduce el error a código HTTP y registra sus rutas en `Rutas()` | No escribe SQL, no decide reglas ni comprueba propiedad |
| **4. Negocio** | `internal/*/servicio.go` | Casos de uso: valida, autoriza la propiedad del recurso, aplica la máquina de estados y decide los efectos (seña, verificación). Declara la interfaz `Repositorio` que necesita | No importa `net/http`, `gorm` ni `database/sql` |
| **5. Dominio** | `internal/reservas/modelos.go`, `internal/usuarios/modelos.go` | Entidades y reglas puras: `estadosValidos`, `transicionesValidas`, `transicionPermitida(de, a)` y `NuevoPagoSena` (30 % de la hora) | No importa `net/http` ni `gorm` |
| **6. Datos** | `internal/almacen/reservas.go`, `internal/*/repositorio.go` | Adaptadores GORM de la interfaz `Repositorio`: consultas (`First`, `Create`, `Save`, `Preload("Pagos")`, `Count`) y transacciones | No conoce códigos HTTP |
| **7. Transversal** | `internal/respuesta/respuesta.go`, `internal/config/config.go`, `internal/*/semilla.go` | Contrato único `{ok, datos}` / `{ok, error:{codigo, mensaje}}`, variables de entorno y datos de ejemplo | No contiene lógica de un recurso concreto |

## 4. Regla de dependencias

```text
manejador ──► Servicio (interfaz ServicioX) ◄── *Servicio
    │                 └──► Repositorio (interfaz) ◄── almacen.Reservas (GORM)
    └──────► ResolutorVinculos (interfaz) ◄── Vinculos{RepositorioVinculosGORM}
```

- **Inversión de dependencias:** el transporte declara la interfaz `ServicioX`
  que consume; el servicio declara la interfaz `Repositorio`; `main.go` inyecta
  las implementaciones concretas.
- `Manejador{Servicio, Vinculos}` ya no guarda la conexión ni construye
  repositorios: la composición completa ocurre en `nuevoEnrutador`.
- Los paquetes de negocio **no importan `net/http`** salvo en sus
  `manejadores.go`; el servicio de reservas **no importa `gorm`**. El adaptador
  GORM de reservas vive en `internal/almacen/reservas.go`.

## 5. Recorrido de una petición real

Ejemplo: `PATCH /reservas/6/estado` con el token del administrador.

1. **Router (chi)** recibe la petición y ejecuta `Registro` (cronómetro) y
   `Recuperacion` (`defer recover`).
2. **`tokens.Autenticar`** valida firma y expiración, y deja
   `Sesion{CuentaID, Rol}` en el contexto. Si falla, responde `401` y el
   manejador nunca se ejecuta.
3. **`sesion.SoloRol(RolAdministrador)`** corta con `403` si el rol no coincide.
4. **`reservas.Manejador.cambiarEstado`** traduce HTTP: sesión y rol →
   `401`/`403` **antes** de mirar el cuerpo, para que un cliente nunca reciba
   `422`; `json.Decode` → `400 cuerpo_malformado`; después delega en
   `Servicio.CambiarEstado`.
5. **`reservas.Servicio.CambiarEstado`** aplica las reglas en este orden:
   1. `estadosValidos[...]` → `422 estado_invalido`, **sin tocar la base**;
   2. `ReservaPorID` → `404 no_encontrado` y `403` si la cancha no es del
      administrador;
   3. `transicionPermitida(estadoActual, nuevo)` (función pura) → `422
      transicion_invalida`;
   4. efectos: `NuevoPagoSena` (30 %) o verificación de la seña, y
      `AplicarCambioEstado`, que escribe todo en una transacción;
   5. relee la reserva y la devuelve.
6. **`almacen.Reservas`** ejecuta el SQL y la transacción. El servicio nunca vio
   `database/sql` ni GORM.

## 6. Por qué está separado así

1. **Cambiar la base o probar** no obliga a tocar HTTP: el manejador solo conoce
   la interfaz del servicio y el servicio, la interfaz `Repositorio`.
2. **Las reglas de negocio son funciones puras** (`transicionPermitida`,
   `NuevoPagoSena`): se prueban sin servidor ni PostgreSQL, como hace
   `internal/reservas/modelos_test.go`.
3. **La seguridad es una capa, no un `if` repetido**: el `401` lo pone
   `Autenticar`, el `403` de rol lo pone `SoloRol`, y la propiedad del recurso la
   decide cada servicio (`puedeAcceder`).

## 7. Resumen para la defensa

> Cada petición baja por cuatro filtros —log, pánico, token y rol— y llega a un
> manejador que solo traduce HTTP. El servicio aplica las reglas, comprueba la
> propiedad y delega el SQL en la interfaz `Repositorio`; las reglas de estado y
> la seña son funciones puras. Por eso las pruebas de servicio usan dobles sin
> base y la prueba de integración usa PostgreSQL real.
