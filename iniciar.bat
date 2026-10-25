@echo off
chcp 65001 >nul
setlocal EnableExtensions EnableDelayedExpansion
cd /d "%~dp0"

REM ------------------------------------------------------------------
REM  Arranca el servidor completo con un doble clic.
REM    - Levanta PostgreSQL en Docker si no está corriendo.
REM    - Espera a que la base acepte conexiones.
REM    - Compila y arranca el servidor Go en el puerto 8080.
REM
REM  Uso:  iniciar.bat          arranque normal
REM        iniciar.bat reset    borra las tablas y vuelve a sembrar
REM ------------------------------------------------------------------

set "ARGS="
if /i "%~1"=="reset" set "ARGS=-reset"

echo ==========================================================
echo   Sistema de reservas - canchas sinteticas
echo ==========================================================
echo.

REM --- 1. El puerto 8080 debe estar libre ------------------------------
netstat -ano | findstr /R /C:":8080 " | findstr /C:"LISTENING" >nul 2>&1
if not errorlevel 1 (
    echo [ERROR] El puerto 8080 ya esta ocupado.
    echo         Lo mas probable es que el servidor ya este corriendo
    echo         en otra ventana. Cierrala con Ctrl+C y vuelve a intentar.
    echo.
    pause
    exit /b 1
)

REM --- 2. Docker debe estar instalado y corriendo ----------------------
where docker >nul 2>&1
if errorlevel 1 (
    echo [ERROR] No se encontro Docker.
    echo         Instala Docker Desktop, o levanta PostgreSQL a mano y
    echo         ejecuta "go run ." directamente.
    echo.
    pause
    exit /b 1
)

docker info >nul 2>&1
if errorlevel 1 (
    echo [ERROR] Docker Desktop no esta corriendo.
    echo         Abrelo, espera a que el motor termine de iniciar
    echo         y vuelve a ejecutar este archivo.
    echo.
    pause
    exit /b 1
)

REM --- 3. Base de datos ------------------------------------------------
echo [1/3] Levantando PostgreSQL...
docker compose up -d db
if errorlevel 1 (
    echo.
    echo [ERROR] No se pudo levantar la base de datos.
    echo         Revisa el detalle con: docker compose logs db
    echo.
    pause
    exit /b 1
)

echo [2/3] Esperando a que la base acepte conexiones...
set /a INTENTOS=0
:ESPERAR
docker compose exec -T db pg_isready -U postgres -d canchas >nul 2>&1
if not errorlevel 1 goto BASE_LISTA
set /a INTENTOS+=1
if !INTENTOS! geq 30 (
    echo.
    echo [ERROR] La base no respondio despues de 60 segundos.
    echo         Revisa el detalle con: docker compose logs db
    echo.
    pause
    exit /b 1
)
timeout /t 2 /nobreak >nul
goto ESPERAR
:BASE_LISTA
echo         Base lista.

REM --- 4. Archivo .env -------------------------------------------------
if not exist ".env" (
    echo [AVISO] No existe .env; se crea a partir de .env.example.
    copy ".env.example" ".env" >nul
    echo [AVISO] Edite .env y defina CLAVE_FIRMA con una clave aleatoria propia de al menos 32 bytes.
    echo         No use CAMBIE_ESTO ni valores de ejemplo; el servidor los rechaza.
)

REM --- 5. Go debe estar instalado --------------------------------------
where go >nul 2>&1
if errorlevel 1 (
    echo.
    echo [ERROR] No se encontro Go en el PATH.
    echo         Instala Go desde https://go.dev/dl/ y vuelve a intentar.
    echo.
    pause
    exit /b 1
)

REM --- 6. Servidor -----------------------------------------------------
echo [3/3] Compilando y arrancando el servidor...
echo.
echo         http://localhost:8080
echo.
echo         Deja esta ventana abierta. Ctrl+C para detener el servidor.
echo ==========================================================
echo.

go run . %ARGS%

echo.
echo El servidor se detuvo.
echo Los datos de la base se conservan. Para apagarla del todo,
echo ejecuta detener.bat.
echo.
pause
