# Arquitectura del servidor

Documento de referencia para el equipo (Jorge incluido). Describe la
estructura real de carpetas, las dependencias entre capas y dónde vive cada
regla de negocio.

## 1. Estructura real de carpetas

```text
ServA-Aragundi-Castro/
├── main.go                      # arranque: config, conexión, migrate/siembra, router
├── main_integracion_test.go     # prueba E2E con `//go:build integration` (PostgreSQL real)
├── internal/
│   ├── canchas/                 # Cancha, Horario
│   │   ├── modelos.go
│   │   ├── repositorio.go       # interfaz + implementación GORM
│   │   ├── manejadores.go       # HTTP
│   │   ├── manejadores_test.go
│   │   └── semilla.go
│   ├── config/                  # variables de entorno (.env)
│   │   ├── config.go
│   │   └── config_test.go
│   ├── inscripcion/             # Inscripcion a un partido
│   │   ├── modelos.go
│   │   ├── repositorio.go
│   │   ├── manejadores.go
│   │   ├── manejadores_test.go
│   │   └── semilla.go
│   ├── middleware/              # Registro y Recuperacion
│   │   └── middleware.go
│   ├── partidos/                # Partido
│   │   ├── modelos.go
│   │   ├── repositorio.go
│   │   ├── manejadores.go
│   │   ├── manejadores_test.go
│   │   └── semilla.go
│   ├── reservas/                # Reserva + Pago + máquina de estados
│   │   ├── modelos.go           # estados y transiciones (sin HTTP)
│   │   ├── modelos_test.go      # prueba pura de la tabla de transiciones
│   │   ├── repositorio.go
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
        ├── interfaz Repositorio          (internal/*/repositorio.go)
        │     └── implementación GORM     (misma carpeta)
        └── sesion.ResolutorVinculos      (internal/sesion/vinculo.go)
              └── implementación GORM
```

- **El manejador HTTP no ejecuta consultas directamente.** Solo conoce la
  interfaz `Repositorio` y el `ResolutorVinculos`; las consultas GORM viven en
  `repositorio.go`.
- `Manejador.DB` solo se usa como valor por defecto en las fábricas
  `m.repositorio()` y `m.vinculos()`. Las pruebas inyectan dobles con los
  campos `Repo` y `Vinculos`, sin abrir PostgreSQL.
- Los paquetes de negocio no importan `net/http` (salvo los `manejadores.go`).
  En particular, `internal/reservas/modelos.go` no depende de HTTP.

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

El handler `cambiarEstado` solo la invoca después de autenticar y autorizar.
La prueba `internal/reservas/modelos_test.go` fija la tabla completa.

## 4. Autenticación y autorización

- `internal/sesion` guarda las cuentas en su propia tabla (`cuentas`), con
  `ClaveHash` en bcrypt y `json:"-"`.
- `POST /login` es público y devuelve únicamente `{"token": "..."}`.
- `Tokens.Autenticar` responde 401 si falta el token, está vencido o no es
  válido.
- `SoloRol` responde 403 si el rol no corresponde. `PATCH /reservas/{id}/estado`
  se registra con `soloAdmin`.
- La propiedad del recurso se comprueba dentro del manejador (403 si la
  reserva no pertenece a una cancha del administrador).

## 5. Pruebas

### Unitarias (sin base de datos)

```bash
go test ./... -count=1
```

Usan dobles de `Repositorio` y `ResolutorVinculos`. Cubren autenticación,
roles, propiedad, validación de datos y transiciones.

### Integración HTTP/PostgreSQL

Archivo `main_integracion_test.go`, etiqueta `//go:build integration`.
Levanta un `httptest.Server` con el mismo router de `main.go`, usa PostgreSQL
real y recorre el guion de `docs/hito2_hoja.md` (login, 401, listado, 403,
200 y 422).

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
