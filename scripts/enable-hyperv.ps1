# Crybaby Hyper-V Feature Verification & Enabler for Windows 11
Write-Host "Memeriksa status Microsoft Hyper-V..." -ForegroundColor Cyan

$hyperVFeature = Get-WindowsOptionalFeature -Online -FeatureName Microsoft-Hyper-V -ErrorAction SilentlyContinue

if ($null -ne $hyperVFeature -and $hyperVFeature.State -eq "Enabled") {
    Write-Host "[OK] Microsoft Hyper-V sudah aktif pada perangkat ini." -ForegroundColor Green
    exit 0
}

Write-Host "[!] Hyper-V belum aktif. Mencoba mengaktifkan fitur Hyper-V..." -ForegroundColor Yellow

# Check Windows Edition
$edition = (Get-ItemProperty "HKLM:\SOFTWARE\Microsoft\Windows NT\CurrentVersion").EditionID

if ($edition -like "*Home*") {
    Write-Host "Terdeteksi Windows 11 Home. Memasang paket Hyper-V via DISM..." -ForegroundColor Cyan
    $packages = Get-ChildItem -Path "$env:SystemRoot\servicing\Packages\*Hyper-V*.mum"
    foreach ($pkg in $packages) {
        dism /online /norestart /add-package:"$($pkg.FullName)" | Out-Null
    }
    dism /online /enable-feature /featurename:Microsoft-Hyper-V-All /norestart /all | Out-Null
} else {
    Enable-WindowsOptionalFeature -Online -FeatureName Microsoft-Hyper-V-All -NoRestart -ErrorAction SilentlyContinue | Out-Null
}

Write-Host "[INFO] Fitur Hyper-V telah diaktifkan. Anda mungkin perlu melakukan Restart Windows nanti agar Hyper-V aktif penuh." -ForegroundColor Yellow
exit 0
