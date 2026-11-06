@echo off
setlocal enabledelayedexpansion

echo =======================================================
echo          CRYBABY CLUSTER - 1-CLICK NODE INSTALLER      
echo =======================================================
echo.

:: 1. Check Administrator Privileges & Auto-Elevate (UAC)
net session >nul 2>&1
if %errorLevel% neq 0 (
    echo [ELEVASI] Meminta hak Administrator (UAC)...
    powershell -NoProfile -ExecutionPolicy Bypass -Command "Start-Process -FilePath '%~f0' -Verb RunAs"
    exit /b
)

:: Pastikan working directory adalah folder tempat script berada (meskipun di-elevate UAC)
cd /d "%~dp0"

echo [1/5] Memeriksa file installer...
if not exist "%~dp0crybaby-agent.exe" (
    echo [WARNING] crybaby-agent.exe tidak ditemukan di folder lokal.
    echo Mencoba mengunduh langsung dari Controller...
    set "CONTROLLER_URL="
    if exist "%~dp0crybaby-config.json" (
        for /f "tokens=2 delims=:, " %%A in ('findstr /i "controller_url" "%~dp0crybaby-config.json"') do (
            set "CONTROLLER_URL=%%~A"
        )
    )
    if "!CONTROLLER_URL!"=="" (
        set /p CONTROLLER_URL="Masukkan URL Controller (contoh http://192.168.100.168:8080): "
    )
    echo Mengunduh crybaby-agent.exe dari !CONTROLLER_URL!/api/v1/binaries/crybaby-agent.exe ...
    powershell -NoProfile -ExecutionPolicy Bypass -Command "Invoke-WebRequest -Uri '!CONTROLLER_URL!/api/v1/binaries/crybaby-agent.exe' -OutFile '%~dp0crybaby-agent.exe' -UseBasicParsing"
    if not exist "%~dp0crybaby-agent.exe" (
        echo [ERROR] Gagal mengunduh crybaby-agent.exe! Periksa koneksi ke Controller.
        pause
        exit /b 1
    )
)
if not exist "%~dp0crybaby-config.json" (
    echo [WARNING] crybaby-config.json tidak ditemukan.
    set /p CTRL_HOST="Masukkan IP:Port Controller (contoh 192.168.100.168:8080): "
    powershell -NoProfile -ExecutionPolicy Bypass -Command "(Invoke-WebRequest -Uri 'http://!CTRL_HOST!/api/v1/provision/usb-profile' -UseBasicParsing).Content | Out-File -Encoding utf8 '%~dp0crybaby-config.json'"
)

echo [2/5] Memasang Crybaby Agent ke %%ProgramFiles%%\Crybaby...
set "TARGET_DIR=%ProgramFiles%\Crybaby"
if not exist "%TARGET_DIR%" mkdir "%TARGET_DIR%"

copy /y "%~dp0crybaby-agent.exe" "%TARGET_DIR%\" >nul
copy /y "%~dp0crybaby-config.json" "%TARGET_DIR%\" >nul

echo [3/6] Mengatur skema daya Windows (Anti-Sleep & Anti-Accidental Shutdown)...
:: Cegah PC masuk mode Sleep / Hibernasi saat dicolok listrik
powercfg -change -standby-timeout-ac 0 >nul 2>&1
powercfg -change -hibernate-timeout-ac 0 >nul 2>&1
powercfg -change -monitor-timeout-ac 15 >nul 2>&1

:: Proteksi Anti-Shutdown: Tombol fisik power casing diatur ke Do Nothing (0)
:: Mencegah PC mati jika tombol power di casing tertekan tidak sengaja
powercfg -setacvalueindex SCHEME_CURRENT SUB_BUTTONS PBUTTONACTION 0 >nul 2>&1
powercfg -setactive SCHEME_CURRENT >nul 2>&1

echo [4/6] Mengonfigurasi Windows Firewall & Profil Jaringan LAN...
:: Ubah profil koneksi aktif menjadi Private agar Windows tidak memblokir komunikasi LAN
powershell -NoProfile -ExecutionPolicy Bypass -Command "Get-NetConnectionProfile | Set-NetConnectionProfile -NetworkCategory Private -ErrorAction SilentlyContinue" >nul 2>&1

:: Buka port RPC Agent (TCP 8081)
netsh advfirewall firewall delete rule name="Crybaby Agent RPC" >nul 2>&1
netsh advfirewall firewall add rule name="Crybaby Agent RPC" dir=in action=allow protocol=TCP localport=8081 profile=any description="Crybaby Cluster RPC & WebSocket" >nul 2>&1

:: Buka port WireGuard Mesh (UDP 51820)
netsh advfirewall firewall delete rule name="Crybaby WireGuard Mesh" >nul 2>&1
netsh advfirewall firewall add rule name="Crybaby WireGuard Mesh" dir=in action=allow protocol=UDP localport=51820 profile=any description="Crybaby P2P Mesh Fabric" >nul 2>&1

:: Izinkan ICMPv4 (Ping) dari jaringan lokal
netsh advfirewall firewall set rule name="File and Printer Sharing (Echo Request - ICMPv4-In)" new enable=yes >nul 2>&1

echo [5/6] Mengatur Kebijakan Windows Update (Anti-Auto Reboot)...
:: Mencegah Windows 11 melakukan restart paksa otomatis saat update terunduh
reg add "HKLM\SOFTWARE\Policies\Microsoft\Windows\WindowsUpdate\AU" /v NoAutoRebootWithLoggedOnUsers /t REG_DWORD /d 1 /f >nul 2>&1
reg add "HKLM\SOFTWARE\Policies\Microsoft\Windows\WindowsUpdate\AU" /v AUOptions /t REG_DWORD /d 3 /f >nul 2>&1
reg add "HKLM\SOFTWARE\Policies\Microsoft\Windows\WindowsUpdate\AU" /v AlwaysAutoRebootAtScheduledTime /t REG_DWORD /d 0 /f >nul 2>&1

echo [6/6] Mendaftarkan Windows Service (CrybabyAgent) & Memeriksa Hyper-V...
sc stop CrybabyAgent >nul 2>&1
sc delete CrybabyAgent >nul 2>&1
timeout /t 1 /nobreak >nul

sc create CrybabyAgent binPath= "\"%TARGET_DIR%\crybaby-agent.exe\" -config \"%TARGET_DIR%\crybaby-config.json\"" start= auto DisplayName= "Crybaby Cluster Agent" >nul
sc description CrybabyAgent "Crybaby Distributed Windows 11 Cluster & Hyper-V Manager Service" >nul
sc failure CrybabyAgent reset= 60 actions= restart/5000/restart/10000/restart/30000 >nul
sc start CrybabyAgent >nul

if exist "%~dp0enable-hyperv.ps1" (
    powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0enable-hyperv.ps1"
) else (
    powershell -NoProfile -ExecutionPolicy Bypass -Command "try { $f = Get-WindowsOptionalFeature -Online -FeatureName Microsoft-Hyper-V -ErrorAction SilentlyContinue; if ($f -and $f.State -ne 'Enabled') { Enable-WindowsOptionalFeature -Online -FeatureName Microsoft-Hyper-V -All -NoRestart -ErrorAction SilentlyContinue } } catch {}"
)

echo.
echo =======================================================
echo [SUCCESS] PC ini resmi bergabung ke Crybaby Cluster!
echo - Service terpasang otomatis berjalan saat Windows booting.
echo - Flashdisk ini SEKARANG BISA DICABUT dengan aman.
echo =======================================================
echo.
pause
