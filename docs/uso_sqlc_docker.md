# Uso de sqlc con Docker

**Herramienta:** sqlc `v1.31.1`
**Imagen:** `sqlc/sqlc:1.31.1`
**Servicio de Compose:** `sqlc` (perfil `tools`)

## ¿Por qué usar Docker para sqlc?

Para que todo el equipo genere exactamente el mismo código con la misma
versión de sqlc, sin instalar nada en su sistema operativo. La versión queda
fijada en `docker-compose.yml`; nadie depende de la versión que tenga
instalada localmente.

La imagen oficial `sqlc/sqlc:1.31.1` ya incluye el binario. La primera vez que
se ejecute, Docker la descarga; después queda en caché.

## Generar el código

Desde la raíz del proyecto:

```bash
docker compose run --rm sqlc
```

Ese comando:

- Inicia un contenedor temporal con `sqlc/sqlc:1.31.1`.
- Monta el proyecto completo en `/src`.
- Ejecuta `sqlc generate` usando `sqlc.yaml`.
- Escribe el resultado en `internal/almacen/filas/`.
- Elimina el contenedor al terminar (`--rm`).

No necesita que PostgreSQL esté iniciado: sqlc trabaja con `db/schema.sql` y
`db/consultas.sql`, que son archivos locales.

## Comprobar la versión

```bash
docker compose run --rm sqlc version
```

Salida esperada:

```text
v1.31.1
```

También se puede comprobar sin pasar por Compose:

```bash
docker run --rm sqlc/sqlc:1.31.1 version
```

Y para ver la imagen exacta que usaría el servicio:

```bash
docker compose --profile tools config --images
```

Debe aparecer `sqlc/sqlc:1.31.1`.

## Ejecutar PostgreSQL y la aplicación

El servicio `sqlc` usa el perfil `tools`, así que **no se inicia** con los
comandos normales:

```bash
# Base de datos y aplicación juntas
docker compose up --build

# Solo PostgreSQL, en segundo plano
docker compose up -d db

# Solo la aplicación (con la base ya levantada)
docker compose up -d app

# Atajos de Windows que ya existían
iniciar.bat
detener.bat
```

`docker compose up` no arranca `sqlc`; solo se ejecuta cuando se le llama
explícitamente con `docker compose run`.

## Flujo de trabajo del equipo

1. Obtener los últimos cambios de GitHub:

   ```bash
   git pull
   ```

2. Modificar `db/schema.sql` o `db/consultas.sql` cuando corresponda.

3. Generar el código con Docker:

   ```bash
   docker compose run --rm sqlc
   ```

4. Revisar los archivos generados en `internal/almacen/filas/`.

5. Ejecutar las pruebas Go:

   ```bash
   go build ./...
   go test ./... -count=1
   ```

6. Subir a GitHub tanto los archivos SQL como el código generado:

   ```bash
   git add db/schema.sql db/consultas.sql internal/almacen/filas/
   git commit -m "Actualizar consultas sqlc"
   git push
   ```

## Archivos que deben subirse a GitHub

- `docker-compose.yml` (contiene el servicio `sqlc` con la versión fijada).
- `docs/taller10/uso_sqlc_docker.md` (esta guía).
- Los cambios en `db/schema.sql` y `db/consultas.sql`, si los hubo.
- El código generado en `internal/almacen/filas/`:
  - `db.go`
  - `models.go`
  - `consultas.sql.go`

El código generado se versiona para que quien clone el repositorio pueda
compilar sin ejecutar sqlc primero.

## Notas

- **No se usa `latest`.** La versión está fijada en `sqlc/sqlc:1.31.1` para
  que todos generen lo mismo.
- **No hace falta PostgreSQL.** El servicio `sqlc` no tiene `depends_on` ni
  conexión a la base.
- **La instalación local de sqlc no se elimina.** Si alguien ya la tiene,
  `sqlc generate` sigue funcionando; Docker es la opción común para el equipo.
- **Linux:** si los archivos generados quedan con propietario `root`, se
  pueden recuperar con:

  ```bash
  sudo chown -R "$(id -u):$(id -g)" internal/almacen/filas
  ```

## Problemas comunes

| Síntoma | Causa y solución |
| --- | --- |
| `no such service: sqlc` | Ejecutar el comando desde la raíz del proyecto, donde está `docker-compose.yml`. |
| `Cannot connect to the Docker daemon` | Docker Desktop o el motor de Docker no está iniciado. |
| La descarga de la imagen falla | No hay conexión con Docker Hub; reintentar cuando haya red. La imagen solo se descarga la primera vez. |
| `docker compose up` no muestra `sqlc` | Es correcto: el servicio está en el perfil `tools` y solo corre con `docker compose run`. |
