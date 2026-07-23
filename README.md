# 🤖 CryBaby RMM — by Suzirz

Aplikasi **Remote Monitoring & Management (RMM)** modern yang dirancang untuk mengelola puluhan hingga ratusan PC secara terpusat. Dilengkapi dengan **Web Dashboard Proxmox-Style**, **Telegram Bot**, **Hermes-Style CLI (`crybaby`)**, serta **Cluster AI Lokal (Ollama)** yang bisa membagi beban komputasi AI secara otomatis ke PC-PC dalam jaringan Anda.

---

## 🌟 Kenapa CryBaby RMM?

- 🖥 **Web Dashboard Proxmox Dark Theme**: Pantau CPU, RAM, Disk, dan Uptime semua PC dalam satu layar. Tersedia juga fitur Shell Terminal & File Manager jarak jauh.
- 📱 **Telegram Bot Control Center**: Eksekusi perintah shell (`/exec`), broadcast command ke semua PC (`/broadcast`), approve PC baru, hingga chat AI langsung dari Telegram.
- 💻 **CLI Interaktif Ala Hermes (`crybaby`)**: Buka CMD di mana saja, ketik `crybaby`, dan langsung kelola fleet Anda lengkap dengan ASCII art banner & AI assistant.
- ⚙️ **Windows Service Native**: Sekali install via `easy_install.bat`, agent otomatis berjalan di latar belakang (`C:\ProgramData\CryBaby`) tanpa jendela CMD dan otomatis menyala saat PC di-boot.
- ⚡ **Multi-Node AI Cluster (Round-Robin)**: Menghubungkan Ollama dari PC-PC client Anda ke backend. Beban komputasi AI dibagi secara otomatis (Load Balancing) sehingga VPS Pterodactyl Anda tetap ringan 0% load.
- 📦 **Dukungan Instalasi Portable / Flashdisk**: Salin file installer & model AI ke flashdisk untuk instalasi cepat tanpa perlu unduh ulang di tiap PC.

---

## 🏗 Arsitektur Sistem

```text
 ┌─────────────────────────────────────────────────────────────┐
 │                Telegram Bot / Web UI / CLI                  │
 └──────────────────────────────┬──────────────────────────────┘
                                │
 ┌──────────────────────────────▼──────────────────────────────┐
 │             CryBaby Backend Server (Pterodactyl)            │
 │               (Listen: 25583 / WebSocket: /ws)              │
 │           - Round-Robin AI Load Balancer Engine -           │
 └──────┬───────────────────────┬───────────────────────┬──────┘
        │                       │                       │
 ┌──────▼──────┐         ┌──────▼──────┐         ┌──────▼──────┐
 │ Agent Node 1│         │ Agent Node 2│   ...   │ Agent Node N│
 │ (Ollama AI) │         │ (Ollama AI) │         │ (Ollama AI) │
 └─────────────┘         └─────────────┘         └─────────────┘
```

---

## 🚀 Panduan Pemasangan Singkat

### 1. Pemasangan Backend (Di Pterodactyl / Linux VPS)

1. Upload file `backend-linux` dari folder `bin/` ke root directory Pterodactyl Anda.
2. Buat file `config.json` di Pterodactyl dengan isi seperti ini:
```json
{
  "port": "PORT_SERVER_ANDA",
  "db_path": "crybaby.db",
  "admin_password": "YOUR_ADMIN_PASSWORD",
  "telegram_token": "YOUR_TELEGRAM_BOT_TOKEN",
  "telegram_admin_ids": [123456789],
  "ollama_url": "",
  "ollama_model": "llama3",
  "allowed_admin_users": null
}
```
3. Restart server Pterodactyl. Backend akan mendengarkan di port `25583`.

---

### 2. Pemasangan Agent di PC Client (Windows)

#### Cara Cepat (1 Klik):
1. Salin file `agent.exe` dan `easy_install.bat` dari folder `bin/` ke PC target (bisa lewat Flashdisk).
2. Klik kanan **`easy_install.bat`** → pilih **Run as administrator**.
3. Buka Telegram Bot Anda, ketik `/pending` lalu `/approve <UUID_PC>` untuk mengizinkan PC tersebut terhubung.

---

### 3. Pemasangan Node AI (Ollama)

Untuk PC yang ingin dijadikan server pemroses AI:

#### Cara 1: Pakai Script Installer (Online)
Buka PowerShell di PC target (Run as Admin):
```powershell
irm https://ollama.com/install.ps1 | iex
ollama pull llama3
```

#### Cara 2: Trik Portable (Offline via Flashdisk)
Jika tidak ingin unduh ulang di tiap PC:
1. Salin file `ollama.exe` (dari `C:\Users\USERNAME\AppData\Local\Programs\Ollama\ollama.exe`).
2. Salin folder `.ollama` (dari `C:\Users\USERNAME\.ollama`).
3. Paste folder `.ollama` ke `C:\Users\USERNAME\.ollama` di PC target.
4. Jalankan `ollama.exe serve` di PC target.

---

### 4. Aktivasi CLI Client (`crybaby`) di PC Admin

1. Buka folder `bin/`, klik kanan **`setup_path.bat`** → **Run as administrator** (cukup 1 kali).
2. Buka CMD baru di mana saja, ketik:
   ```cmd
   crybaby
   ```

---

## 📜 Daftar Perintah Slash (Telegram / CLI / Web)

| Perintah | Fungsi |
|---|---|
| `/status` | Melihat rangkuman kondisi fleet (Server Online/Offline, Total CPU & RAM) |
| `/list` | Menampilkan daftar seluruh server dan metriknya |
| `/pending` | Menampilkan PC baru yang menunggu persetujuan admin |
| `/approve <UUID>` | Menyetujui akses PC baru ke sistem |
| `/reject <UUID>` | Menolak PC baru |
| `/remove <UUID>` | Menghentikan & mencopot agent dari PC target |
| `/info <UUID>` | Informasi detail spesifikasi hardware & statistik PC |
| `/rename <UUID> <NAMA>` | Memberikan nama kustom untuk PC target |
| `/exec <UUID> <CMD>` | Menjalankan perintah shell di PC tertentu |
| `/broadcast <CMD>` | Menjalankan perintah shell di **SEMUA** PC yang online bersamaan |
| `/clusters` | Menampilkan daftar cluster |
| `/cluster-create <NAMA>` | Membuat cluster PC baru |
| `/logs` | Menampilkan 20 riwayat audit log terakhir |
| *(Chat biasa)* | Berbicara langsung dengan **Local AI Agent (Ollama)** |

---

## 🛠 Lisensi & Kredit

Dibuat & Dikembangkan oleh **Suzirz**  
CryBaby RMM Architecture v1.0.0 — All rights reserved.
