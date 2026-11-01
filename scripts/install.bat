@echo off
setlocal enabledelayedexpansion

echo =======================================================
echo          CRYBABY CLUSTER - 1-CLICK NODE INSTALLER      
echo =======================================================
echo.

:: 1. Check Administrator Privileges
net session >nul 2>&1
if %errorLevel% neq 0 (
    echo [ERROR] Harap jalankan installer ini sebagai Administrator!
    echo Klik kanan install.bat -> "Run as administrator"
    pause
    exit /b 1
)

echo [1/5] Memeriksa file installer...
if not exist "%~dp0crybaby-agent.exe" (
    echo [ERROR] crybaby-agent.exe tidak ditemukan di folder flashdisk ini!
    pause
    exit /b 1
)
if not exist "%~dp0crybaby-config.json" (
    echo [ERROR] crybaby-config.json tidak ditemukan di folder flashdisk ini!
    pause
    exit /b 1
)

echo [2/5] Memasang Crybaby Agent ke %%ProgramFiles%%\Crybaby...
set "TARGET_DIR=%ProgramFiles%\Crybaby"
if not exist "%TARGET_DIR%" mkdir "%TARGET_DIR%"

copy /y "%~dp0crybaby-agent.exe" "%TARGET_DIR%\" >nul
copy /y "%~dp0crybaby-config.json" "%TARGET_DIR%\" >nul

echo [3/5] Mengatur skema daya Windows (Anti-Sleep Policy)...
:: Cegah PC masuk mode Sleep / Hibernasi saat dicolok listrik
powercfg -change -standby-timeout-ac 0 >nul 2>&1
powercfg -change -hibernate-timeout-ac 0 >nul 2>&1
powercfg -change -monitor-timeout-ac 15 >nul 2>&1

echo [4/5] Mendaftarkan Windows Service (CrybabyAgent)...
sc stop CrybabyAgent >nul 2>&1
sc delete CrybabyAgent >nul 2>&1
timeout /t 1 /nobreak >nul

sc create CrybabyAgent binPath= "\"%TARGET_DIR%\crybaby-agent.exe\" -config \"%TARGET_DIR%\crybaby-config.json\"" start= auto DisplayName= "Crybaby Cluster Agent" >nul
sc description CrybabyAgent "Crybaby Distributed Windows 11 Cluster & Hyper-V Manager Service" >nul
sc failure CrybabyAgent reset= 60 actions= restart/5000/restart/10000/restart/30000 >nul
sc start CrybabyAgent >nul

echo [5/5] Memeriksa status fitur Microsoft Hyper-V...
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0enable-hyperv.ps1"

echo.
echo =======================================================
echo [SUCCESS] PC ini resmi bergabung ke Crybaby Cluster!
echo - Service terpasang otomatis berjalan saat Windows booting.
echo - Flashdisk ini SEKARANG BISA DICABUT dengan aman.
echo =======================================================
echo.
pause
