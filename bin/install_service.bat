@echo off
:: Ensure Administrator Privileges
net session >nul 2>&1
if %errorlevel% neq 0 (
    echo [ERROR] Script ini harus dijalankan sebagai Administrator!
    echo Silakan klik kanan file ini lalu pilih "Run as administrator".
    pause
    exit /b 1
)

echo ========================================================
echo        Pemasangan CryBaby Agent sebagai Windows Service
echo ========================================================
echo.

:: Path default menggunakan direktori saat ini
set "AGENT_PATH=%~dp0agent.exe"

echo Path agent saat ini: %AGENT_PATH%
echo.
set /p USER_PATH="Masukkan path agent.exe (Tekan ENTER untuk menggunakan path default di atas): "

if not "%USER_PATH%"=="" (
    set "AGENT_PATH=%USER_PATH%"
)

if not exist "%AGENT_PATH%" (
    echo [ERROR] File agent.exe tidak ditemukan di: %AGENT_PATH%
    pause
    exit /b 1
)

echo.
echo Mendaftarkan Windows Service 'CryBabyAgent'...
sc create CryBabyAgent binPath= "\"%AGENT_PATH%\"" start= auto displayname= "CryBaby RMM Agent Service"

if %errorlevel% equ 0 (
    echo [OK] Service berhasil didaftarkan!
    echo Menjalankan Service...
    sc start CryBabyAgent
    echo.
    echo Pemasangan selesai. Service dapat dikelola via services.msc.
) else (
    echo [ERROR] Gagal mendaftarkan service.
)

pause
