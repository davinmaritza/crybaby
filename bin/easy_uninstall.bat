@echo off
:: ============================================================
:: Easy Uninstaller - CryBaby Agent Windows Service
:: ============================================================

net session >nul 2>&1
if %errorlevel% neq 0 (
    echo [ERROR] Script ini harus dijalankan sebagai Administrator!
    echo Klik kanan file ini lalu pilih "Run as administrator".
    pause
    exit /b 1
)

set "TARGET_DIR=C:\ProgramData\CryBaby"
set "SERVICE_NAME=CryBabyAgent"

echo ============================================================
echo   Mencopot CryBaby Agent Service
echo ============================================================
echo.

echo [1/3] Menghentikan service %SERVICE_NAME%...
sc stop %SERVICE_NAME% >nul 2>&1
timeout /t 2 /nobreak >nul

echo [2/3] Menghapus Windows Service...
sc delete %SERVICE_NAME% >nul 2>&1

echo [3/3] Menghapus folder %TARGET_DIR%...
if exist "%TARGET_DIR%" (
    rmdir /S /Q "%TARGET_DIR%" >nul 2>&1
)

echo.
echo ============================================================
echo  PEMPCOPOTAN SELESAI SUKSES!
echo ============================================================
pause
