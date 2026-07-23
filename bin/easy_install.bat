@echo off
:: ============================================================
:: Easy Installer - CryBaby Agent Windows Service
:: ============================================================

net session >nul 2>&1
if %errorlevel% neq 0 (
    echo [ERROR] Script ini harus dijalankan sebagai Administrator!
    echo Klik kanan file ini lalu pilih "Run as administrator".
    pause
    exit /b 1
)

set "TARGET_DIR=C:\ProgramData\CryBaby"
set "EXE_NAME=agent.exe"
set "SERVICE_NAME=CryBabyAgent"

echo ============================================================
echo   Memasang CryBaby Agent ke %TARGET_DIR%
echo ============================================================
echo.

:: 1. Buat direktori target
if not exist "%TARGET_DIR%" (
    mkdir "%TARGET_DIR%"
)

:: 2. Hentikan service lama jika ada
sc query %SERVICE_NAME% >nul 2>&1
if %errorlevel% equ 0 (
    echo [1/4] Menghentikan service lama...
    sc stop %SERVICE_NAME% >nul 2>&1
    timeout /t 2 /nobreak >nul
    sc delete %SERVICE_NAME% >nul 2>&1
    timeout /t 1 /nobreak >nul
)

:: 3. Salin binary ke C:\ProgramData\CryBaby
echo [2/4] Menyalin file ke %TARGET_DIR%...
copy /Y "%~dp0%EXE_NAME%" "%TARGET_DIR%\%EXE_NAME%" >nul
if %errorlevel% neq 0 (
    echo [ERROR] Gagal menyalin %EXE_NAME%. Pastikan file ada di folder yang sama dengan batch script ini.
    pause
    exit /b 1
)

:: 4. Daftarkan sebagai Windows Service
echo [3/4] Mendaftarkan Windows Service (%SERVICE_NAME%)...
sc create %SERVICE_NAME% binPath= "\"%TARGET_DIR%\%EXE_NAME%\"" start= auto DisplayName= "CryBaby RMM Agent" >nul
if %errorlevel% neq 0 (
    echo [ERROR] Gagal membuat Windows Service.
    pause
    exit /b 1
)

:: Set deskripsi service
sc description %SERVICE_NAME% "Remote Monitoring & Management Agent by Suzirz" >nul

:: 5. Jalankan Service
echo [4/4] Menjalankan Service...
sc start %SERVICE_NAME% >nul
if %errorlevel% neq 0 (
    echo [WARNING] Service berhasil terpasang tetapi belum dapat di-start otomatis.
) else (
    echo [OK] Service berhasil dijalankan di latar belakang!
)

echo.
echo ============================================================
echo  PEMASANGAN SELESAI SUKSES!
echo.
echo  • Lokasi File : %TARGET_DIR%\%EXE_NAME%
echo  • Mode        : Windows Service (Otostart & Background)
echo  • Pengelolaan : Buka 'services.msc' (Nama: CryBaby RMM Agent)
echo ============================================================
pause
