@echo off
setlocal enabledelayedexpansion

echo =======================================================
echo          CRYBABY CLUSTER - 1-CLICK UNINSTALLER         
echo =======================================================
echo.

:: 1. Check Administrator Privileges & Auto-Elevate (UAC)
net session >nul 2>&1
if %errorLevel% neq 0 (
    echo [ELEVASI] Meminta hak Administrator (UAC)...
    powershell -NoProfile -ExecutionPolicy Bypass -Command "Start-Process -FilePath '%~f0' -Verb RunAs"
    exit /b
)

:: Pastikan working directory adalah folder tempat script berada
cd /d "%~dp0"

echo [1/5] Menghentikan dan menghapus Daemon (CrybabyAgent)...
taskkill /f /im crybaby-agent.exe >nul 2>&1
sc stop CrybabyAgent >nul 2>&1
sc delete CrybabyAgent >nul 2>&1
schtasks /delete /tn CrybabyAgent /f >nul 2>&1
reg delete "HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\Run" /v CrybabyAgent /f >nul 2>&1

echo [2/5] Menghapus aturan Windows Firewall...
netsh advfirewall firewall delete rule name="Crybaby Agent RPC" >nul 2>&1
netsh advfirewall firewall delete rule name="Crybaby WireGuard Mesh" >nul 2>&1

echo [3/5] Mengembalikan pengaturan daya dan tombol power casing...
:: Kembalikan fungsi tombol power fisik casing ke normal (1 = Shutdown)
powercfg -setacvalueindex SCHEME_CURRENT SUB_BUTTONS PBUTTONACTION 1 >nul 2>&1
powercfg -setactive SCHEME_CURRENT >nul 2>&1

:: Kembalikan kebijakan Windows Update
reg delete "HKLM\SOFTWARE\Policies\Microsoft\Windows\WindowsUpdate\AU" /v NoAutoRebootWithLoggedOnUsers /f >nul 2>&1
reg delete "HKLM\SOFTWARE\Policies\Microsoft\Windows\WindowsUpdate\AU" /v AlwaysAutoRebootAtScheduledTime /f >nul 2>&1

echo [4/5] Menghapus file instalasi di %%ProgramFiles%%\Crybaby...
set "TARGET_DIR=%ProgramFiles%\Crybaby"
if exist "%TARGET_DIR%" (
    rmdir /s /q "%TARGET_DIR%" >nul 2>&1
)

echo [5/5] Pembersihan data disk VM...
set /p DEL_VMS="Hapus semua disk VM Hyper-V di C:\Crybaby? (Y/N, default: N): "
if /i "%DEL_VMS%"=="Y" (
    if exist "C:\Crybaby" (
        rmdir /s /q "C:\Crybaby" >nul 2>&1
        echo [OK] Direktori C:\Crybaby dan disk VM berhasil dihapus.
    )
) else (
    echo [INFO] Direktori C:\Crybaby dipertahankan.
)

echo.
echo =======================================================
echo [SUCCESS] Crybaby telah BERHASIL DIHAPUS dari PC ini!
echo Service, firewall, dan pengaturan daya telah kembali normal.
echo =======================================================
echo.
pause
