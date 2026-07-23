@echo off
:: Script ini mendaftarkan 'crybaby' sebagai perintah global di CMD Windows
:: Jalankan sebagai Administrator

net session >nul 2>&1
if %errorlevel% neq 0 (
    echo [ERROR] Jalankan script ini sebagai Administrator!
    pause
    exit /b 1
)

set "CLI_PATH=%~dp0cyrbaby-cli.exe"

if not exist "%CLI_PATH%" (
    echo [ERROR] File cyrbaby-cli.exe tidak ditemukan di: %CLI_PATH%
    pause
    exit /b 1
)

:: Tambahkan folder bin ke PATH Windows secara permanen (System-wide)
for /f "tokens=2*" %%A in ('reg query "HKLM\SYSTEM\CurrentControlSet\Control\Session Manager\Environment" /v Path 2^>nul') do set "SYS_PATH=%%B"

:: Cek apakah folder sudah ada di PATH
echo %SYS_PATH% | findstr /i "%~dp0" >nul 2>&1
if %errorlevel% equ 0 (
    echo [INFO] Folder sudah ada di PATH. Tidak perlu diubah.
) else (
    :: Tambahkan ke System PATH
    setx /M PATH "%SYS_PATH%;%~dp0"
    echo [OK] Folder %~dp0 berhasil ditambahkan ke System PATH!
)

echo.
echo ==================================================
echo  Instalasi selesai!
echo  Buka CMD baru lalu ketik: crybaby
echo ==================================================
pause
