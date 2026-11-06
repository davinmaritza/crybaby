package main

import (
	"net/http"
	"strings"
)

func (s *Server) handleWebDashboard(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	html := strings.ReplaceAll(dashboardHTML, "{{CLUSTER_TOKEN}}", s.clusterToken)
	html = strings.ReplaceAll(html, "{{PORT}}", s.port)
	_, _ = w.Write([]byte(html))
}

const dashboardHTML = `<!DOCTYPE html>
<html lang="en" class="dark">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Crybaby | Windows 11 Cluster Orchestrator</title>
    <script src="https://cdn.tailwindcss.com"></script>
    <link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.4.0/css/all.min.css">
    <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/xterm@5.3.0/css/xterm.min.css">
    <script src="https://cdn.jsdelivr.net/npm/xterm@5.3.0/lib/xterm.min.js"></script>
    <script src="https://cdn.jsdelivr.net/npm/xterm-addon-fit@0.8.0/lib/xterm-addon-fit.min.js"></script>
    <script>
        tailwind.config = {
            darkMode: 'class',
            theme: {
                extend: {
                    colors: {
                        brand: {
                            50: '#eef2ff',
                            500: '#6366f1',
                            600: '#4f46e5',
                            900: '#312e81'
                        },
                        darkbg: '#0B0F19',
                        cardbg: '#111827',
                        borderbg: '#1F2937'
                    }
                }
            }
        }
    </script>
    <style>
        body { background-color: #0B0F19; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; }
        .glass { background: rgba(17, 24, 39, 0.7); backdrop-filter: blur(12px); border: 1px solid rgba(255, 255, 255, 0.08); }
        .pulse-dot { animation: pulse 2s cubic-bezier(0.4, 0, 0.6, 1) infinite; }
        @keyframes pulse { 0%, 100% { opacity: 1; } 50% { opacity: .4; } }
    </style>
</head>
<body class="text-gray-200 antialiased min-h-screen flex flex-col">
    <!-- Top Navigation -->
    <header class="border-b border-gray-800 glass sticky top-0 z-40 px-6 py-4 flex items-center justify-between">
        <div class="flex items-center space-x-3">
            <div class="w-9 h-9 rounded-xl bg-gradient-to-tr from-indigo-600 to-violet-500 flex items-center justify-center shadow-lg shadow-indigo-500/20">
                <i class="fa-solid fa-bolt text-white text-lg"></i>
            </div>
            <div>
                <h1 class="font-bold text-lg tracking-tight text-white flex items-center gap-2">
                    Crybaby Cluster
                    <span class="inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-xs font-medium bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">
                        <span class="w-1.5 h-1.5 rounded-full bg-emerald-400 pulse-dot"></span>
                        Mesh 10.42.0.1
                    </span>
                </h1>
                <p class="text-xs text-gray-400">Windows 11 Hyper-V Distributed Fabric</p>
            </div>
        </div>

        <div class="flex items-center gap-3">
            <button onclick="resetCluster()" title="Kosongkan semua node dan mulai fresh" class="px-3 py-1.5 rounded-lg bg-gray-900 hover:bg-red-500/10 text-gray-400 hover:text-red-400 border border-gray-800 hover:border-red-500/30 text-xs font-semibold flex items-center gap-1.5 transition">
                <i class="fa-solid fa-eraser"></i> Reset / Fresh
            </button>
            <button onclick="openISOCatalogModal()" title="Katalog ISO &amp; Download Installer OS" class="px-3.5 py-1.5 rounded-lg bg-gray-900 hover:bg-gray-800 text-gray-200 border border-gray-800 hover:border-gray-700 text-xs font-semibold flex items-center gap-2 transition">
                <i class="fa-solid fa-compact-disc text-indigo-400"></i>
                ISO Catalog
            </button>
            <button onclick="openUSBModal()" class="px-3.5 py-1.5 rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-semibold flex items-center gap-2 transition shadow-md shadow-indigo-600/20">
                <i class="fa-solid fa-plus-circle"></i>
                Tambah Node / Installer Server
            </button>
            <div class="h-6 w-px bg-gray-800"></div>
            <span id="wsStatus" class="text-xs font-mono px-2.5 py-1 rounded-md bg-gray-900 border border-gray-800 text-emerald-400">WS CONNECTED</span>
        </div>
    </header>

    <!-- Main Workspace -->
    <main class="flex-1 max-w-7xl w-full mx-auto p-6 space-y-6">
        <!-- Cluster Health Cards -->
        <div class="grid grid-cols-1 md:grid-cols-4 gap-4">
            <div class="glass p-5 rounded-xl border border-gray-800/80">
                <div class="flex justify-between items-start">
                    <div>
                        <p class="text-xs font-medium text-gray-400 uppercase tracking-wider">Node Fleet</p>
                        <h3 id="statNodes" class="text-2xl font-bold text-white mt-1">0</h3>
                    </div>
                    <div class="p-2.5 rounded-lg bg-indigo-500/10 text-indigo-400 border border-indigo-500/20">
                        <i class="fa-solid fa-server"></i>
                    </div>
                </div>
                <div class="mt-3 flex items-center text-xs text-gray-400">
                    <span id="statOnline" class="text-emerald-400 font-semibold mr-1">0 online</span> &bull; <span id="statOffline" class="text-gray-500 ml-1">0 offline</span>
                </div>
            </div>

            <div class="glass p-5 rounded-xl border border-gray-800/80">
                <div class="flex justify-between items-start">
                    <div>
                        <p class="text-xs font-medium text-gray-400 uppercase tracking-wider">Cluster CPU Load</p>
                        <h3 id="statCPU" class="text-2xl font-bold text-white mt-1">0.0%</h3>
                    </div>
                    <div class="p-2.5 rounded-lg bg-violet-500/10 text-violet-400 border border-violet-500/20">
                        <i class="fa-solid fa-microchip"></i>
                    </div>
                </div>
                <div class="w-full bg-gray-800 h-1.5 rounded-full mt-3 overflow-hidden">
                    <div id="barCPU" class="bg-indigo-500 h-full w-0 transition-all duration-500"></div>
                </div>
                <div class="mt-2.5 pt-2 border-t border-gray-800/50">
                    <div class="flex justify-between text-[10px] text-gray-400 mb-1">
                        <span>Trend (60s)</span>
                        <span id="cpuTrendVal" class="text-indigo-400 font-mono">0.0%</span>
                    </div>
                    <canvas id="cpuSparkline" class="w-full h-8 block rounded" height="32" width="220"></canvas>
                </div>
            </div>

            <div class="glass p-5 rounded-xl border border-gray-800/80">
                <div class="flex justify-between items-start">
                    <div>
                        <p class="text-xs font-medium text-gray-400 uppercase tracking-wider">Memory Allocated</p>
                        <h3 id="statRAM" class="text-2xl font-bold text-white mt-1">0.0 GB</h3>
                    </div>
                    <div class="p-2.5 rounded-lg bg-blue-500/10 text-blue-400 border border-blue-500/20">
                        <i class="fa-solid fa-memory"></i>
                    </div>
                </div>
                <div class="w-full bg-gray-800 h-1.5 rounded-full mt-3 overflow-hidden">
                    <div id="barRAM" class="bg-blue-500 h-full w-0 transition-all duration-500"></div>
                </div>
                <div class="mt-2.5 pt-2 border-t border-gray-800/50">
                    <div class="flex justify-between text-[10px] text-gray-400 mb-1">
                        <span>Trend (60s)</span>
                        <span id="ramTrendVal" class="text-blue-400 font-mono">0.0%</span>
                    </div>
                    <canvas id="ramSparkline" class="w-full h-8 block rounded" height="32" width="220"></canvas>
                </div>
            </div>

            <div class="glass p-5 rounded-xl border border-gray-800/80">
                <div class="flex justify-between items-start">
                    <div>
                        <p class="text-xs font-medium text-gray-400 uppercase tracking-wider">Hyper-V Virtual Machines</p>
                        <h3 id="statVMs" class="text-2xl font-bold text-white mt-1">0</h3>
                    </div>
                    <div class="p-2.5 rounded-lg bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">
                        <i class="fa-solid fa-cube"></i>
                    </div>
                </div>
                <div class="mt-3 flex items-center text-xs text-gray-400">
                    <span class="text-emerald-400 font-semibold mr-1">Active Hyper-V Fabric</span>
                </div>
            </div>
        </div>

        <!-- Node Fleet Table -->
        <div class="glass rounded-xl border border-gray-800/80 overflow-hidden shadow-2xl">
            <div class="px-6 py-4 border-b border-gray-800/80 flex items-center justify-between">
                <div>
                    <h2 class="font-semibold text-white text-base">Windows 11 Node Fleet</h2>
                    <p class="text-xs text-gray-400 mt-0.5">Physical workstations joined to the Crybaby mesh cluster</p>
                </div>
                <button onclick="fetchNodesManual()" class="px-3 py-1.5 rounded-lg bg-gray-800 hover:bg-gray-700 text-xs font-medium text-gray-300 transition">
                    <i class="fa-solid fa-arrows-rotate mr-1.5"></i> Refresh
                </button>
            </div>

            <div class="overflow-x-auto">
                <table class="w-full text-left text-sm">
                    <thead class="text-xs uppercase bg-gray-900/60 text-gray-400 border-b border-gray-800">
                        <tr>
                            <th class="px-6 py-3.5">Node Name</th>
                            <th class="px-6 py-3.5">Status</th>
                            <th class="px-6 py-3.5">Mesh & Physical IP</th>
                            <th class="px-6 py-3.5">Hardware / CPU</th>
                            <th class="px-6 py-3.5">Live CPU / RAM</th>
                            <th class="px-6 py-3.5">Hyper-V</th>
                            <th class="px-6 py-3.5 text-right">Actions</th>
                        </tr>
                    </thead>
                    <tbody id="nodeTableBody" class="divide-y divide-gray-800/50">
                        <tr>
                            <td colspan="7" class="px-6 py-12 text-center text-gray-500 text-sm">
                                <i class="fa-solid fa-satellite-dish text-2xl mb-3 block text-gray-600"></i>
                                Belum ada Windows 11 Node yang terhubung.<br>
                                Klik <b class="text-indigo-400">"Tambah Node / Installer Server"</b> di kanan atas untuk menghubungkan PC server pertama Anda.
                            </td>
                        </tr>
                    </tbody>
                </table>
            </div>
        </div>
    </main>

    <!-- Node Provisioning & Server Installer Modal -->
    <div id="usbModal" class="fixed inset-0 bg-black/75 backdrop-blur-sm z-50 flex items-center justify-center hidden p-4">
        <div class="glass max-w-xl w-full rounded-2xl border border-gray-700 p-6 space-y-4 shadow-2xl">
            <div class="flex justify-between items-center border-b border-gray-800 pb-3">
                <div>
                    <h3 class="font-bold text-white text-base flex items-center gap-2">
                        <i class="fa-solid fa-server text-indigo-400"></i>
                        Tambah Node / Server Installer
                    </h3>
                    <p class="text-xs text-gray-400 mt-0.5">Pasang dan hubungkan PC server fisik ke cluster Crybaby</p>
                </div>
                <button onclick="closeUSBModal()" class="text-gray-400 hover:text-white text-lg">&times;</button>
            </div>

            <!-- Tab Selectors -->
            <div class="flex bg-gray-950 p-1 rounded-xl border border-gray-800 text-xs">
                <button id="tabBtnPS1" onclick="switchInstallTab('ps1')" class="flex-1 py-2 rounded-lg font-semibold transition text-white bg-indigo-600 shadow-sm">
                    <i class="fa-brands fa-windows mr-1.5"></i> 1-Line PowerShell
                </button>
                <button id="tabBtnZip" onclick="switchInstallTab('zip')" class="flex-1 py-2 rounded-lg font-semibold transition text-gray-400 hover:text-gray-200">
                    <i class="fa-solid fa-file-zipper mr-1.5"></i> Offline ZIP / USB
                </button>
                <button id="tabBtnLinux" onclick="switchInstallTab('linux')" class="flex-1 py-2 rounded-lg font-semibold transition text-gray-400 hover:text-gray-200">
                    <i class="fa-brands fa-linux mr-1.5"></i> Linux Server
                </button>
            </div>

            <!-- Tab 1: 1-Line PowerShell (Recommended) -->
            <div id="tabContentPS1" class="space-y-3">
                <p class="text-xs text-gray-300 leading-relaxed">
                    Di PC Server Windows 11 target: Buka <b class="text-white">PowerShell</b>, tempel perintah di bawah dan tekan <b class="text-white">Enter</b>:
                </p>

                <div class="bg-gray-950 border border-gray-800 rounded-xl p-3 flex items-center justify-between gap-3">
                    <code id="ps1CommandText" class="font-mono text-xs text-indigo-300 select-all break-all">irm http://127.0.0.1:8080/install | iex</code>
                    <button onclick="copyCommand('ps1CommandText', this)" class="shrink-0 px-3 py-1.5 rounded-lg bg-indigo-600 hover:bg-indigo-500 text-xs font-semibold text-white flex items-center gap-1.5 transition">
                        <i class="fa-solid fa-copy"></i> Salin
                    </button>
                </div>

                <div class="bg-indigo-950/40 border border-indigo-500/20 rounded-xl p-3 text-xs text-indigo-200 space-y-1">
                    <div class="font-semibold text-indigo-300 flex items-center gap-1.5">
                        <i class="fa-solid fa-bolt text-indigo-400"></i> Otomatis &amp; Siap Pakai:
                    </div>
                    <ul class="list-disc list-inside text-[11px] text-gray-300 space-y-0.5">
                        <li>Meminta izin Administrator (UAC) otomatis jika belum elevated</li>
                        <li>Mengunduh biner <code class="text-indigo-300">crybaby-agent.exe</code> &amp; konfigurasi token cluster</li>
                        <li>Mengonfigurasi skema daya <b>Anti-Sleep</b> &amp; tombol power casing fisik (Server Mode)</li>
                        <li>Membuka port Windows Firewall (RPC TCP 8081, WireGuard UDP 51820, ICMP Ping)</li>
                        <li>Mengaktifkan <b>Microsoft Hyper-V</b> &amp; mendaftarkan service Windows background</li>
                    </ul>
                </div>

                <div class="text-[11px] text-gray-400 flex items-center gap-1">
                    <i class="fa-solid fa-keyboard text-gray-500"></i>
                    <span>Tips: Dari kotak dialog Run (<kbd class="px-1 py-0.5 bg-gray-800 rounded text-gray-300">Win + R</kbd>), ketik: <code class="text-indigo-400 font-mono">powershell -c "irm http://[IP]:8080/install | iex"</code></span>
                </div>
            </div>

            <!-- Tab 2: Offline ZIP / USB -->
            <div id="tabContentZip" class="space-y-3 hidden">
                <p class="text-xs text-gray-300 leading-relaxed">
                    Unduh paket installer offline ke Flashdisk atau download langsung di PC server target. Cukup <b class="text-white">Double-Click install.bat</b> (otomatis meminta Administrator UAC tanpa perlu klik kanan).
                </p>

                <div class="space-y-3 bg-gray-950 p-3.5 rounded-xl border border-gray-800 text-xs">
                    <div>
                        <label for="usbHostInput" class="block text-gray-300 font-semibold mb-1">Alamat IP Controller LAN:</label>
                        <input id="usbHostInput" oninput="updateInstallerHost(this.value)" type="text" autocomplete="off" placeholder="e.g. 192.168.100.168:8080" class="w-full bg-gray-900 border border-gray-700 focus:border-indigo-500 rounded-lg px-3 py-2 text-white font-mono text-xs">
                        <p class="text-[11px] text-indigo-400 mt-1"><i class="fa-solid fa-wand-magic-sparkles mr-1"></i>Otomatis mendeteksi interface IP LAN controller Anda.</p>
                    </div>
                    <div class="flex justify-between text-gray-400 font-mono text-xs pt-1 border-t border-gray-900">
                        <span>Cluster Join Token:</span>
                        <span class="text-indigo-400 select-all" id="modalToken">{{CLUSTER_TOKEN}}</span>
                    </div>
                </div>

                <div class="flex justify-end gap-2 pt-1">
                    <button onclick="downloadUSBPack()" class="w-full py-2.5 rounded-lg bg-indigo-600 hover:bg-indigo-500 text-xs font-semibold text-white flex items-center justify-center gap-2 transition shadow-md shadow-indigo-600/20">
                        <i class="fa-solid fa-download"></i> Unduh Paket Offline ZIP (.zip)
                    </button>
                </div>
            </div>

            <!-- Tab 3: Linux Server -->
            <div id="tabContentLinux" class="space-y-3 hidden">
                <p class="text-xs text-gray-300 leading-relaxed">
                    Untuk server Ubuntu/Debian/RHEL target, jalankan satu baris perintah ini di terminal:
                </p>

                <div class="bg-gray-950 border border-gray-800 rounded-xl p-3 flex items-center justify-between gap-3">
                    <code id="shCommandText" class="font-mono text-xs text-emerald-300 select-all break-all">curl -sSL http://127.0.0.1:8080/install.sh | sudo bash</code>
                    <button onclick="copyCommand('shCommandText', this)" class="shrink-0 px-3 py-1.5 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-xs font-semibold text-white flex items-center gap-1.5 transition">
                        <i class="fa-solid fa-copy"></i> Salin
                    </button>
                </div>

                <div class="bg-emerald-950/30 border border-emerald-500/20 rounded-xl p-3 text-xs text-emerald-200">
                    <i class="fa-solid fa-circle-check mr-1 text-emerald-400"></i>
                    Otomatis mengunduh biner Linux <code class="text-emerald-300">crybaby-agent-linux</code>, menyimpan config cluster di <code class="text-emerald-300">/etc/crybaby/</code>, dan mendaftarkan service daemon <code class="text-emerald-300">systemd</code>.
                </div>
            </div>

            <div class="flex justify-end border-t border-gray-800 pt-3">
                <button onclick="closeUSBModal()" class="px-4 py-2 rounded-lg bg-gray-800 hover:bg-gray-700 text-xs font-semibold text-gray-300">Tutup</button>
            </div>
        </div>
    </div>

    <!-- Hyper-V VM Management Modal -->
    <div id="vmModal" class="fixed inset-0 bg-black/75 backdrop-blur-sm z-50 flex items-center justify-center hidden p-4">
        <div class="glass max-w-4xl w-full rounded-2xl border border-gray-700 p-6 space-y-5 shadow-2xl max-h-[90vh] overflow-y-auto">
            <div class="flex justify-between items-center border-b border-gray-800 pb-3">
                <div>
                    <h3 class="font-bold text-white text-base flex items-center gap-2">
                        <i class="fa-solid fa-cube text-emerald-400"></i>
                        Hyper-V Virtual Machines &bull; <span id="vmNodeTitle" class="text-indigo-400">Node</span>
                    </h3>
                    <p class="text-xs text-gray-400">Manajemen mesin virtual Hyper-V lokal pada node ini</p>
                </div>
                <button onclick="closeVMModal()" class="text-gray-400 hover:text-white text-xl">&times;</button>
            </div>

            <!-- New VM Collapsible Form -->
            <div class="bg-gray-900/80 rounded-xl p-4 border border-gray-800 space-y-3">
                <div class="flex justify-between items-center cursor-pointer" onclick="toggleNewVMForm()">
                    <span class="text-xs font-bold text-gray-200 uppercase tracking-wider flex items-center gap-1.5">
                        <i class="fa-solid fa-plus text-emerald-400"></i> Buat VM Hyper-V Baru
                    </span>
                    <i id="newVMChevron" class="fa-solid fa-chevron-down text-gray-400 text-xs transition-transform"></i>
                </div>
                <form id="newVMForm" autocomplete="off" class="hidden space-y-3 pt-2" onsubmit="submitNewVM(event)">
                    <div class="grid grid-cols-1 md:grid-cols-2 gap-3 text-xs">
                        <div>
                            <label for="vmName" class="block text-gray-400 mb-1">Nama Virtual Machine</label>
                            <input id="vmName" name="name" autocomplete="off" required type="text" placeholder="e.g. win11-sandbox-01" class="w-full bg-gray-950 border border-gray-800 rounded-lg px-3 py-2 text-white font-mono focus:border-indigo-500 focus:outline-none">
                        </div>
                        <div>
                            <label for="vmSwitch" class="block text-gray-400 mb-1">Virtual Switch</label>
                            <select id="vmSwitch" name="switch_name" class="w-full bg-gray-950 border border-gray-800 rounded-lg px-3 py-2 text-white font-mono focus:border-indigo-500 focus:outline-none">
                                <option value="Default Switch">Default Switch</option>
                            </select>
                        </div>
                        <div>
                            <label for="vmCores" class="block text-gray-400 mb-1">vCPU Cores</label>
                            <input id="vmCores" name="cpu_count" type="number" min="1" max="64" value="2" class="w-full bg-gray-950 border border-gray-800 rounded-lg px-3 py-2 text-white font-mono focus:border-indigo-500 focus:outline-none">
                        </div>
                        <div>
                            <label for="vmRAM" class="block text-gray-400 mb-1">RAM (MB)</label>
                            <input id="vmRAM" name="memory_mb" type="number" min="512" step="512" value="4096" class="w-full bg-gray-950 border border-gray-800 rounded-lg px-3 py-2 text-white font-mono focus:border-indigo-500 focus:outline-none">
                        </div>
                        <div class="md:col-span-2">
                            <label for="vmDisk" class="block text-gray-400 mb-1">Ukuran Hard Disk VHDX (GB)</label>
                            <input id="vmDisk" name="disk_size_gb" type="number" min="10" max="2000" value="40" class="w-full bg-gray-950 border border-gray-800 rounded-lg px-3 py-2 text-white font-mono focus:border-indigo-500 focus:outline-none">
                        </div>
                        <div class="md:col-span-2">
                            <div class="flex justify-between items-center mb-1">
                                <label for="vmISOSelect" class="block text-gray-400">Pilih ISO Installer (ISO Repository Controller)</label>
                                <button type="button" onclick="openISOCatalogModal()" class="text-[11px] text-indigo-400 hover:text-indigo-300 hover:underline flex items-center gap-1">
                                    <i class="fa-solid fa-cloud-arrow-down"></i> Unduh ISO Baru dari URL
                                </button>
                            </div>
                            <select id="vmISOSelect" class="w-full bg-gray-950 border border-gray-800 rounded-lg px-3 py-2 text-white font-mono focus:border-indigo-500 focus:outline-none text-xs" onchange="syncISOInput(this.value)">
                                <option value="">-- Tanpa ISO (Boot dari VHDX kosong) --</option>
                            </select>
                            <input id="vmISO" name="iso_path" type="text" autocomplete="off" placeholder="Atau ketik custom path / URL (e.g. http://.../ubuntu.iso atau C:\ISO\win.iso)" class="w-full bg-gray-950/70 border border-gray-800 rounded-lg px-3 py-1.5 text-gray-300 font-mono focus:border-indigo-500 focus:outline-none text-xs mt-1.5">
                            <p class="text-[11px] text-gray-400 mt-1"><i class="fa-solid fa-bolt text-amber-400 mr-1"></i>Otomatis di-cache ke <code>C:\Crybaby\ISOs</code> pada Windows node jika menggunakan link HTTP.</p>
                        </div>
                    </div>
                    <div class="flex justify-end">
                        <button type="submit" class="px-4 py-2 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-white text-xs font-semibold flex items-center gap-2">
                            <i class="fa-solid fa-play"></i> Provision VM Sekarang
                        </button>
                    </div>
                </form>
            </div>

            <!-- Existing VM List Table -->
            <div class="overflow-x-auto">
                <table class="w-full text-left text-xs">
                    <thead class="uppercase bg-gray-950/80 text-gray-400 border-b border-gray-800">
                        <tr>
                            <th class="px-4 py-2.5">VM Name</th>
                            <th class="px-4 py-2.5">State</th>
                            <th class="px-4 py-2.5">CPU &amp; Cores</th>
                            <th class="px-4 py-2.5">RAM &amp; Demand</th>
                            <th class="px-4 py-2.5">IP Address</th>
                            <th class="px-4 py-2.5">VHDX Path</th>
                            <th class="px-4 py-2.5 text-right">Actions</th>
                        </tr>
                    </thead>
                    <tbody id="vmTableBody" class="divide-y divide-gray-800/40 font-mono">
                        <tr>
                            <td colspan="7" class="px-4 py-6 text-center text-gray-500">Memuat VM...</td>
                        </tr>
                    </tbody>
                </table>
            </div>

            <div class="flex justify-end pt-2 border-t border-gray-800">
                <button onclick="closeVMModal()" class="px-4 py-2 rounded-lg bg-gray-800 hover:bg-gray-700 text-xs font-semibold text-gray-300">Tutup</button>
            </div>
        </div>
    </div>

    <!-- Interactive Terminal Shell Modal -->
    <div id="terminalModal" class="fixed inset-0 bg-black/80 backdrop-blur-sm z-50 flex items-center justify-center hidden p-4">
        <div class="glass max-w-4xl w-full rounded-2xl border border-gray-700 p-5 space-y-3 shadow-2xl flex flex-col h-[75vh]">
            <div class="flex justify-between items-center border-b border-gray-800 pb-3">
                <div>
                    <h3 class="font-bold text-white text-base flex items-center gap-2">
                        <i class="fa-solid fa-terminal text-indigo-400"></i>
                        Remote Shell &bull; <span id="termNodeTitle" class="text-indigo-400">Node</span>
                    </h3>
                    <p class="text-xs text-gray-400">Direct interactive PTY terminal (PowerShell / Shell)</p>
                </div>
                <button onclick="closeTerminal()" class="text-gray-400 hover:text-white text-xl">&times;</button>
            </div>
            <div id="terminalContainer" class="flex-1 bg-black rounded-lg p-2 overflow-hidden border border-gray-800 font-mono"></div>
            <div class="flex justify-between items-center pt-2 border-t border-gray-800 text-xs text-gray-500">
                <span>Tekan <kbd class="px-1.5 py-0.5 bg-gray-800 rounded text-gray-300">Ctrl+C</kbd> atau ketik <code>exit</code> untuk keluar</span>
                <button onclick="closeTerminal()" class="px-4 py-1.5 rounded-lg bg-gray-800 hover:bg-gray-700 text-xs font-semibold text-gray-300">Tutup</button>
            </div>
        </div>
    </div>

    <!-- Snapshot / Checkpoint Modal -->
    <div id="snapshotModal" class="fixed inset-0 bg-black/80 backdrop-blur-sm z-50 flex items-center justify-center hidden p-4">
        <div class="glass max-w-2xl w-full rounded-2xl border border-gray-700 p-5 space-y-4 shadow-2xl">
            <div class="flex justify-between items-center border-b border-gray-800 pb-3">
                <div>
                    <h3 class="font-bold text-white text-base flex items-center gap-2">
                        <i class="fa-solid fa-camera text-purple-400"></i>
                        Snapshots / Checkpoints &bull; <span id="snapVMTitle" class="text-indigo-400">VM</span>
                    </h3>
                    <p class="text-xs text-gray-400">Kelola riwayat snapshot dan rollback VM ke kondisi sebelumnya</p>
                </div>
                <button onclick="closeSnapshotsModal()" class="text-gray-400 hover:text-white text-xl">&times;</button>
            </div>

            <!-- Create Snapshot Form -->
            <form id="createSnapshotForm" class="flex gap-2 text-xs" onsubmit="submitNewSnapshot(event)">
                <input id="snapNameInput" type="text" placeholder="Nama snapshot (misal: sebelum-update-db)" required class="flex-1 bg-gray-950 border border-gray-800 rounded-lg px-3 py-2 text-white font-mono focus:border-purple-500 focus:outline-none">
                <button type="submit" class="px-4 py-2 rounded-lg bg-purple-600 hover:bg-purple-500 text-white font-semibold flex items-center gap-1.5">
                    <i class="fa-solid fa-plus"></i> Buat Snapshot
                </button>
            </form>

            <!-- Snapshots Table -->
            <div class="overflow-x-auto max-h-60 overflow-y-auto">
                <table class="w-full text-left text-xs">
                    <thead class="uppercase bg-gray-950/80 text-gray-400 border-b border-gray-800">
                        <tr>
                            <th class="px-3 py-2">Nama Snapshot</th>
                            <th class="px-3 py-2">Dibuat Pada</th>
                            <th class="px-3 py-2 text-right">Aksi</th>
                        </tr>
                    </thead>
                    <tbody id="snapshotTableBody" class="divide-y divide-gray-800/40 font-mono">
                        <tr>
                            <td colspan="3" class="px-3 py-4 text-center text-gray-500">Memuat snapshot...</td>
                        </tr>
                    </tbody>
                </table>
            </div>

            <div class="flex justify-end pt-2 border-t border-gray-800">
                <button onclick="closeSnapshotsModal()" class="px-4 py-1.5 rounded-lg bg-gray-800 hover:bg-gray-700 text-xs font-semibold text-gray-300">Tutup</button>
            </div>
        </div>
    </div>

    <!-- ISO Catalog & Download Modal -->
    <div id="isoCatalogModal" class="fixed inset-0 bg-black/80 backdrop-blur-sm z-50 flex items-center justify-center hidden p-4">
        <div class="glass max-w-3xl w-full rounded-2xl border border-gray-700 p-5 space-y-4 shadow-2xl max-h-[85vh] overflow-y-auto">
            <div class="flex justify-between items-center border-b border-gray-800 pb-3">
                <div>
                    <h3 class="font-bold text-white text-base flex items-center gap-2">
                        <i class="fa-solid fa-compact-disc text-indigo-400"></i>
                        Controller ISO Repository &amp; Downloader
                    </h3>
                    <p class="text-xs text-gray-400">Penyimpanan ISO lokal controller untuk instalasi VM via LAN tanpa internet di node</p>
                </div>
                <button onclick="closeISOCatalogModal()" class="text-gray-400 hover:text-white text-xl">&times;</button>
            </div>

            <!-- Download ISO from URL form -->
            <form id="downloadISOForm" class="bg-gray-900/70 p-3.5 rounded-xl border border-gray-800 space-y-2.5 text-xs" onsubmit="submitDownloadISO(event)">
                <div class="font-semibold text-gray-200 flex items-center gap-1.5">
                    <i class="fa-solid fa-cloud-arrow-down text-indigo-400"></i> Unduh ISO dari URL Internet ke Controller:
                </div>
                <div class="grid grid-cols-1 md:grid-cols-3 gap-2">
                    <div class="md:col-span-2">
                        <input id="isoDownloadURL" type="url" required placeholder="https://releases.ubuntu.com/24.04/ubuntu-24.04-live-server.iso" class="w-full bg-gray-950 border border-gray-800 rounded-lg px-3 py-2 text-white font-mono focus:border-indigo-500 focus:outline-none">
                    </div>
                    <div>
                        <input id="isoDownloadFilename" type="text" placeholder="ubuntu-24.04.iso (opsional)" class="w-full bg-gray-950 border border-gray-800 rounded-lg px-3 py-2 text-white font-mono focus:border-indigo-500 focus:outline-none">
                    </div>
                </div>
                <div class="flex justify-between items-center pt-1">
                    <span class="text-[11px] text-gray-400"><i class="fa-solid fa-folder-open text-indigo-400 mr-1"></i>Tersimpan di direktori <code>./iso</code> controller</span>
                    <button type="submit" id="btnStartISODownload" class="px-3.5 py-1.5 rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white font-semibold flex items-center gap-1.5">
                        <i class="fa-solid fa-download"></i> Mulai Download ISO
                    </button>
                </div>
                <!-- Active Downloads Progress list -->
                <div id="isoDownloadsList" class="hidden space-y-2 pt-2 border-t border-gray-800/60 font-mono text-[11px]"></div>
            </form>

            <!-- ISO List Table -->
            <div class="overflow-x-auto">
                <table class="w-full text-left text-xs">
                    <thead class="uppercase bg-gray-950/80 text-gray-400 border-b border-gray-800">
                        <tr>
                            <th class="px-3 py-2">Nama Berkas ISO</th>
                            <th class="px-3 py-2">Ukuran</th>
                            <th class="px-3 py-2">Waktu Modifikasi</th>
                            <th class="px-3 py-2 text-right">Aksi</th>
                        </tr>
                    </thead>
                    <tbody id="isoTableBody" class="divide-y divide-gray-800/40 font-mono">
                        <tr>
                            <td colspan="4" class="px-3 py-4 text-center text-gray-500">Memuat berkas ISO...</td>
                        </tr>
                    </tbody>
                </table>
            </div>

            <div class="flex justify-end pt-2 border-t border-gray-800">
                <button onclick="closeISOCatalogModal()" class="px-4 py-1.5 rounded-lg bg-gray-800 hover:bg-gray-700 text-xs font-semibold text-gray-300">Tutup</button>
            </div>
        </div>
    </div>

    <!-- Resize VM Modal -->
    <div id="resizeVMModal" class="fixed inset-0 bg-black/80 backdrop-blur-sm z-50 flex items-center justify-center hidden p-4">
        <div class="glass max-w-md w-full rounded-2xl border border-gray-700 p-5 space-y-4 shadow-2xl">
            <div class="flex justify-between items-center border-b border-gray-800 pb-3">
                <div>
                    <h3 class="font-bold text-white text-base flex items-center gap-2">
                        <i class="fa-solid fa-sliders text-indigo-400"></i>
                        Ubah Alokasi Spek VM &bull; <span id="resizeVMTitle" class="text-indigo-400">VM</span>
                    </h3>
                    <p class="text-xs text-gray-400">Ubah jumlah vCPU Cores &amp; RAM tanpa merusak disk VHDX</p>
                </div>
                <button onclick="closeResizeModal()" class="text-gray-400 hover:text-white text-xl">&times;</button>
            </div>

            <form id="resizeVMForm" class="space-y-3 text-xs" onsubmit="submitResizeVM(event)">
                <div>
                    <label for="resizeCoresInput" class="block text-gray-400 mb-1">Jumlah vCPU Cores:</label>
                    <input id="resizeCoresInput" type="number" min="1" max="64" required class="w-full bg-gray-950 border border-gray-800 rounded-lg px-3 py-2 text-white font-mono focus:border-indigo-500 focus:outline-none">
                </div>
                <div>
                    <label for="resizeRAMInput" class="block text-gray-400 mb-1">Alokasi RAM (MB):</label>
                    <input id="resizeRAMInput" type="number" min="512" step="512" max="131072" required class="w-full bg-gray-950 border border-gray-800 rounded-lg px-3 py-2 text-white font-mono focus:border-indigo-500 focus:outline-none">
                    <p class="text-[11px] text-gray-500 mt-1">Contoh: 2048 (2 GB), 4096 (4 GB), 8192 (8 GB), 16384 (16 GB)</p>
                </div>

                <div class="bg-amber-950/30 border border-amber-500/20 rounded-xl p-3 text-[11px] text-amber-300/90 leading-relaxed">
                    <i class="fa-solid fa-triangle-exclamation text-amber-400 mr-1"></i>
                    Hyper-V mengharuskan CPU diubah saat VM mati. Jika VM sedang RUNNING, Crybaby akan mematikan VM sementara, menerapkan spek baru, dan menyalakan kembali secara otomatis.
                </div>

                <div class="flex justify-end gap-2 pt-2 border-t border-gray-800">
                    <button type="button" onclick="closeResizeModal()" class="px-4 py-2 rounded-lg bg-gray-800 hover:bg-gray-700 text-xs font-semibold text-gray-300">Batal</button>
                    <button type="submit" class="px-4 py-2 rounded-lg bg-indigo-600 hover:bg-indigo-500 text-xs font-semibold text-white flex items-center gap-1.5">
                        <i class="fa-solid fa-check"></i> Terapkan Spek Baru
                    </button>
                </div>
            </form>
        </div>
    </div>

    <!-- Expand Disk Modal -->
    <div id="expandDiskModal" class="fixed inset-0 bg-black/80 backdrop-blur-sm z-50 flex items-center justify-center hidden p-4">
        <div class="glass max-w-md w-full rounded-2xl border border-gray-700 p-5 space-y-4 shadow-2xl">
            <div class="flex justify-between items-center border-b border-gray-800 pb-3">
                <div>
                    <h3 class="font-bold text-white text-base flex items-center gap-2">
                        <i class="fa-solid fa-hard-drive text-teal-400"></i>
                        Perbesar Hard Disk VHDX &bull; <span id="expandDiskTitle" class="text-teal-400">VM</span>
                    </h3>
                    <p class="text-xs text-gray-400">Ekspansi kapasitas virtual disk VHDX Hyper-V</p>
                </div>
                <button onclick="closeExpandDiskModal()" class="text-gray-400 hover:text-white text-xl">&times;</button>
            </div>

            <form id="expandDiskForm" class="space-y-3 text-xs" onsubmit="submitExpandDisk(event)">
                <div>
                    <label for="expandSizeInput" class="block text-gray-400 mb-1">Ukuran Baru Hard Disk (GB):</label>
                    <input id="expandSizeInput" type="number" min="10" max="16000" required class="w-full bg-gray-950 border border-gray-800 rounded-lg px-3 py-2 text-white font-mono focus:border-teal-500 focus:outline-none">
                    <p class="text-[11px] text-gray-500 mt-1">Harus lebih besar dari kapasitas VHDX saat ini (Hyper-V hanya mendukung pembesaran ukuran).</p>
                </div>

                <div class="bg-teal-950/30 border border-teal-500/20 rounded-xl p-3 text-[11px] text-teal-300/90 leading-relaxed">
                    <i class="fa-solid fa-circle-info text-teal-400 mr-1"></i>
                    Setelah disk VHDX diperbesar, ruang ekstra dapat dialokasikan di dalam OS guest via <b>Disk Management</b> (Windows) atau <code>growpart &amp; resize2fs</code> (Linux).
                </div>

                <div class="flex justify-end gap-2 pt-2 border-t border-gray-800">
                    <button type="button" onclick="closeExpandDiskModal()" class="px-4 py-2 rounded-lg bg-gray-800 hover:bg-gray-700 text-xs font-semibold text-gray-300">Batal</button>
                    <button type="submit" class="px-4 py-2 rounded-lg bg-teal-600 hover:bg-teal-500 text-xs font-semibold text-white flex items-center gap-1.5">
                        <i class="fa-solid fa-up-right-and-down-left-from-center"></i> Perbesar Disk Sekarang
                    </button>
                </div>
            </form>
        </div>
    </div>

    <!-- Clone VM Modal -->
    <div id="cloneVMModal" class="fixed inset-0 bg-black/80 backdrop-blur-sm z-50 flex items-center justify-center hidden p-4">
        <div class="glass max-w-md w-full rounded-2xl border border-gray-700 p-5 space-y-4 shadow-2xl">
            <div class="flex justify-between items-center border-b border-gray-800 pb-3">
                <div>
                    <h3 class="font-bold text-white text-base flex items-center gap-2">
                        <i class="fa-solid fa-clone text-sky-400"></i>
                        Clone Virtual Machine &bull; <span id="cloneVMTitle" class="text-sky-400">VM</span>
                    </h3>
                    <p class="text-xs text-gray-400">Duplikasi VM secara instan beserta disk VHDX nya</p>
                </div>
                <button onclick="closeCloneModal()" class="text-gray-400 hover:text-white text-xl">&times;</button>
            </div>

            <form id="cloneVMForm" class="space-y-3 text-xs" onsubmit="submitCloneVM(event)">
                <div>
                    <label for="cloneNameInput" class="block text-gray-400 mb-1">Nama VM Kloning Baru:</label>
                    <input id="cloneNameInput" type="text" required placeholder="e.g. vm-clone-worker-02" class="w-full bg-gray-950 border border-gray-800 rounded-lg px-3 py-2 text-white font-mono focus:border-sky-500 focus:outline-none">
                </div>

                <div class="bg-sky-950/30 border border-sky-500/20 rounded-xl p-3 text-[11px] text-sky-300/90 leading-relaxed">
                    <i class="fa-solid fa-copy text-sky-400 mr-1"></i>
                    Crybaby akan menyalin file VHDX sumber ke direktori VM baru dan mendaftarkan konfigurasi CPU, RAM, serta Switch yang identik.
                </div>

                <div class="flex justify-end gap-2 pt-2 border-t border-gray-800">
                    <button type="button" onclick="closeCloneModal()" class="px-4 py-2 rounded-lg bg-gray-800 hover:bg-gray-700 text-xs font-semibold text-gray-300">Batal</button>
                    <button type="submit" class="px-4 py-2 rounded-lg bg-sky-600 hover:bg-sky-500 text-xs font-semibold text-white flex items-center gap-1.5">
                        <i class="fa-solid fa-clone"></i> Mulai Kloning VM
                    </button>
                </div>
            </form>
        </div>
    </div>

    <script>
        function switchInstallTab(tab) {
            const tabs = ['ps1', 'zip', 'linux'];
            tabs.forEach(t => {
                const btn = document.getElementById('tabBtn' + t.charAt(0).toUpperCase() + t.slice(1));
                const content = document.getElementById('tabContent' + t.charAt(0).toUpperCase() + t.slice(1));
                if (t === tab) {
                    if (btn) {
                        btn.className = 'flex-1 py-2 rounded-lg font-semibold transition text-white bg-indigo-600 shadow-sm';
                    }
                    if (content) content.classList.remove('hidden');
                } else {
                    if (btn) {
                        btn.className = 'flex-1 py-2 rounded-lg font-semibold transition text-gray-400 hover:text-gray-200';
                    }
                    if (content) content.classList.add('hidden');
                }
            });
        }

        function updateInstallerHost(hostVal) {
            const proto = window.location.protocol || 'http:';
            const host = (hostVal || window.location.host || '127.0.0.1:8080').trim();
            const ps1Elem = document.getElementById('ps1CommandText');
            if (ps1Elem) ps1Elem.innerText = 'irm ' + proto + '//' + host + '/install | iex';
            const shElem = document.getElementById('shCommandText');
            if (shElem) shElem.innerText = 'curl -sSL ' + proto + '//' + host + '/install.sh | sudo bash';
        }

        function copyCommand(elemId, btn) {
            const elem = document.getElementById(elemId);
            if (!elem) return;
            const text = elem.innerText.trim();
            navigator.clipboard.writeText(text).then(() => {
                const orig = btn.innerHTML;
                btn.innerHTML = '<i class="fa-solid fa-check"></i> Tersalin!';
                btn.classList.add('bg-emerald-600');
                btn.classList.remove('bg-indigo-600');
                setTimeout(() => {
                    btn.innerHTML = orig;
                    btn.classList.remove('bg-emerald-600');
                    btn.classList.add('bg-indigo-600');
                }, 2000);
            }).catch(() => {
                alert('Tersalin: ' + text);
            });
        }

        async function openUSBModal() { 
            document.getElementById('usbModal').classList.remove('hidden'); 
            let detected = window.location.host;
            const hostInput = document.getElementById('usbHostInput');
            if (hostInput && !hostInput.value) hostInput.value = detected;
            updateInstallerHost(hostInput ? hostInput.value : detected);

            try {
                const res = await fetchJson('/api/v1/provision/usb-profile');
                if (res && res.detected_host) {
                    detected = res.detected_host;
                    if (hostInput && (!hostInput.value || hostInput.value === window.location.host)) {
                        hostInput.value = detected;
                        updateInstallerHost(detected);
                    }
                }
            } catch (e) {}
        }
        function closeUSBModal() { document.getElementById('usbModal').classList.add('hidden'); }
        function downloadUSBPack() {
            const hostInput = document.getElementById('usbHostInput');
            const hostVal = (hostInput ? hostInput.value : '').trim();
            window.location.href = '/api/v1/provision/usb-zip?host=' + encodeURIComponent(hostVal);
        }

        async function fetchJson(url, options) {
            const res = await fetch(url, options);
            let data;
            const contentType = res.headers.get('content-type') || '';
            if (contentType.includes('application/json')) {
                try {
                    data = await res.json();
                } catch (e) {
                    data = null;
                }
            } else {
                const text = await res.text();
                data = { error: text };
            }
            if (!res.ok) {
                const msg = (data && data.error) ? data.error : ('HTTP ' + res.status);
                throw new Error(msg);
            }
            return data;
        }

        async function removeNode(nodeId, nodeName) {
            if (!confirm('Hapus node \"' + nodeName + '\" dari cluster?')) return;
            try {
                await fetchJson('/api/v1/nodes/' + nodeId, { method: 'DELETE' });
                setTimeout(() => {
                    fetch('/api/v1/nodes').then(r => r.json()).then(renderNodes).catch(() => {});
                }, 200);
            } catch (err) {
                alert('Gagal menghapus node: ' + err.message);
            }
        }

        async function resetCluster() {
            if (!confirm('PERINGATAN: Apakah Anda yakin ingin me-reset cluster?\n\nSemua node yang terdaftar di dashboard akan dihapus untuk memulai dari kondisi bersih/fresh.')) return;
            try {
                await fetchJson('/api/v1/cluster/reset', { method: 'POST' });
                alert('Cluster berhasil di-reset ke kondisi fresh.');
                setTimeout(() => {
                    fetch('/api/v1/nodes').then(r => r.json()).then(renderNodes).catch(() => {});
                }, 200);
            } catch (err) {
                alert('Gagal me-reset cluster: ' + err.message);
            }
        }

        // Real-time WebSocket Stream
        const wsProto = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const wsUrl = wsProto + '//' + window.location.host + '/api/v1/ws/telemetry';
        let ws;

        function connectWS() {
            ws = new WebSocket(wsUrl);
            ws.onopen = () => {
                document.getElementById('wsStatus').innerText = 'WS CONNECTED';
                document.getElementById('wsStatus').className = 'text-xs font-mono px-2.5 py-1 rounded-md bg-gray-900 border border-gray-800 text-emerald-400';
            };
            ws.onmessage = (e) => {
                const data = JSON.parse(e.data);
                if (data.summary) updateSummary(data.summary);
                if (data.nodes) renderNodes(data.nodes);
                if (data.history) renderHistory(data.history);
            };
            ws.onclose = () => {
                document.getElementById('wsStatus').innerText = 'WS RECONNECTING';
                document.getElementById('wsStatus').className = 'text-xs font-mono px-2.5 py-1 rounded-md bg-gray-900 border border-gray-800 text-amber-400';
                setTimeout(connectWS, 3000);
            };
        }

        function updateSummary(s) {
            document.getElementById('statNodes').innerText = s.total_nodes;
            document.getElementById('statOnline').innerText = s.online_nodes + ' online';
            document.getElementById('statOffline').innerText = s.offline_nodes + ' offline';
            
            const cpu = (s.avg_cpu_usage_pct || 0).toFixed(1);
            document.getElementById('statCPU').innerText = cpu + '%';
            document.getElementById('barCPU').style.width = Math.min(100, cpu) + '%';

            const usedRAMGB = ((s.used_ram_bytes || 0) / (1024*1024*1024)).toFixed(1);
            const totalRAMGB = ((s.total_ram_bytes || 0) / (1024*1024*1024)).toFixed(1);
            document.getElementById('statRAM').innerText = usedRAMGB + ' / ' + totalRAMGB + ' GB';
            const ramPct = s.total_ram_bytes ? (s.used_ram_bytes / s.total_ram_bytes * 100) : 0;
            document.getElementById('barRAM').style.width = Math.min(100, ramPct) + '%';

            document.getElementById('statVMs').innerText = s.total_active_vms || 0;
        }

        function renderHistory(points) {
            if (!points || points.length === 0) return;
            drawSparkline('cpuSparkline', points, 'cpu', '#818cf8', 'rgba(99, 102, 241, 0.2)');
            drawSparkline('ramSparkline', points, 'ram_pct', '#60a5fa', 'rgba(59, 130, 246, 0.2)');
            const latest = points[points.length - 1];
            if (document.getElementById('cpuTrendVal')) document.getElementById('cpuTrendVal').innerText = (latest.cpu || 0).toFixed(1) + '%';
            if (document.getElementById('ramTrendVal')) document.getElementById('ramTrendVal').innerText = (latest.ram_pct || 0).toFixed(1) + '%';
        }

        function drawSparkline(canvasId, points, key, lineColor, fillColor) {
            const canvas = document.getElementById(canvasId);
            if (!canvas) return;
            const ctx = canvas.getContext('2d');
            const width = canvas.width;
            const height = canvas.height;
            ctx.clearRect(0, 0, width, height);

            if (!points || points.length === 0) return;

            const values = points.map(p => p[key] || 0);
            let maxVal = 100;
            for (const v of values) { if (v > maxVal) maxVal = v; }
            const minVal = 0;

            const stepX = width / Math.max(1, points.length - 1);

            ctx.beginPath();
            const firstY = height - ((values[0] - minVal) / (maxVal - minVal || 1)) * (height - 6) - 3;
            ctx.moveTo(0, firstY);

            for (let i = 1; i < values.length; i++) {
                const x = i * stepX;
                const y = height - ((values[i] - minVal) / (maxVal - minVal || 1)) * (height - 6) - 3;
                ctx.lineTo(x, y);
            }

            ctx.strokeStyle = lineColor;
            ctx.lineWidth = 2;
            ctx.stroke();

            ctx.lineTo(width, height);
            ctx.lineTo(0, height);
            ctx.closePath();
            ctx.fillStyle = fillColor;
            ctx.fill();
        }

        const CONTROLLER_VERSION = '0.6.0';

        function updateNode(nodeId, nodeName) {
            if (!confirm('Update Crybaby Agent pada node \"' + nodeName + '\" ke versi terbaru (v' + CONTROLLER_VERSION + ') via OTA over LAN?\n\nAgent akan mengunduh biner baru dari laptop controller dan me-restart service otomatis dalam 2-3 detik.')) {
                return;
            }

            fetchJson('/api/v1/nodes/' + nodeId + '/update', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' }
            })
            .then(data => {
                alert('Pembaruan terkirim ke ' + nodeName + '!\n' + (data.message || 'Agent sedang memperbarui diri dan akan aktif kembali dalam beberapa detik.'));
                setTimeout(fetchNodesManual, 3000);
            })
            .catch(err => {
                alert('Gagal memperbarui node: ' + err.message);
            });
        }

        function renderNodes(nodes) {
            const tbody = document.getElementById('nodeTableBody');
            if (!nodes || nodes.length === 0) {
                tbody.innerHTML = '<tr><td colspan=\"7\" class=\"px-6 py-12 text-center text-gray-500 text-sm\"><i class=\"fa-solid fa-satellite-dish text-2xl mb-3 block text-gray-600\"></i>Belum ada Windows 11 Node yang terhubung.<br>Klik <b class=\"text-indigo-400\">\"Generate Node USB\"</b> untuk membuat installer.</td></tr>';
                return;
            }

            let html = '';
            for (const n of nodes) {
                const isOnline = n.status === 'ONLINE';
                const statusBadge = isOnline 
                    ? '<span class=\"px-2 py-0.5 rounded-full text-xs font-medium bg-emerald-500/10 text-emerald-400 border border-emerald-500/20\"><span class=\"w-1.5 h-1.5 inline-block rounded-full bg-emerald-400 mr-1 pulse-dot\"></span>ONLINE</span>'
                    : '<span class=\"px-2 py-0.5 rounded-full text-xs font-medium bg-gray-500/10 text-gray-400 border border-gray-500/20\">OFFLINE</span>';

                const nodeVersion = n.version || '0.4.0';
                const isOutdated = isOnline && (nodeVersion !== CONTROLLER_VERSION);
                const versionBadge = isOutdated
                    ? '<span title=\"Versi node: v' + escapeHtml(nodeVersion) + ' (Update tersedia: v' + CONTROLLER_VERSION + ')\" onclick=\"updateNode(\'' + escapeHtml(n.id) + '\', \'' + escapeHtml(n.name) + '\')\" class=\"cursor-pointer text-[10px] font-semibold px-1.5 py-0.5 rounded bg-amber-500/20 text-amber-300 border border-amber-500/30 flex items-center gap-1 hover:bg-amber-500/30 transition\"><i class=\"fa-solid fa-arrow-up-from-bracket\"></i> v' + escapeHtml(nodeVersion) + '</span>'
                    : '<span class=\"text-[10px] font-mono px-1.5 py-0.5 rounded bg-gray-800 text-gray-400 border border-gray-700\">v' + escapeHtml(nodeVersion) + '</span>';

                const cpu = (n.latest_metrics && n.latest_metrics.cpu_usage_pct !== undefined) ? n.latest_metrics.cpu_usage_pct.toFixed(1) + '%' : '-';
                const ram = (n.latest_metrics && n.latest_metrics.ram_used_bytes) ? (n.latest_metrics.ram_used_bytes / (1024*1024*1024)).toFixed(1) + ' GB' : '-';
                const hw = n.hardware || {};
                const cpuCores = hw.cpu_cores || 0;
                const totalRamGB = hw.total_ram_bytes ? (hw.total_ram_bytes / (1024*1024*1024)).toFixed(0) : '0';
                const hypervBadge = hw.hyperv_enabled
                    ? '<span class=\"text-emerald-400 font-semibold\"><i class=\"fa-solid fa-circle-check mr-1\"></i>Ready</span>'
                    : '<span class=\"text-amber-400 font-semibold\"><i class=\"fa-solid fa-triangle-exclamation mr-1\"></i>Disabled</span>';

                html += '<tr class=\"hover:bg-gray-800/30 transition\">' +
                    '<td class=\"px-6 py-4 font-medium text-white flex items-center gap-2\"><i class=\"fa-brands fa-windows text-blue-400\"></i>' + escapeHtml(n.name) + ' ' + versionBadge + '</td>' +
                    '<td class=\"px-6 py-4\">' + statusBadge + '</td>' +
                    '<td class=\"px-6 py-4 font-mono text-xs\"><span class=\"text-indigo-400 font-semibold\">' + (n.mesh_ip || '-') + '</span><br><span class=\"text-gray-500\">' + (n.ip_address || '-') + '</span></td>' +
                    '<td class=\"px-6 py-4 text-xs text-gray-300\">' + escapeHtml(hw.cpu_model || 'Unknown CPU') + '<br><span class=\"text-gray-500\">' + cpuCores + ' Cores &bull; ' + totalRamGB + ' GB RAM</span></td>' +
                    '<td class=\"px-6 py-4 text-xs font-mono\"><span class=\"text-white font-semibold\">CPU: ' + cpu + '</span><br><span class=\"text-gray-400\">RAM: ' + ram + '</span></td>' +
                    '<td class=\"px-6 py-4 text-xs\">' + hypervBadge + '</td>' +
                    '<td class=\"px-6 py-4 text-right space-x-1.5\">' +
                        (isOutdated ? '<button onclick=\"updateNode(\'' + escapeHtml(n.id) + '\', \'' + escapeHtml(n.name) + '\')\" title=\"Update agent ke v' + CONTROLLER_VERSION + ' via OTA\" class=\"px-2.5 py-1 rounded bg-amber-500/20 hover:bg-amber-500/30 text-amber-300 text-xs font-semibold border border-amber-500/40 transition\"><i class=\"fa-solid fa-arrow-up-from-bracket mr-1\"></i>Update</button>' : '') +
                        (!isOnline ? '<button onclick=\"triggerWoL(\'' + escapeHtml(n.id) + '\')\" class=\"px-2.5 py-1 rounded bg-amber-500/10 hover:bg-amber-500/20 text-amber-400 text-xs font-medium border border-amber-500/20 transition\"><i class=\"fa-solid fa-power-off mr-1\"></i>WoL</button>' : '') +
                        '<button onclick=\"openVMModal(\'' + escapeHtml(n.id) + '\', \'' + escapeHtml(n.name) + '\')\" class=\"px-2.5 py-1 rounded bg-emerald-500/10 hover:bg-emerald-500/20 text-emerald-400 text-xs font-medium border border-emerald-500/20 transition\"><i class=\"fa-solid fa-cube mr-1\"></i>VMs</button>' +
                        '<button onclick=\"openTerminal(\'' + escapeHtml(n.id) + '\', \'' + escapeHtml(n.name) + '\')\" class=\"px-2.5 py-1 rounded bg-indigo-500/10 hover:bg-indigo-500/20 text-indigo-400 text-xs font-medium border border-indigo-500/20 transition\"><i class=\"fa-solid fa-terminal mr-1\"></i>Shell</button>' +
                        (isOnline ? '<button onclick=\"rebootHost(\'' + escapeHtml(n.id) + '\', \'' + escapeHtml(n.name) + '\')\" title=\"Restart PC Host ini\" class=\"px-2.5 py-1 rounded bg-blue-500/10 hover:bg-blue-500/20 text-blue-400 text-xs font-medium border border-blue-500/20 transition\"><i class=\"fa-solid fa-arrows-rotate mr-1\"></i>Reboot</button>' : '') +
                        (isOnline ? '<button onclick=\"shutdownHost(\'' + escapeHtml(n.id) + '\', \'' + escapeHtml(n.name) + '\')\" title=\"Shutdown PC Host ini\" class=\"px-2.5 py-1 rounded bg-red-500/10 hover:bg-red-500/20 text-red-400 text-xs font-medium border border-red-500/20 transition\"><i class=\"fa-solid fa-power-off mr-1\"></i>Power Off</button>' : '') +
                        (!isOnline ? '<button onclick=\"removeNode(\'' + escapeHtml(n.id) + '\', \'' + escapeHtml(n.name) + '\')\" title=\"Hapus node offline ini dari cluster\" class=\"px-2.5 py-1 rounded bg-red-500/10 hover:bg-red-500/20 text-red-400 text-xs font-medium border border-red-500/20 transition\"><i class=\"fa-solid fa-trash\"></i></button>' : '') +
                    '</td>' +
                '</tr>';
            }
            tbody.innerHTML = html;
        }

        // VM Modal Management
        let currentVMNodeId = null;

        function openVMModal(nodeId, nodeName) {
            currentVMNodeId = nodeId;
            document.getElementById('vmNodeTitle').innerText = nodeName;
            document.getElementById('vmModal').classList.remove('hidden');
            loadVMs(nodeId);
            loadSwitches(nodeId);
            loadISODropdown();
        }

        function closeVMModal() {
            document.getElementById('vmModal').classList.add('hidden');
            currentVMNodeId = null;
        }

        function toggleNewVMForm() {
            const form = document.getElementById('newVMForm');
            const chev = document.getElementById('newVMChevron');
            if (form.classList.contains('hidden')) {
                form.classList.remove('hidden');
                chev.classList.add('rotate-180');
            } else {
                form.classList.add('hidden');
                chev.classList.remove('rotate-180');
            }
        }

        function loadISODropdown() {
            fetch('/api/v1/iso')
                .then(r => r.json())
                .then(isos => {
                    const sel = document.getElementById('vmISOSelect');
                    if (!sel) return;
                    sel.innerHTML = '<option value=\"\">-- Tanpa ISO (Boot dari VHDX kosong) --</option>';
                    (isos || []).forEach(iso => {
                        const opt = document.createElement('option');
                        const url = 'http://' + window.location.host + '/iso/' + encodeURIComponent(iso.name);
                        const mb = (iso.size_bytes / (1024 * 1024)).toFixed(1);
                        opt.value = url;
                        opt.innerText = iso.name + ' (' + mb + ' MB)';
                        sel.appendChild(opt);
                    });
                })
                .catch(() => {});
        }

        function syncISOInput(val) {
            const input = document.getElementById('vmISO');
            if (input) input.value = val;
        }

        function loadSwitches(nodeId) {
            fetch('/api/v1/nodes/' + nodeId + '/switches')
                .then(r => r.json())
                .then(switches => {
                    const sel = document.getElementById('vmSwitch');
                    sel.innerHTML = '';
                    (switches || ['Default Switch']).forEach(s => {
                        const opt = document.createElement('option');
                        opt.value = s;
                        opt.innerText = s;
                        sel.appendChild(opt);
                    });
                })
                .catch(() => {});
        }

        function loadVMs(nodeId) {
            const tbody = document.getElementById('vmTableBody');
            tbody.innerHTML = '<tr><td colspan=\"7\" class=\"px-4 py-6 text-center text-gray-500\">Memuat VM Hyper-V...</td></tr>';
            
            fetchJson('/api/v1/nodes/' + nodeId + '/vms')
                .then(vms => {
                    if (!vms || vms.length === 0) {
                        tbody.innerHTML = '<tr><td colspan=\"7\" class=\"px-4 py-6 text-center text-gray-500\">Belum ada VM di node ini. Buat VM baru menggunakan form di atas.</td></tr>';
                        return;
                    }
                    let html = '';
                    vms.forEach(vm => {
                        const isRunning = vm.state === 'RUNNING';
                        const badge = isRunning
                            ? '<span class=\"px-2 py-0.5 rounded text-[10px] font-semibold bg-emerald-500/10 text-emerald-400 border border-emerald-500/20\">RUNNING</span>'
                            : '<span class=\"px-2 py-0.5 rounded text-[10px] font-semibold bg-gray-500/10 text-gray-400 border border-gray-500/20\">STOPPED</span>';
                        
                        let ipBadges = '<span class=\"text-gray-500 text-[11px]\">-</span>';
                        let rdpBtn = '';
                        if (vm.ip_addresses && vm.ip_addresses.length > 0) {
                            const firstIp = vm.ip_addresses[0];
                            ipBadges = vm.ip_addresses.map(ip => '<span class=\"px-1.5 py-0.5 rounded bg-emerald-500/10 border border-emerald-500/20 text-emerald-400 text-[11px]\">' + escapeHtml(ip) + '</span>').join(' ');
                            rdpBtn = ' <button onclick=\"downloadRdp(\'' + escapeHtml(vm.name) + '\', \'' + escapeHtml(firstIp) + '\')\" title=\"Download file .rdp untuk Remote Desktop instan\" class=\"px-1.5 py-0.5 rounded bg-indigo-500/20 text-indigo-300 hover:bg-indigo-500/30 text-[10px] font-bold transition\"><i class=\"fa-solid fa-desktop mr-1\"></i>RDP</button>';
                        }

                        const cpuText = '<span class=\"font-bold ' + ((vm.cpu_usage_pct || 0) > 50 ? 'text-amber-400' : 'text-emerald-400') + '\">' + (vm.cpu_usage_pct || 0) + '%</span> &bull; ' + (vm.cpu_count || 2) + ' vCPU';
                        const ramText = (vm.memory_demand_mb ? '<span class=\"text-indigo-400 font-semibold\">' + vm.memory_demand_mb + ' MB</span> / ' : '') + (vm.memory_mb || 2048) + ' MB';

                        html += '<tr class=\"hover:bg-gray-800/20\">' +
                            '<td class=\"px-4 py-3 font-bold text-white\">' + escapeHtml(vm.name) + '</td>' +
                            '<td class=\"px-4 py-3\">' + badge + '</td>' +
                            '<td class=\"px-4 py-3 text-gray-300\">' + cpuText + '</td>' +
                            '<td class=\"px-4 py-3 text-gray-300\">' + ramText + '</td>' +
                            '<td class=\"px-4 py-3 font-mono\">' + ipBadges + rdpBtn + '</td>' +
                            '<td class=\"px-4 py-3 text-gray-500 truncate max-w-xs\" title=\"' + (vm.disk_path || '-') + '\">' + (vm.disk_path || '-') + '</td>' +
                            '<td class=\"px-4 py-3 text-right space-x-1\">' +
                                (!isRunning 
                                    ? '<button onclick=\"controlVM(\'' + vm.id + '\', \'start\')\" class=\"px-2 py-1 rounded bg-emerald-600/20 text-emerald-400 hover:bg-emerald-600/30 transition\" title=\"Start VM\"><i class=\"fa-solid fa-play\"></i></button>'
                                    : '<button onclick=\"controlVM(\'' + vm.id + '\', \'stop\')\" class=\"px-2 py-1 rounded bg-amber-600/20 text-amber-400 hover:bg-amber-600/30 transition\" title=\"Stop VM\"><i class=\"fa-solid fa-stop\"></i></button>' +
                                      '<button onclick=\"controlVM(\'' + vm.id + '\', \'restart\')\" class=\"px-2 py-1 rounded bg-blue-600/20 text-blue-400 hover:bg-blue-600/30 transition\" title=\"Restart VM\"><i class=\"fa-solid fa-arrows-rotate\"></i></button>') +
                                '<button onclick=\"openResizeModal(\'' + vm.id + '\', \'' + escapeHtml(vm.name) + '\', ' + (vm.cpu_count || 2) + ', ' + (vm.memory_mb || 2048) + ')\" class=\"px-2 py-1 rounded bg-indigo-600/20 text-indigo-400 hover:bg-indigo-600/30 transition\" title=\"Ubah vCPU &amp; RAM\"><i class=\"fa-solid fa-sliders\"></i></button>' +
                                '<button onclick=\"openExpandDiskModal(\'' + vm.id + '\', \'' + escapeHtml(vm.name) + '\')\" class=\"px-2 py-1 rounded bg-teal-600/20 text-teal-400 hover:bg-teal-600/30 transition\" title=\"Perbesar Disk VHDX\"><i class=\"fa-solid fa-hard-drive\"></i></button>' +
                                '<button onclick=\"openCloneModal(\'' + vm.id + '\', \'' + escapeHtml(vm.name) + '\')\" class=\"px-2 py-1 rounded bg-sky-600/20 text-sky-400 hover:bg-sky-600/30 transition\" title=\"Clone VM\"><i class=\"fa-solid fa-clone\"></i></button>' +
                                '<button onclick=\"openSnapshotsModal(\'' + vm.id + '\', \'' + escapeHtml(vm.name) + '\')\" class=\"px-2 py-1 rounded bg-purple-600/20 text-purple-400 hover:bg-purple-600/30 transition\" title=\"Kelola Snapshot / Checkpoints\"><i class=\"fa-solid fa-camera\"></i></button>' +
                                '<button onclick=\"deleteVM(\'' + vm.id + '\')\" class=\"px-2 py-1 rounded bg-red-600/20 text-red-400 hover:bg-red-600/30 transition\" title=\"Hapus VM\"><i class=\"fa-solid fa-trash\"></i></button>' +
                            '</td>' +
                        '</tr>';
                    });
                    tbody.innerHTML = html;
                })
                .catch(err => {
                    tbody.innerHTML = '<tr><td colspan=\"7\" class=\"px-4 py-6 text-center text-amber-400/90 font-medium\"><i class=\"fa-solid fa-triangle-exclamation mr-2 text-amber-400\"></i>Gagal memuat VM: ' + escapeHtml(err.message) + '</td></tr>';
                });
        }

        function downloadRdp(vmName, ip) {
            const rdpContent = [
                'full address:s:' + ip + ':3389',
                'prompt for credentials:i:1',
                'administrative session:i:1',
                'screen mode id:i:2',
                'use multimon:i:0',
                'desktopwidth:i:1920',
                'desktopheight:i:1080',
                'session bpp:i:32',
                'compression:i:1',
                'keyboardhook:i:2',
                'audiomode:i:0'
            ].join('\r\n');

            const blob = new Blob([rdpContent], { type: 'application/x-rdp' });
            const url = URL.createObjectURL(blob);
            const a = document.createElement('a');
            a.href = url;
            a.download = (vmName || 'crybaby-vm') + '.rdp';
            document.body.appendChild(a);
            a.click();
            document.body.removeChild(a);
            URL.revokeObjectURL(url);
        }

        let currentSnapshotVMId = null;

        function openSnapshotsModal(vmId, vmName) {
            currentSnapshotVMId = vmId;
            document.getElementById('snapVMTitle').innerText = vmName || vmId;
            document.getElementById('snapshotModal').classList.remove('hidden');
            loadSnapshots(vmId);
        }

        function closeSnapshotsModal() {
            document.getElementById('snapshotModal').classList.add('hidden');
            currentSnapshotVMId = null;
        }

        function loadSnapshots(vmId) {
            const tbody = document.getElementById('snapshotTableBody');
            tbody.innerHTML = '<tr><td colspan=\"3\" class=\"px-3 py-4 text-center text-gray-500\">Memuat snapshot...</td></tr>';
            
            fetchJson('/api/v1/nodes/' + currentVMNodeId + '/vms/' + vmId + '/checkpoints')
                .then(snaps => {
                    if (!snaps || snaps.length === 0) {
                        tbody.innerHTML = '<tr><td colspan=\"3\" class=\"px-3 py-4 text-center text-gray-500\">Belum ada snapshot untuk VM ini.</td></tr>';
                        return;
                    }
                    let html = '';
                    snaps.forEach(s => {
                        const dateStr = s.created_at ? new Date(s.created_at).toLocaleString('id-ID') : '-';
                        html += '<tr class=\"hover:bg-gray-800/20\">' +
                            '<td class=\"px-3 py-2 font-bold text-white\">' + escapeHtml(s.name) + '</td>' +
                            '<td class=\"px-3 py-2 text-gray-400 text-[11px]\">' + dateStr + '</td>' +
                            '<td class=\"px-3 py-2 text-right space-x-1\">' +
                                '<button onclick=\"restoreSnapshot(\'' + escapeHtml(s.name) + '\')\" class=\"px-2 py-0.5 rounded bg-amber-600/20 text-amber-300 hover:bg-amber-600/30 text-[10px] font-semibold\" title=\"Rollback ke snapshot ini\"><i class=\"fa-solid fa-clock-rotate-left mr-1\"></i>Rollback</button>' +
                                '<button onclick=\"deleteSnapshot(\'' + escapeHtml(s.name) + '\')\" class=\"px-2 py-0.5 rounded bg-red-600/20 text-red-400 hover:bg-red-600/30 text-[10px]\" title=\"Hapus snapshot\"><i class=\"fa-solid fa-trash\"></i></button>' +
                            '</td>' +
                        '</tr>';
                    });
                    tbody.innerHTML = html;
                })
                .catch(err => {
                    tbody.innerHTML = '<tr><td colspan=\"3\" class=\"px-3 py-4 text-center text-amber-400\">Gagal memuat snapshot: ' + escapeHtml(err.message) + '</td></tr>';
                });
        }

        function submitNewSnapshot(e) {
            e.preventDefault();
            if (!currentVMNodeId || !currentSnapshotVMId) return;

            const nameInput = document.getElementById('snapNameInput');
            const name = nameInput.value.trim();
            if (!name) return;

            fetchJson('/api/v1/nodes/' + currentVMNodeId + '/vms/' + currentSnapshotVMId + '/checkpoints', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ name: name })
            })
            .then(() => {
                nameInput.value = '';
                loadSnapshots(currentSnapshotVMId);
            })
            .catch(err => alert('Gagal membuat snapshot: ' + err.message));
        }

        function restoreSnapshot(snapName) {
            if (!currentVMNodeId || !currentSnapshotVMId) return;
            if (!confirm('PERINGATAN: Kembalikan (Rollback) VM ini ke snapshot \"' + snapName + '\"?\\n\\nKondisi VM saat ini akan ditimpa dengan keadaan saat snapshot dibuat.')) return;

            fetchJson('/api/v1/nodes/' + currentVMNodeId + '/vms/' + currentSnapshotVMId + '/checkpoints/restore', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ name: snapName })
            })
            .then(() => {
                alert('VM berhasil di-rollback ke snapshot \"' + snapName + '\"!');
                loadVMs(currentVMNodeId);
            })
            .catch(err => alert('Gagal rollback snapshot: ' + err.message));
        }

        function deleteSnapshot(snapName) {
            if (!currentVMNodeId || !currentSnapshotVMId) return;
            if (!confirm('Hapus snapshot \"' + snapName + '\"?')) return;

            fetchJson('/api/v1/nodes/' + currentVMNodeId + '/vms/' + currentSnapshotVMId + '/checkpoints/' + encodeURIComponent(snapName), {
                method: 'DELETE'
            })
            .then(() => {
                loadSnapshots(currentSnapshotVMId);
            })
            .catch(err => alert('Gagal menghapus snapshot: ' + err.message));
        }

        function submitNewVM(e) {
            e.preventDefault();
            if (!currentVMNodeId) return;

            const payload = {
                name: document.getElementById('vmName').value,
                switch_name: document.getElementById('vmSwitch').value,
                cpu_count: parseInt(document.getElementById('vmCores').value, 10),
                memory_mb: parseInt(document.getElementById('vmRAM').value, 10),
                disk_size_gb: parseInt(document.getElementById('vmDisk').value, 10),
                iso_path: (document.getElementById('vmISO') ? document.getElementById('vmISO').value.trim() : '')
            };

            fetchJson('/api/v1/nodes/' + currentVMNodeId + '/vms', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(payload)
            })
            .then(() => {
                alert('VM ' + payload.name + ' berhasil dibuat!');
                toggleNewVMForm();
                document.getElementById('newVMForm').reset();
                loadVMs(currentVMNodeId);
            })
            .catch(err => alert('Gagal membuat VM: ' + err.message));
        }

        function controlVM(vmId, action) {
            if (!currentVMNodeId) return;
            fetchJson('/api/v1/nodes/' + currentVMNodeId + '/vms/' + vmId + '/' + action, { method: 'POST' })
                .then(() => loadVMs(currentVMNodeId))
                .catch(err => alert('Gagal ' + action + ' VM: ' + err.message));
        }

        function deleteVM(vmId) {
            if (!currentVMNodeId || !confirm('Yakin ingin menghapus VM ini beserta disk VHDX nya?')) return;
            fetchJson('/api/v1/nodes/' + currentVMNodeId + '/vms/' + vmId, { method: 'DELETE' })
                .then(() => loadVMs(currentVMNodeId))
                .catch(err => alert('Gagal menghapus VM: ' + err.message));
        }

        function triggerWoL(nodeId) {
            fetchJson('/api/v1/nodes/' + nodeId + '/wol', { method: 'POST' })
                .then(d => alert('Wake-on-LAN packet terkirim ke MAC: ' + d.mac))
                .catch(err => alert('Gagal mengirim WoL: ' + err.message));
        }

        async function rebootHost(nodeId, nodeName) {
            if (!confirm('PERINGATAN: Restart PC fisik \"' + nodeName + '\"?\n\nSemua VM yang berjalan di node ini akan tersimpan otomatis dan menyala kembali saat Windows selesai booting.')) return;
            try {
                await fetchJson('/api/v1/nodes/' + nodeId + '/reboot', { method: 'POST' });
                alert('Perintah restart telah dikirim ke PC ' + nodeName);
            } catch (err) {
                alert('Gagal restart PC: ' + err.message);
            }
        }

        async function shutdownHost(nodeId, nodeName) {
            if (!confirm('PERINGATAN: Matikan (Shutdown) PC fisik \"' + nodeName + '\"?\n\nUntuk menyalakannya kembali nanti, Anda bisa menggunakan tombol WoL.')) return;
            try {
                await fetchJson('/api/v1/nodes/' + nodeId + '/shutdown', { method: 'POST' });
                alert('Perintah shutdown telah dikirim ke PC ' + nodeName);
            } catch (err) {
                alert('Gagal shutdown PC: ' + err.message);
            }
        }

        let termWS = null;
        let term = null;
        let fitAddon = null;

        function openTerminal(nodeId, nodeName) {
            document.getElementById('termNodeTitle').innerText = nodeName || nodeId;
            document.getElementById('terminalModal').classList.remove('hidden');

            const container = document.getElementById('terminalContainer');
            container.innerHTML = '';

            if (term) {
                try { term.dispose(); } catch (e) {}
            }

            term = new Terminal({
                cursorBlink: true,
                theme: {
                    background: '#0B0F19',
                    foreground: '#F3F4F6',
                    cursor: '#6366F1',
                    selectionBackground: '#4338CA'
                },
                fontSize: 13,
                fontFamily: 'Menlo, Monaco, "Courier New", monospace'
            });

            fitAddon = new FitAddon.FitAddon();
            term.loadAddon(fitAddon);
            term.open(container);
            fitAddon.fit();

            const wsProto = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
            const wsUrl = wsProto + '//' + window.location.host + '/api/v1/ws/terminal/' + nodeId;
            
            term.writeln('\x1b[36m⚡ Connecting to Crybaby Node Terminal...\x1b[0m\r\n');
            termWS = new WebSocket(wsUrl);

            termWS.onopen = () => {
                term.writeln('\x1b[32m✔ Connected to node shell.\x1b[0m\r\n');
            };

            termWS.onmessage = (e) => {
                if (!term) return;
                try {
                    const msg = JSON.parse(e.data);
                    if (msg.type === 'output' && msg.data) {
                        term.write(msg.data);
                    } else if (msg.type === 'error') {
                        term.writeln('\r\n\x1b[31m[Error] ' + msg.data + '\x1b[0m\r\n');
                    }
                } catch (err) {
                    if (term) term.write(e.data);
                }
            };

            termWS.onerror = () => {
                if (term) {
                    try { term.writeln('\r\n\x1b[31m✖ WebSocket error encountered.\x1b[0m\r\n'); } catch (e) {}
                }
            };

            termWS.onclose = () => {
                if (term) {
                    try { term.writeln('\r\n\x1b[33m⚡ Terminal session disconnected.\x1b[0m\r\n'); } catch (e) {}
                }
            };

            term.onData(data => {
                if (termWS && termWS.readyState === WebSocket.OPEN) {
                    termWS.send(JSON.stringify({ type: 'input', data: data }));
                }
            });

        }

        function closeTerminal() {
            document.getElementById('terminalModal').classList.add('hidden');
            if (termWS) {
                termWS.onclose = null;
                termWS.onerror = null;
                termWS.onmessage = null;
                try { termWS.close(); } catch (e) {}
                termWS = null;
            }
            if (term) {
                try { term.dispose(); } catch(e) {}
                term = null;
            }
        }

        window.addEventListener('resize', () => {
            if (fitAddon && term && !document.getElementById('terminalModal').classList.contains('hidden')) {
                try { fitAddon.fit(); } catch(e) {}
            }
        });

        function fetchNodesManual() {
            fetch('/api/v1/nodes')
                .then(r => r.json())
                .then(nodes => renderNodes(nodes))
                .catch(() => {});
        }

        // ISO Catalog & Downloader
        let isoPollTimer = null;

        function openISOCatalogModal() {
            document.getElementById('isoCatalogModal').classList.remove('hidden');
            loadISOCatalog();
            pollISODownloads();
        }

        function closeISOCatalogModal() {
            document.getElementById('isoCatalogModal').classList.add('hidden');
            if (isoPollTimer) {
                clearInterval(isoPollTimer);
                isoPollTimer = null;
            }
        }

        function loadISOCatalog() {
            const tbody = document.getElementById('isoTableBody');
            tbody.innerHTML = '<tr><td colspan=\"4\" class=\"px-3 py-4 text-center text-gray-500\">Memuat berkas ISO...</td></tr>';
            fetchJson('/api/v1/iso')
                .then(isos => {
                    if (!isos || isos.length === 0) {
                        tbody.innerHTML = '<tr><td colspan=\"4\" class=\"px-3 py-6 text-center text-gray-500\">Belum ada berkas ISO di folder <code>./iso</code> controller.<br>Gunakan form di atas untuk mengunduh ISO via URL internet.</td></tr>';
                        return;
                    }
                    let html = '';
                    isos.forEach(iso => {
                        const mb = (iso.size_bytes / (1024*1024)).toFixed(1);
                        const sizeStr = iso.size_bytes > 1024*1024*1024 
                            ? (iso.size_bytes / (1024*1024*1024)).toFixed(2) + ' GB'
                            : mb + ' MB';
                        const modStr = iso.modified_at ? new Date(iso.modified_at).toLocaleString('id-ID') : '-';
                        const directURL = 'http://' + window.location.host + '/iso/' + encodeURIComponent(iso.name);
                        html += '<tr class=\"hover:bg-gray-800/20\">' +
                            '<td class=\"px-3 py-2 font-bold text-white flex items-center gap-1.5\"><i class=\"fa-solid fa-compact-disc text-indigo-400\"></i>' + escapeHtml(iso.name) + '</td>' +
                            '<td class=\"px-3 py-2 text-indigo-300\">' + sizeStr + '</td>' +
                            '<td class=\"px-3 py-2 text-gray-400 text-[11px]\">' + modStr + '</td>' +
                            '<td class=\"px-3 py-2 text-right space-x-1\">' +
                                '<button onclick=\"copyISOLink(\'' + escapeHtml(directURL) + '\')\" class=\"px-2 py-0.5 rounded bg-indigo-600/20 text-indigo-300 hover:bg-indigo-600/30 text-[10px] font-semibold\"><i class=\"fa-solid fa-copy mr-1\"></i>Copy Link</button>' +
                                '<button onclick=\"selectISOForNewVM(\'' + escapeHtml(directURL) + '\')\" class=\"px-2 py-0.5 rounded bg-emerald-600/20 text-emerald-300 hover:bg-emerald-600/30 text-[10px] font-semibold\"><i class=\"fa-solid fa-plus mr-1\"></i>Pakai di VM</button>' +
                            '</td>' +
                        '</tr>';
                    });
                    tbody.innerHTML = html;
                })
                .catch(err => {
                    tbody.innerHTML = '<tr><td colspan=\"4\" class=\"px-3 py-4 text-center text-red-400\">Gagal memuat ISO: ' + escapeHtml(err.message) + '</td></tr>';
                });
        }

        function copyISOLink(url) {
            navigator.clipboard.writeText(url).then(() => {
                alert('LAN Download URL disalin ke clipboard:\n' + url);
            }).catch(() => {
                prompt('Salin link ISO ini:', url);
            });
        }

        function selectISOForNewVM(url) {
            closeISOCatalogModal();
            const isoInput = document.getElementById('vmISO');
            if (isoInput) isoInput.value = url;
            const form = document.getElementById('newVMForm');
            if (form && form.classList.contains('hidden')) {
                toggleNewVMForm();
            }
        }

        function submitDownloadISO(e) {
            e.preventDefault();
            const urlInput = document.getElementById('isoDownloadURL');
            const fileInput = document.getElementById('isoDownloadFilename');
            const url = urlInput.value.trim();
            const filename = fileInput.value.trim();
            if (!url) return;

            const btn = document.getElementById('btnStartISODownload');
            btn.disabled = true;
            btn.innerHTML = '<i class=\"fa-solid fa-spinner fa-spin\"></i> Memulai...';

            fetchJson('/api/v1/iso/download', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ url: url, filename: filename })
            })
            .then(() => {
                urlInput.value = '';
                fileInput.value = '';
                btn.disabled = false;
                btn.innerHTML = '<i class=\"fa-solid fa-download\"></i> Mulai Download ISO';
                pollISODownloads();
            })
            .catch(err => {
                btn.disabled = false;
                btn.innerHTML = '<i class=\"fa-solid fa-download\"></i> Mulai Download ISO';
                alert('Gagal memulai unduhan ISO: ' + err.message);
            });
        }

        function pollISODownloads() {
            fetch('/api/v1/iso/downloads')
                .then(r => r.json())
                .then(tasks => {
                    const container = document.getElementById('isoDownloadsList');
                    if (!tasks || tasks.length === 0) {
                        container.classList.add('hidden');
                        return;
                    }
                    container.classList.remove('hidden');
                    let html = '';
                    let hasActive = false;
                    tasks.forEach(t => {
                        if (t.status === 'downloading') hasActive = true;
                        const pct = t.total_bytes > 0 ? ((t.downloaded_bytes / t.total_bytes) * 100).toFixed(1) : '0.0';
                        const dlMB = (t.downloaded_bytes / (1024*1024)).toFixed(1);
                        const totMB = t.total_bytes > 0 ? (t.total_bytes / (1024*1024)).toFixed(1) + ' MB' : 'unknown';
                        const statusColor = t.status === 'completed' ? 'text-emerald-400' : (t.status === 'failed' ? 'text-red-400' : 'text-indigo-400');
                        html += '<div class=\"p-2 rounded-lg bg-gray-950 border border-gray-800 space-y-1\">' +
                            '<div class=\"flex justify-between items-center\">' +
                                '<span class=\"font-bold text-white\">' + escapeHtml(t.filename) + '</span>' +
                                '<span class=\"' + statusColor + ' font-semibold\">' + t.status.toUpperCase() + ' (' + pct + '%)</span>' +
                            '</div>' +
                            '<div class=\"w-full bg-gray-800 h-1 rounded-full overflow-hidden\">' +
                                '<div class=\"bg-indigo-500 h-full\" style=\"width:' + pct + '%\"></div>' +
                            '</div>' +
                            '<div class=\"flex justify-between text-gray-500 text-[10px]\">' +
                                '<span>' + dlMB + ' MB / ' + totMB + '</span>' +
                                (t.error ? '<span class=\"text-red-400 truncate max-w-xs\">' + escapeHtml(t.error) + '</span>' : '') +
                            '</div>' +
                        '</div>';
                    });
                    container.innerHTML = html;
                    if (hasActive && !isoPollTimer) {
                        isoPollTimer = setInterval(pollISODownloads, 1500);
                    } else if (!hasActive && isoPollTimer) {
                        clearInterval(isoPollTimer);
                        isoPollTimer = null;
                        loadISOCatalog();
                        loadISODropdown();
                    }
                })
                .catch(() => {});
        }

        // Resize VM
        let currentResizeVMId = null;
        function openResizeModal(vmId, vmName, currentCores, currentRAM) {
            currentResizeVMId = vmId;
            document.getElementById('resizeVMTitle').innerText = vmName || vmId;
            document.getElementById('resizeCoresInput').value = currentCores || 2;
            document.getElementById('resizeRAMInput').value = currentRAM || 4096;
            document.getElementById('resizeVMModal').classList.remove('hidden');
        }
        function closeResizeModal() {
            document.getElementById('resizeVMModal').classList.add('hidden');
            currentResizeVMId = null;
        }
        function submitResizeVM(e) {
            e.preventDefault();
            if (!currentVMNodeId || !currentResizeVMId) return;

            const cores = parseInt(document.getElementById('resizeCoresInput').value, 10);
            const ram = parseInt(document.getElementById('resizeRAMInput').value, 10);

            fetchJson('/api/v1/nodes/' + currentVMNodeId + '/vms/' + currentResizeVMId + '/resize', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ cpu_count: cores, memory_mb: ram })
            })
            .then(() => {
                alert('Spesifikasi VM berhasil diubah menjadi ' + cores + ' vCPU dan ' + ram + ' MB RAM!');
                closeResizeModal();
                loadVMs(currentVMNodeId);
            })
            .catch(err => alert('Gagal mengubah spek VM: ' + err.message));
        }

        // Expand Disk
        let currentExpandDiskVMId = null;
        function openExpandDiskModal(vmId, vmName) {
            currentExpandDiskVMId = vmId;
            document.getElementById('expandDiskTitle').innerText = vmName || vmId;
            document.getElementById('expandSizeInput').value = '';
            document.getElementById('expandDiskModal').classList.remove('hidden');
        }
        function closeExpandDiskModal() {
            document.getElementById('expandDiskModal').classList.add('hidden');
            currentExpandDiskVMId = null;
        }
        function submitExpandDisk(e) {
            e.preventDefault();
            if (!currentVMNodeId || !currentExpandDiskVMId) return;

            const sizeGB = parseInt(document.getElementById('expandSizeInput').value, 10);
            if (!sizeGB || sizeGB <= 0) return;

            fetchJson('/api/v1/nodes/' + currentVMNodeId + '/vms/' + currentExpandDiskVMId + '/expand-disk', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ new_size_gb: sizeGB })
            })
            .then(() => {
                alert('Hard Disk VHDX berhasil diperbesar menjadi ' + sizeGB + ' GB!');
                closeExpandDiskModal();
                loadVMs(currentVMNodeId);
            })
            .catch(err => alert('Gagal memperbesar disk VM: ' + err.message));
        }

        // Clone VM
        let currentCloneVMId = null;
        function openCloneModal(vmId, vmName) {
            currentCloneVMId = vmId;
            document.getElementById('cloneVMTitle').innerText = vmName || vmId;
            document.getElementById('cloneNameInput').value = (vmName ? vmName + '-clone' : 'vm-clone');
            document.getElementById('cloneVMModal').classList.remove('hidden');
        }
        function closeCloneModal() {
            document.getElementById('cloneVMModal').classList.add('hidden');
            currentCloneVMId = null;
        }
        function submitCloneVM(e) {
            e.preventDefault();
            if (!currentVMNodeId || !currentCloneVMId) return;

            const newName = document.getElementById('cloneNameInput').value.trim();
            if (!newName) return;

            const btn = e.target.querySelector('button[type="submit"]');
            btn.disabled = true;
            btn.innerHTML = '<i class=\"fa-solid fa-spinner fa-spin\"></i> Mengkloning...';

            fetchJson('/api/v1/nodes/' + currentVMNodeId + '/vms/' + currentCloneVMId + '/clone', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ new_name: newName })
            })
            .then(cloned => {
                btn.disabled = false;
                btn.innerHTML = '<i class=\"fa-solid fa-clone\"></i> Mulai Kloning VM';
                alert('VM berhasil dikloning dengan nama \"' + cloned.name + '\"!');
                closeCloneModal();
                loadVMs(currentVMNodeId);
            })
            .catch(err => {
                btn.disabled = false;
                btn.innerHTML = '<i class=\"fa-solid fa-clone\"></i> Mulai Kloning VM';
                alert('Gagal mengkloning VM: ' + err.message);
            });
        }

        function escapeHtml(str) {
            return String(str || '')
                .replace(/&/g, '&amp;')
                .replace(/</g, '&lt;')
                .replace(/>/g, '&gt;')
                .replace(/"/g, '&quot;')
                .replace(/'/g, '&#39;');
        }

        fetch('/api/v1/cluster/history')
            .then(r => r.json())
            .then(points => renderHistory(points))
            .catch(() => {});

        connectWS();
        fetchNodesManual();
    </script>
</body>
</html>`
