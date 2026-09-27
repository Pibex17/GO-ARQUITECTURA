# Decisiones

## D1 · Orden de los middleware
**Decisión:** Registro afuera, Recuperación adentro (`r.Use(middleware.Registro); r.Use(middleware.Recuperacion)`).
**Por qué:** si el orden fuera al revés, un pánico en un handler saltaría por encima de Registro antes de que este alcance a escribir la línea del log, y perderíamos el rastro de qué petición reventó.
**Alternativa descartada:** Recuperación afuera, Registro adentro — el pánico nunca llegaría a Registro y el log nunca mostraría el 500.
**Cómo lo comprobamos:** una ruta que fuerza un pánico (por ejemplo, dividir por índice fuera de rango dentro de un handler de prueba); en el log aparece la línea con el 500 antes de que el proceso siga vivo para la siguiente petición.

## D2 · ¿Qué pasa si se intenta borrar una cancha con reservas?
**Decisión:** no implementamos borrado físico de canchas: solo `Activa = false` desde `PATCH /canchas/{id}`. Una cancha con reservas nunca se puede eliminar de la base.
**Por qué:** en nuestro negocio el historial de reservas y pagos de una cancha tiene valor para el administrador (cuánto facturó, qué días se usa más), y borrar la cancha borraría ese historial o rompería la clave foránea de sus reservas.
**Alternativa descartada:** `DELETE /canchas/{id}` con `db.Delete`, que en PostgreSQL respondería `violates foreign key constraint` en cuanto la cancha tuviera una sola reserva — un error de base de datos disfrazado de operación normal del negocio.

## D3 · La prueba que no escribimos
**Prueba que no escribimos:** que crear una reserva para un horario ya ocupado responde `409` (`horario_ocupado`), y que cambiar el estado de una reserva a un estado no permitido por la Sección 5 responde `400` (`transicion_invalida`) cuando la reserva sí existe.
**Dónde empieza la base de datos:** en `internal/reservas/manejadores.go`, dentro de `crear`, en la línea marcada `// --- desde aquí hace falta la base de datos ---` (justo antes de `m.DB.First(&cancha, ...)`); y dentro de `cambiarEstado`, en la misma marca, justo antes de `m.DB.First(&reserva, id)`.
**Qué haría falta para probarla:** una base de datos real (o sqlite en memoria) con canchas, usuarios y reservas ya sembradas, para poder crear una segunda reserva que choque con la primera y comprobar el 409, y para poder traer una reserva ya `jugada` y comprobar que no se puede pasar a `cancelada`.

## D4 · Cómo resuelve el servidor la dirección de la base dentro de Docker
**Decisión:** `DATABASE_URL` con `host=db` se define directamente en `docker-compose.yml`, no en el `.env` del repositorio. El `.env` que se usa para correr sin Docker sigue con `host=localhost`.
**Por qué:** el nombre de host de PostgreSQL depende de si el servidor corre dentro de la red de Docker (ahí "localhost" sería el propio contenedor del servidor, no el de la base) o directamente en la máquina. Es un valor de infraestructura, no un secreto ni una regla de negocio, así que vive en el archivo que describe esa infraestructura.
**Alternativa descartada:** un único `.env` con `host=db` siempre, que habría roto `go run .` fuera de Docker para cualquiera que no use Docker Compose.
