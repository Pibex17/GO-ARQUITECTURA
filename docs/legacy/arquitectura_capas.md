# Arquitectura en capas

Explicación corta y técnica de cómo está organizado el código del servidor: qué
capa hace cada cosa, cómo se comunican y dónde vive cada regla. Complementa a
`docs/arquitectura.md`, que describe la estructura de carpetas y el detalle de
cada paquete.

## 1. La idea en una frase

El proyecto es un **monolito modular en capas**: la petición entra por HTTP
(`main.go` → router), atraviesa los *middleware* (log, pánico, autenticación y
rol), llega al **manejador** del recurso, que orquesta; este usa **reglas de
dominio puras** (sin base de datos) y accede a PostgreSQL **solo a través de la
interfaz `Repositorio`**. Las dependencias apuntan siempre hacia dentro: la capa
HTTP conoce al dominio, el dominio no conoce HTTP.

## 2. Diagrama de capas

```text
┌──────────────────────────────────────────────────────────┐
│ 0. ARRANQUE          main.go                             │
│    config → conexión (reintentos) → migrate/siembra      │
│    → nuevoEnrutador()                                    │
├──────────────────────────────────────────────────────────┤
│ 1. TRANSPORTE        internal/middleware, chi.Router     │
│    Registro · Recuperacion · NotFound/MethodNotAllowed   │
├──────────────────────────────────────────────────────────┤
│ 2. SEGURIDAD         internal/sesion                     │
│    Autenticar (401) → SoloRol (403) → De(r) vía context  │
├──────────────────────────────────────────────────────────┤
│ 3. APLICACIÓN        internal/*/manejadores.go           │
│    valida, autoriza propiedad, orquesta, responde        │
├──────────────────────────────────────────────────────────┤
│ 4. DOMINIO           internal/*/modelos.go               │
│    entidades + reglas puras (transiciones, seña 30 %)    │
├──────────────────────────────────────────────────────────┤
│ 5. DATOS             internal/*/repositorio.go           │
│    interfaz Repositorio  ←→  implementación GORM         │
├──────────────────────────────────────────────────────────┤
│ 6. TRANSVERSAL       respuesta, config, semilla          │
│    envoltura {ok,datos|error} · entorno · datos demo     │
└──────────────────────────────────────────────────────────┘
```

## 3. Responsabilidad de cada capa

| Capa | Archivos | Qué hace | Qué **no** hace |
|---|---|---|---|
| **0. Arranque** | `main.go` | Lee el entorno, conecta con hasta 10 reintentos, ejecuta `AutoMigrate`, siembra datos y arranca el `http.Server` con *timeouts* | No define rutas ni reglas de negocio |
| **1. Transporte** | `internal/middleware/middleware.go` | `Registro` (método, ruta, código y duración con el envoltorio `grabador`) y `Recuperacion` (`recover()` → `500` sin tumbar el proceso) | No valida datos de negocio |
| **2. Seguridad** | `internal/sesion/guardia.go`, `tokens.go` | `Autenticar`: lee `Bearer`, valida el JWT y responde `401` (`falta_token`, `token_vencido`, `token_invalido`) dejando la `Sesion` en el `context`. `SoloRol`: responde `403 permiso_denegado` | No decide la propiedad del recurso |
| **3. Aplicación** | `internal/*/manejadores.go` | Decodifica el JSON, valida, comprueba propiedad, llama al repositorio, responde con `respuesta.Exito/Error` y registra sus rutas en `Rutas()` | No escribe SQL ni implementa la máquina de estados |
| **4. Dominio** | `internal/reservas/modelos.go`, `internal/usuarios/modelos.go` | Entidades y reglas puras: `estadosValidos`, `transicionesValidas`, `transicionPermitida(de, a)` y `NuevoPagoSena` (30 % de la hora) | No importa `net/http` ni `gorm` |
| **5. Datos** | `internal/*/repositorio.go` | Interfaz `Repositorio` más `RepositorioGORM` con las consultas (`First`, `Create`, `Save`, `Preload("Pagos")`, `Count` para conflictos) | No conoce códigos HTTP |
| **6. Transversal** | `internal/respuesta/respuesta.go`, `internal/config/config.go`, `internal/*/semilla.go` | Contrato único `{ok, datos}` / `{ok, error:{codigo, mensaje}}`, variables de entorno y datos de ejemplo | No contiene lógica de un recurso concreto |

## 4. Regla de dependencias

```text
manejador ──► Repositorio (interfaz)      ◄── RepositorioGORM (implementación)
    └──────► ResolutorVinculos (interfaz) ◄── Vinculos{RepositorioVinculosGORM}
```

- **Inversión de dependencias:** el manejador declara qué necesita mediante
  `interface`; la implementación GORM se enchufa después.
- `Manejador{DB, Repo, Vinculos}`: si `Repo` o `Vinculos` quedan en `nil`, las
  fábricas `m.repositorio()` y `m.vinculos()` usan GORM sobre `DB`. Las pruebas
  inyectan dobles y no abren PostgreSQL.
- Los paquetes de negocio **no importan `net/http`** salvo en sus
  `manejadores.go`. En particular, `internal/reservas/modelos.go` no depende de
  HTTP.

## 5. Recorrido de una petición real

Ejemplo: `PATCH /reservas/6/estado` con el token del administrador.

1. **Router (chi)** recibe la petición y ejecuta `Registro` (cronómetro) y
   `Recuperacion` (`defer recover`).
2. **`tokens.Autenticar`** valida firma y expiración, y deja
   `Sesion{CuentaID, Rol}` en el contexto. Si falla, responde `401` y el
   manejador nunca se ejecuta.
3. **`sesion.SoloRol(RolAdministrador)`** corta con `403` si el rol no coincide.
4. **`reservas.cambiarEstado`** orquesta en este orden (está comentado en el
   archivo porque el orden es parte del contrato):
   1. sesión y rol → `401` / `403` **antes** de mirar el cuerpo, para que un
      cliente nunca reciba `422`;
   2. `json.Decode` → `400 cuerpo_malformado`;
   3. `estadosValidos[...]` → `422 estado_invalido`, **sin tocar la base**;
   4. a partir de aquí sí hay base: `ReservaPorID`, y `403` si la cancha no es
      del administrador;
   5. `transicionPermitida(estadoActual, nuevo)` (función pura) → `422
      transicion_invalida`;
   6. efectos: `NuevoPagoSena` (30 %) o `MarcarPagoSenaVerificado`, y
      `GuardarReserva`;
   7. `respuesta.Exito(200, actualizada)`.
5. **`RepositorioGORM`** ejecuta el SQL. El dominio nunca vio `database/sql` ni
   GORM.

## 6. Por qué está separado así

1. **Cambiar la base o probar** no obliga a tocar HTTP: el manejador solo conoce
   la interfaz `Repositorio`.
2. **Las reglas de negocio son funciones puras** (`transicionPermitida`,
   `NuevoPagoSena`): se prueban sin servidor ni PostgreSQL, como hace
   `internal/reservas/modelos_test.go`.
3. **La seguridad es una capa, no un `if` repetido**: el `401` lo pone
   `Autenticar`, el `403` de rol lo pone `SoloRol`, y la propiedad del recurso es
   lo único que decide cada manejador (`puedeAcceder`).

## 7. Resumen para la defensa

> Cada petición baja por cuatro filtros —log, pánico, token y rol— y llega a un
> manejador que solo orquesta: valida, comprueba propiedad, delega el SQL en una
> interfaz y las reglas de negocio en funciones puras. Por eso el mismo código se
> prueba con dobles en las pruebas unitarias y con PostgreSQL real en la prueba
> de integración.
