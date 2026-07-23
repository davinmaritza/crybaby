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
echo        Pencopotan CryBaby Agent Windows Service
echo ========================================================
echo.

echo Menghentikan Service CryBabyAgent...
sc stop CryBabyAgent >nul 2>&1

echo Menghapus Service CryBabyAgent...
sc delete CryBabyAgent

if %errorlevel% equ 0 (
    echo [OK] Service berhasil dihapus secara bersih dari sistem!
) else (
    echo [ERROR] Gagal menghapus service atau service tidak ditemukan.
)

pause
