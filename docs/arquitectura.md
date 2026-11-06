# Arquitectura del servidor

Documento de referencia para el equipo (Jorge incluido). Describe la
estructura real de carpetas, las dependencias entre capas y dónde vive cada
regla de negocio.

## 1. Estructura real de carpetas

```text
ServA-Aragundi-Castro/
├── main.go                      # arranque: config, conexión, migrate/siembra, composición y router
├── main_integracion_test.go     # prueba E2E con `//go:build integration` (PostgreSQL real)
├── internal/
│   ├── almacen/                 # adaptadores GORM que implementan las interfaces de negocio
│   │   └── reservas.go          # implementa reservas.Repositorio
│   ├── canchas/                 # Cancha, Horario
│   │   ├── modelos.go
│   │   ├── servicio.go          # interfaz Repositorio + casos de uso
│   │   ├── servicio_test.go
│   │   ├── repositorio.go       # implementación GORM
│   │   ├── manejadores.go       # HTTP
│   │   ├── manejadores_test.go
│   │   └── semilla.go
│   ├── config/                  # variables de entorno (.env)
│   │   ├── config.go
│   │   └── config_test.go
│   ├── inscripcion/             # Inscripcion a un partido
│   │   ├── modelos.go
│   │   ├── servicio.go
│   │   ├── servicio_test.go
│   │   ├── repositorio.go
│   │   ├── manejadores.go
│   │   ├── manejadores_test.go
│   │   └── semilla.go
│   ├── middleware/              # Registro y Recuperacion
│   │   └── middleware.go
│   ├── partidos/                # Partido
│   │   ├── modelos.go
│   │   ├── servicio.go
│   │   ├── servicio_test.go
│   │   ├── repositorio.go
│   │   ├── manejadores.go
│   │   ├── manejadores_test.go
│   │   └── semilla.go
│   ├── reservas/                # Reserva + Pago + máquina de estados
│   │   ├── modelos.go           # estados y transiciones (sin HTTP)
│   │   ├── modelos_test.go      # prueba pura de la tabla de transiciones
│   │   ├── servicio.go          # interfaz Repositorio + casos de uso
│   │   ├── servicio_test.go
│   │   ├── repositorio.go       # FiltroReservas (sin GORM)
│   │   ├── manejadores.go
│   │   ├── manejadores_test.go
│   │   └── semilla.go
│   ├── respuesta/               # envoltura JSON {ok, datos} / {ok, error}
│   │   └── respuesta.go
│   ├── sesion/                  # cuentas, bcrypt, JWT, guardias y login
│   │   ├── cuentas.go
│   │   ├── contrasenas.go
│   │   ├── tokens.go
│   │   ├── guardia.go
│   │   ├── vinculo.go
│   │   ├── servicio.go
│   │   ├── servicio_test.go
│   │   ├── manejadores.go
│   │   ├── semilla.go
│   │   └── *_test.go
│   └── usuarios/                # Usuario de negocio
│       ├── modelos.go
│       └── semilla.go
├── docs/
│   ├── arquitectura.md          # este documento
│   ├── openapi.yaml             # contrato de la API
│   ├── hito2_hoja.md            # guion de demostración
│   └── hito2_bruno/             # colección de peticiones
└── docker-compose.yml           # PostgreSQL + app
```

## 2. Regla de dependencias

```text
main.go
  └── manejadores HTTP (internal/*/manejadores.go)
        ├── interfaz ServicioX                (definida en manejadores.go)
        │     └── *Servicio                   (internal/*/servicio.go)
        │           └── interfaz Repositorio  (internal/*/servicio.go)
        │                 └── implementación GORM (internal/almacen o repositorio.go)
        └── sesion.ResolutorVinculos          (internal/sesion/vinculo.go)
              └── implementación GORM
```

- **El manejador HTTP no ejecuta consultas ni decide reglas.** Solo conoce la
  interfaz de su servicio y el `ResolutorVinculos`; el servicio conoce la
  interfaz `Repositorio` y las consultas GORM viven en `internal/almacen` o en el
  `repositorio.go` del paquete.
- `Manejador{Servicio, Vinculos}`: la composición (repositorio → servicio →
  manejador) ocurre una sola vez en `nuevoEnrutador`. Las pruebas inyectan dobles
  de servicio o de repositorio, sin abrir PostgreSQL.
- Los paquetes de negocio no importan `net/http` (salvo los `manejadores.go`).
  En particular, `internal/reservas/servicio.go` no importa `gorm`.

## 3. Máquina de estados de la reserva

Vive en `internal/reservas/modelos.go`:

- `estadosValidos`: `pendiente`, `pago_por_verificar`, `confirmada`,
  `jugada`, `cancelada`.
- `transicionesValidas`:
  - `pendiente` → `pago_por_verificar`, `cancelada`
  - `pago_por_verificar` → `confirmada`, `cancelada`
  - `confirmada` → `jugada`, `cancelada`
  - `jugada` y `cancelada` son terminales.
- `transicionPermitida(de, a)` es una función pura.
- `NuevoPagoSena(reservaID, precioHora, metodo, comprobante)` construye el
  pago de seña con la regla de negocio del 30 % de `PrecioHora`.

El manejador `cambiarEstado` solo traduce HTTP; `Servicio.CambiarEstado` valida
el estado, comprueba la propiedad y aplica la transición. La prueba
`internal/reservas/modelos_test.go` fija la tabla completa y el cálculo de la
seña.

## 4. Autenticación y autorización

- `internal/sesion` guarda las cuentas en su propia tabla (`cuentas`), con
  `ClaveHash` en bcrypt y `json:"-"`.
- `POST /login` es público y devuelve únicamente `{"token": "..."}`.
- `Tokens.Autenticar` responde 401 si falta el token, está vencido o no es
  válido.
- `SoloRol` responde 403 si el rol no corresponde. `PATCH /reservas/{id}/estado`
  se registra con `soloAdmin`.
- La propiedad del recurso se comprueba dentro del servicio (`puedeAcceder`, 403
  si la reserva no pertenece a una cancha del administrador).

## 5. Pruebas

### Unitarias (sin base de datos)

```bash
go test ./... -count=1
```

Las pruebas de `internal/*/servicio_test.go` usan un repositorio falso; las de
`manejadores_test.go`, dobles de servicio y `ResolutorVinculos`. Cubren
autenticación, roles, propiedad, validación de datos y transiciones.

### Integración HTTP/PostgreSQL

Archivo `main_integracion_test.go`, etiqueta `//go:build integration`.
Levanta un `httptest.Server` con el mismo router de `main.go`, usa PostgreSQL
real, crea una reserva nueva por API y recorre el guion de
`docs/hito2_hoja.md` (login, 401, listado, 403, 200 y 422). Después comprueba
la transición válida `pendiente → pago_por_verificar` (`200`) y la seña del
30 %.

```bash
# Base desechable: la prueba borra y vuelve a sembrar sus tablas.
docker compose exec -T db createdb -U postgres canchas_test

TEST_DATABASE_URL="host=localhost user=postgres password=postgres dbname=canchas_test port=5432 sslmode=disable" \
  go test -tags=integration . -run TestAPIExtremoAExtremo -v -count=1
```

Sin `TEST_DATABASE_URL` la prueba se omite y `go test ./...` sigue funcionando
sin Docker, como en GitHub Actions.

## 6. Cambios recientes que hay que documentar

1. `internal/partidos/repositorio.go` incorpora `PartidoPorID` y
   `ContarInscripciones`; `GET /partidos/{id}` ya no usa `m.DB`.
2. `main.go` extrae `prepararBase(db, reset)` y `nuevoEnrutador(db, tokens)`
   para que `main()` y la prueba de integración ejecuten el mismo esquema y
   las mismas siembras.
3. `internal/reservas/modelos_test.go` fija la máquina de estados.
4. `main_integracion_test.go` añade la prueba de extremo a extremo.
5. La capa de servicio (`internal/*/servicio.go`) separa el transporte HTTP de
   las reglas de negocio, y el adaptador GORM de reservas vive en
   `internal/almacen/reservas.go`.
