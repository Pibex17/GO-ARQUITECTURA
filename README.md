# Sistema de reservas de canchas de fútbol sintéticas y organización de partidos

Servidor HTTP en Go (Chi + GORM/PostgreSQL) para los Hitos 1 y 2 de Aplicaciones Web II. Incluye autenticación con JWT, roles, propiedad de recursos y el contrato OpenAPI 3.1 de todos los endpoints implementados.

## Arranque rápido (Windows)

Doble clic en **`iniciar.bat`**. El script hace todo solo:

1. Levanta PostgreSQL en Docker (`docker compose up -d db`).
2. Espera a que la base acepte conexiones.
3. Compila y arranca el servidor en <http://localhost:8080>.

Deje la ventana abierta mientras usa el servidor; `Ctrl+C` lo detiene. Para apagar también la base, doble clic en **`detener.bat`** (los datos se conservan).

Si no existe `.env`, `iniciar.bat` lo crea copiando `.env.example`. **Genere una clave aleatoria propia de al menos 32 bytes y péguela en `CLAVE_FIRMA` dentro de `.env`.** El servidor rechaza valores vacíos, compuestos solo por espacios, marcadores como `CAMBIE_ESTO` y claves de menos de 32 bytes. No incluya una clave real en `.env.example` ni en la documentación.

Si necesita recrear las tablas y los datos de ejemplo:

```bat
iniciar.bat reset
```

Requisitos: Docker Desktop iniciado y Go instalado.

## Cómo arrancar sin Docker

```bash
go mod tidy
cp .env.example .env        # complete DATABASE_URL y genere CLAVE_FIRMA (32+ bytes)
psql -U postgres -c "CREATE DATABASE canchas;"
go run . -reset             # primera vez: crea las tablas y siembra datos de ejemplo
```

Debe imprimir `escuchando en el puerto 8080`. En arranques posteriores, sin `-reset`.

Variables de entorno:

| Variable | Uso | Valor por defecto |
| --- | --- | --- |
| `DATABASE_URL` | Cadena de conexión a PostgreSQL. Incluye la contraseña; es un secreto. | Sin valor por defecto |
| `CLAVE_FIRMA` | Secreto con el que se firman los JWT HS256. Mínimo 32 bytes; no se aceptan marcadores de ejemplo. | Sin valor por defecto |
| `PUERTO` | Puerto HTTP. | `8080` |
| `TIEMPO_ESPERA_SEGUNDOS` | `ReadTimeout` y `WriteTimeout` del servidor. | `5` |

## Cómo arrancar con Docker

No hace falta tener Go ni PostgreSQL instalados en la máquina: Docker arma los dos contenedores.

```bash
docker compose up --build
```

`docker compose` lee el archivo `.env` de la raíz para sustituir `CLAVE_FIRMA`; defina ahí una clave aleatoria propia de al menos 32 bytes o pásela en el entorno antes de levantar los contenedores. La primera vez (o si borran el volumen `datos_postgres`), en otra terminal, para crear las tablas y sembrar los datos de ejemplo:

```bash
docker compose run --rm app -reset
```

Dentro de Docker, `DATABASE_URL` apunta a `host=db` (el nombre del servicio en `docker-compose.yml`), no a `localhost`; ese valor se define ahí mismo, no en el `.env` que usan para correr sin Docker. El resto del comportamiento —rutas, validaciones, envoltura de respuestas— es exactamente el mismo con o sin Docker: es la misma imagen la que corre.

Para bajar todo (y borrar los datos):

```bash
docker compose down -v
```

## Pruebas

```bash
go test ./... -v -count=1
go build ./...
go vet ./...
go mod verify
```

Las pruebas unitarias de servicios y manejadores usan dobles deterministas y no necesitan PostgreSQL. Las de `internal/*/servicio_test.go` prueban las reglas de negocio con repositorios falsos; las de `internal/*/manejadores_test.go` prueban el transporte HTTP.

La prueba de extremo a extremo está en `main_integracion_test.go` y lleva la etiqueta `//go:build integration`. Usa PostgreSQL real y una base desechable:

```bash
docker compose exec -T db createdb -U postgres canchas_test

TEST_DATABASE_URL="host=localhost user=postgres password=postgres dbname=canchas_test port=5432 sslmode=disable" \
  go test -tags=integration . -run TestAPIExtremoAExtremo -v -count=1
```

`prepararBase` borra y vuelve a sembrar las tablas de `canchas_test`; no apunte `TEST_DATABASE_URL` a la base de trabajo. Sin esa variable, la prueba se omite y `go test ./...` sigue funcionando sin Docker.

## Documentación de la API

- **`docs/openapi.yaml`**: contrato OpenAPI 3.1, fuente de verdad de la documentación.
- **`docs/api.html`**: página estática generada desde el contrato con Redoc 3. Se puede abrir directamente con doble clic: el contrato y el bundle de Redoc están embebidos y no hay dependencias de CDN ni de un servidor local.

Validación y regeneración:

```bash
npx @redocly/cli@2.71.0 lint docs/openapi.yaml
npx @redocly/cli@2.71.0 build-docs docs/openapi.yaml -o docs/api.html --inlineBundle
```

En PowerShell, si la política de ejecución bloquea `npx.ps1`, use `cmd /c "npx ..."`. `docs/api.html` es un archivo generado: para cambiar la documentación edite `docs/openapi.yaml` y vuelva a ejecutar el comando de `build-docs`.

El contrato documenta los **11 endpoints implementados**:

| Método | Ruta | Quién puede | Éxito | Errores reales |
| --- | --- | --- | --- | --- |
| POST | `/login` | Público | `200` con `datos.token` (JWT HS256, 1 hora) | `400 cuerpo_malformado` · `401 credenciales_invalidas` · `500 error_interno` |
| GET | `/canchas` | Cualquier sesión | `200` lista; cliente ve activas, administrador ve las propias | `400 id_invalido` · `401` · `403 permiso_denegado` / `cuenta_sin_vinculo` · `500 error_base` |
| POST | `/canchas` | Administrador | `201` cancha creada y activa | `400 cuerpo_malformado` · `401` · `403` · `422 datos_invalidos` / `tipo_invalido` · `500` |
| PATCH | `/canchas/{id}` | Administrador dueño | `200` cancha actualizada | `400 id_invalido` / `cuerpo_malformado` · `401` · `403` · `404 no_encontrado` · `422 tipo_invalido` · `500` |
| POST | `/reservas` | Cliente | `201` reserva en estado `pendiente` | `400 cuerpo_malformado` · `401` · `403` · `409 horario_ocupado` · `422 datos_invalidos` / `cancha_invalida` · `500` |
| GET | `/reservas` | Cualquier sesión | `200` reservas propias (cliente) o de sus canchas (administrador) | `400 id_invalido` · `401` · `403` · `500` |
| GET | `/reservas/{id}` | Cualquier sesión, con propiedad | `200` reserva con sus pagos | `400 id_invalido` · `401` · `403` · `404 no_encontrado` · `500` |
| PATCH | `/reservas/{id}/estado` | Administrador dueño | `200` reserva con el nuevo estado | `400 id_invalido` / `cuerpo_malformado` · `401` · `403` · `404 no_encontrado` · `422 estado_invalido` / `transicion_invalida` · `500` |
| POST | `/partidos` | Cliente dueño de la reserva | `201` partido creado con código de invitación | `400 cuerpo_malformado` · `401` · `403` · `404 no_encontrado` · `409 partido_existente` · `422 datos_invalidos` / `reserva_no_confirmada` · `500` |
| GET | `/partidos/{id}` | Cualquier token válido | `200` `{partido, inscriptos, cupos_restantes}` | `400 id_invalido` · `401` · `404 no_encontrado` · `500` |
| POST | `/partidos/{id}/inscripciones` | Cliente | `201` inscripción creada | `400 id_invalido` / `cuerpo_malformado` · `401` · `403` · `404 no_encontrado` · `409 ya_inscripto` / `partido_lleno` · `422 datos_invalidos` / `codigo_invalido` · `500` |
| * | Ruta inexistente | — | — | `404 ruta_inexistente` |
| * | Método no permitido en una ruta existente | — | — | `405 metodo_no_permitido` |

Notas que el contrato refleja porque son el comportamiento real del servidor:

- Los modelos se serializan en **PascalCase** (`CanchaID`, `ClienteID`, `HoraInicio`, `PrecioHora`, `Pagos`, etc.) porque no tienen etiquetas `json`. La solicitud de login usa `usuario` y `clave`; el detalle de partido usa `partido`, `inscriptos` y `cupos_restantes`.
- `PATCH /reservas/{id}/estado` es **solo para administrador**. Una transición prohibida responde `422 transicion_invalida`; no es `400`.
- `POST /reservas` no acepta `requiere_comprobante`: ese campo no existe en el servidor actual.
- Las fechas y horas son cadenas no vacías. El servidor no valida formato, calendario ni solapamiento por `HoraFin`; el conflicto se comprueba por `CanchaID`, `Fecha` y `HoraInicio`.
- Al pasar una reserva a `pago_por_verificar` se crea una seña del 30 % de `PrecioHora`; al pasar a `confirmada` se marca la seña pendiente como verificada; al pasar a `cancelada` no se modifican los pagos.
- `GET /partidos/{id}` solo exige un token válido: no resuelve el vínculo de negocio ni aplica propiedad.

## Estructura

```text
.
├── main.go                     # configuración, conexión, migrate/siembra y servidor
├── main_integracion_test.go    # prueba E2E con //go:build integration
├── go.mod / go.sum
├── iniciar.bat                 # arranque rápido en Windows (base + servidor)
├── detener.bat                 # apaga la base de datos
├── .env.example                # plantilla; sí se sube
├── .env                        # valores reales; NO se sube (está en .gitignore)
├── .github/workflows/          # CI de pruebas
├── Dockerfile
├── docker-compose.yml
├── .dockerignore
├── redocly.yaml                # reglas de validación del contrato
├── docs/
│   ├── openapi.yaml            # contrato OpenAPI 3.1
│   ├── api.html                # Redoc 3 generado, autocontenido
│   ├── arquitectura.md         # capas, dependencias y ubicación de las reglas
│   ├── decisiones.md           # decisiones de diseño (D1–D6)
│   ├── hito2_hoja.md           # guion de demostración y cumplimiento
│   ├── hito2_bruno/            # colección Bruno y capturas
│   ├── hito1_addendum.md
│   └── hito1_ficha_del_negocio.md
└── internal/
    ├── respuesta/              # envoltura única {"ok": ...}
    ├── middleware/             # Registro y Recuperacion
    ├── config/                 # lee .env y variables del sistema
    ├── usuarios/               # Usuario (cliente/administrador)
    ├── sesion/                 # login, JWT, roles y vínculos de negocio
    ├── canchas/                # Cancha, Horario + servicio, repositorio y HTTP
    ├── reservas/               # Reserva, Pagos, máquina de estados y seña
    ├── partidos/               # Partido y detalle con cupos
    └── inscripcion/            # Inscripción a partidos
```

## Declaración de IA

Usamos Claude (Anthropic) para redactar el esqueleto completo del servidor (paquetes `respuesta`, `middleware`, `config`, `usuarios`, `canchas`, `reservas`, `main.go` y las pruebas), siguiendo la estructura enseñada en los talleres de las semanas 2 a 4, adaptada a nuestro negocio. Las reglas de negocio (seña del 30 %, máquina de estados, validaciones propias) fueron decisiones nuestras, discutidas antes de escribir el código.

En el Hito 2 se usó IA como apoyo para revisar la autenticación JWT, la autorización por rol y propiedad, los dobles de prueba y el contrato OpenAPI. Las decisiones de alcance y los cambios se revisaron contra el código que realmente se ejecuta; los ejemplos de la documentación son ficticios y no contienen tokens ni contraseñas reales.

En la revisión final del Hito 2 se usó DeepSeek V4.1 Flash como apoyo para contrastar la lista de comprobación, revisar la documentación y proponer cambios. Cada propuesta se validó ejecutando las pruebas y revisando el diff; las decisiones de negocio y la entrega siguen siendo responsabilidad del equipo.
