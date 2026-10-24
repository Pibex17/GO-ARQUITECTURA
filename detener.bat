@echo off
chcp 65001 >nul
cd /d "%~dp0"

REM Detiene PostgreSQL. Los datos NO se borran: quedan en el volumen
REM datos_postgres y vuelven a estar disponibles al ejecutar iniciar.bat.
REM
REM El servidor Go se detiene aparte, con Ctrl+C en su propia ventana.

echo Deteniendo PostgreSQL...
docker compose stop db
echo.
echo Listo. Los datos se conservan.
echo Recuerda cerrar la ventana del servidor con Ctrl+C si sigue abierta.
echo.
pause
