# Sistema de reservas de canchas de fútbol sintéticas y organización de partidos.

Servidor HTTP en Go (Chi + GORM/PostgreSQL) para el Hito 1 de Aplicaciones Web II.

## Arranque rápido (Windows)

Doble clic en **`iniciar.bat`**. El script hace todo solo:

1. Levanta PostgreSQL en Docker (`docker compose up -d db`).
2. Espera a que la base acepte conexiones.
3. Compila y arranca el servidor en <http://localhost:8080>.

Deje la ventana abierta mientras usa el servidor; `Ctrl+C` lo detiene. Para apagar también la base, doble clic en **`detener.bat`** (los datos se conservan).

Si necesita recrear las tablas y los datos de ejemplo:

```bat
iniciar.bat reset
```

Requisitos: Docker Desktop iniciado y Go instalado.

## Cómo arrancar

```bash
go mod tidy
cp .env.example .env        # y complete la contraseña real de su PostgreSQL
psql -U postgres -c "CREATE DATABASE canchas;"
go run . -reset             # primera vez: crea las tablas y siembra datos de ejemplo
```

Debe imprimir `escuchando en el puerto 8080`. En arranques posteriores, sin `-reset`.

## Cómo arrancar con Docker

No hace falta tener Go ni PostgreSQL instalados en la máquina: Docker arma los dos contenedores.

```bash
docker compose up --build
```

La primera vez (o si borran el volumen `datos_postgres`), en otra terminal, para crear las tablas y sembrar los datos de ejemplo:

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
go test ./... -v
```

## Estructura

```
.
├── main.go                     # conexión, AutoMigrate, semillas, rutas
├── go.mod / go.sum
├── iniciar.bat                  # arranque rápido en Windows (base + servidor)
├── detener.bat                  # apaga la base de datos
├── .env.example                 # sí se sube
├── .env                          # NO se sube (está en .gitignore)
├── .github/workflows/pruebas.yml
├── Dockerfile
├── docker-compose.yml
├── .dockerignore
├── docs/decisiones.md
└── internal/
    ├── respuesta/        # envoltura única {"ok": ...}
    ├── middleware/       # Registro y Recuperacion
    ├── config/           # lee .env
    ├── usuarios/         # Usuario (sin rutas propias todavía)
    ├── canchas/          # Cancha, Horario + CRUD del catálogo
    └── reservas/         # Reserva (máquina de estados), Pago + pruebas
```

## Rutas

| Método | Ruta | Éxito | Errores posibles |
| --- | --- | --- | --- |
| GET | /canchas | 200 catálogo de canchas activas (o `?administrador_id=` para ver las propias) | 500 error_base |
| POST | /canchas | 201 cancha creada | 400 cuerpo_malformado · 422 datos_invalidos · 422 tipo_invalido · 422 administrador_invalido |
| PATCH | /canchas/{id} | 200 cancha actualizada | 400 id_invalido · 400 cuerpo_malformado · 404 no_encontrado · 422 tipo_invalido |
| POST | /reservas | 201 reserva creada, en estado `pendiente` | 400 cuerpo_malformado · 422 datos_invalidos · 422 cancha_invalida · 422 cliente_invalido · 409 horario_ocupado |
| GET | /reservas | 200 lista (con `?estado=` y `?cliente_id=`) | 500 error_base |
| PATCH | /reservas/{id}/estado | 200 reserva con su nuevo estado | 400 id_invalido · 400 cuerpo_malformado · 422 estado_invalido · 404 no_encontrado · 400 transicion_invalida |
| * | ruta inexistente | — | 404 ruta_inexistente |
| * | método no soportado | — | 405 metodo_no_permitido |

## Declaración de IA

Usamos Claude (Anthropic) para redactar el esqueleto completo del servidor (paquetes `respuesta`, `middleware`, `config`, `usuarios`, `canchas`, `reservas`, `main.go` y las pruebas), siguiendo la estructura enseñada en los talleres de las semanas 2 a 4, adaptada a nuestro negocio. Las reglas de negocio (seña del 30 %, máquina de estados, validaciones propias) fueron decisiones nuestras, discutidas antes de escribir el código.
