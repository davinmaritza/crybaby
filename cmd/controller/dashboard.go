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
            <button onclick="openUSBModal()" class="px-3.5 py-1.5 rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-semibold flex items-center gap-2 transition shadow-md shadow-indigo-600/20">
                <i class="fa-solid fa-usb"></i>
                Generate Node USB
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
                                Klik <b class="text-indigo-400">"Generate Node USB"</b> di kanan atas untuk membuat flashdisk installer pertama Anda.
                            </td>
                        </tr>
                    </tbody>
                </table>
            </div>
        </div>
    </main>

    <!-- USB Provisioning Modal -->
    <div id="usbModal" class="fixed inset-0 bg-black/75 backdrop-blur-sm z-50 flex items-center justify-center hidden p-4">
        <div class="glass max-w-lg w-full rounded-2xl border border-gray-700 p-6 space-y-4 shadow-2xl">
            <div class="flex justify-between items-center border-b border-gray-800 pb-3">
                <h3 class="font-bold text-white text-base flex items-center gap-2">
                    <i class="fa-solid fa-usb text-indigo-400"></i>
                    1-Click USB Node Provisioner
                </h3>
                <button onclick="closeUSBModal()" class="text-gray-400 hover:text-white">&times;</button>
            </div>
            
            <p class="text-xs text-gray-300 leading-relaxed">
                Colok USB flashdisk ke laptop ini, salin file <b>crybaby-agent.exe</b> dan <b>install.bat</b> ke dalam flashdisk. Saat dicolok ke PC Windows 11, jalankan <code>install.bat</code> sekali saja.
            </p>

            <div class="space-y-2 bg-gray-950 p-3.5 rounded-xl border border-gray-800 text-xs font-mono">
                <div class="flex justify-between text-gray-400">
                    <span>Controller Endpoint:</span>
                    <span class="text-white" id="modalEndpoint">http://127.0.0.1:{{PORT}}</span>
                </div>
                <div class="flex justify-between text-gray-400">
                    <span>Cluster Token:</span>
                    <span class="text-indigo-400 select-all" id="modalToken">{{CLUSTER_TOKEN}}</span>
                </div>
            </div>

            <div class="bg-indigo-950/40 border border-indigo-500/20 rounded-xl p-3 text-xs text-indigo-200">
                <i class="fa-solid fa-circle-info mr-1 text-indigo-400"></i>
                Installer akan otomatis memasang agent sebagai <b>Windows Service</b> dan mengonfigurasi skema <b>Anti-Sleep</b>. Flashdisk bisa dicabut setelah instalasi selesai.
            </div>

            <div class="flex justify-end gap-2 pt-2">
                <button onclick="closeUSBModal()" class="px-4 py-2 rounded-lg bg-gray-800 hover:bg-gray-700 text-xs font-semibold text-gray-300">Tutup</button>
                <button onclick="downloadUSBPack()" class="px-4 py-2 rounded-lg bg-indigo-600 hover:bg-indigo-500 text-xs font-semibold text-white flex items-center gap-2">
                    <i class="fa-solid fa-download"></i> Unduh Paket USB (.zip)
                </button>
            </div>
        </div>
    </div>

    <!-- Hyper-V VM Management Modal -->
    <div id="vmModal" class="fixed inset-0 bg-black/75 backdrop-blur-sm z-50 flex items-center justify-center hidden p-4">
        <div class="glass max-w-3xl w-full rounded-2xl border border-gray-700 p-6 space-y-5 shadow-2xl max-h-[90vh] overflow-y-auto">
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
                            <th class="px-4 py-2.5">Specs</th>
                            <th class="px-4 py-2.5">VHDX Path</th>
                            <th class="px-4 py-2.5 text-right">Actions</th>
                        </tr>
                    </thead>
                    <tbody id="vmTableBody" class="divide-y divide-gray-800/40 font-mono">
                        <tr>
                            <td colspan="5" class="px-4 py-6 text-center text-gray-500">Memuat VM...</td>
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

    <script>
        function openUSBModal() { 
            const ep = document.getElementById('modalEndpoint');
            if (ep) ep.innerText = window.location.origin;
            document.getElementById('usbModal').classList.remove('hidden'); 
        }
        function closeUSBModal() { document.getElementById('usbModal').classList.add('hidden'); }
        function downloadUSBPack() {
            window.location.href = '/api/v1/provision/usb-zip';
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

                const cpu = (n.latest_metrics && n.latest_metrics.cpu_usage_pct !== undefined) ? n.latest_metrics.cpu_usage_pct.toFixed(1) + '%' : '-';
                const ram = (n.latest_metrics && n.latest_metrics.ram_used_bytes) ? (n.latest_metrics.ram_used_bytes / (1024*1024*1024)).toFixed(1) + ' GB' : '-';
                const hw = n.hardware || {};
                const cpuCores = hw.cpu_cores || 0;
                const totalRamGB = hw.total_ram_bytes ? (hw.total_ram_bytes / (1024*1024*1024)).toFixed(0) : '0';
                const hypervBadge = hw.hyperv_enabled
                    ? '<span class=\"text-emerald-400 font-semibold\"><i class=\"fa-solid fa-circle-check mr-1\"></i>Ready</span>'
                    : '<span class=\"text-amber-400 font-semibold\"><i class=\"fa-solid fa-triangle-exclamation mr-1\"></i>Disabled</span>';

                html += '<tr class=\"hover:bg-gray-800/30 transition\">' +
                    '<td class=\"px-6 py-4 font-medium text-white flex items-center gap-2\"><i class=\"fa-brands fa-windows text-blue-400\"></i>' + escapeHtml(n.name) + '</td>' +
                    '<td class=\"px-6 py-4\">' + statusBadge + '</td>' +
                    '<td class=\"px-6 py-4 font-mono text-xs\"><span class=\"text-indigo-400 font-semibold\">' + (n.mesh_ip || '-') + '</span><br><span class=\"text-gray-500\">' + (n.ip_address || '-') + '</span></td>' +
                    '<td class=\"px-6 py-4 text-xs text-gray-300\">' + escapeHtml(hw.cpu_model || 'Unknown CPU') + '<br><span class=\"text-gray-500\">' + cpuCores + ' Cores &bull; ' + totalRamGB + ' GB RAM</span></td>' +
                    '<td class=\"px-6 py-4 text-xs font-mono\"><span class=\"text-white font-semibold\">CPU: ' + cpu + '</span><br><span class=\"text-gray-400\">RAM: ' + ram + '</span></td>' +
                    '<td class=\"px-6 py-4 text-xs\">' + hypervBadge + '</td>' +
                    '<td class=\"px-6 py-4 text-right space-x-1.5\">' +
                        (!isOnline ? '<button onclick=\"triggerWoL(\'' + escapeHtml(n.id) + '\')\" class=\"px-2.5 py-1 rounded bg-amber-500/10 hover:bg-amber-500/20 text-amber-400 text-xs font-medium border border-amber-500/20 transition\"><i class=\"fa-solid fa-power-off mr-1\"></i>WoL</button>' : '') +
                        '<button onclick=\"openVMModal(\'' + escapeHtml(n.id) + '\', \'' + escapeHtml(n.name) + '\')\" class=\"px-2.5 py-1 rounded bg-emerald-500/10 hover:bg-emerald-500/20 text-emerald-400 text-xs font-medium border border-emerald-500/20 transition\"><i class=\"fa-solid fa-cube mr-1\"></i>VMs</button>' +
                        '<button onclick=\"openTerminal(\'' + escapeHtml(n.id) + '\', \'' + escapeHtml(n.name) + '\')\" class=\"px-2.5 py-1 rounded bg-indigo-500/10 hover:bg-indigo-500/20 text-indigo-400 text-xs font-medium border border-indigo-500/20 transition\"><i class=\"fa-solid fa-terminal mr-1\"></i>Shell</button>' +
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
            tbody.innerHTML = '<tr><td colspan=\"5\" class=\"px-4 py-6 text-center text-gray-500\">Memuat VM Hyper-V...</td></tr>';
            
            fetchJson('/api/v1/nodes/' + nodeId + '/vms')
                .then(vms => {
                    if (!vms || vms.length === 0) {
                        tbody.innerHTML = '<tr><td colspan=\"5\" class=\"px-4 py-6 text-center text-gray-500\">Belum ada VM di node ini. Buat VM baru menggunakan form di atas.</td></tr>';
                        return;
                    }
                    let html = '';
                    vms.forEach(vm => {
                        const isRunning = vm.state === 'RUNNING';
                        const badge = isRunning
                            ? '<span class=\"px-2 py-0.5 rounded text-[10px] font-semibold bg-emerald-500/10 text-emerald-400 border border-emerald-500/20\">RUNNING</span>'
                            : '<span class=\"px-2 py-0.5 rounded text-[10px] font-semibold bg-gray-500/10 text-gray-400 border border-gray-500/20\">STOPPED</span>';
                        
                        html += '<tr class=\"hover:bg-gray-800/20\">' +
                            '<td class=\"px-4 py-3 font-bold text-white\">' + escapeHtml(vm.name) + '</td>' +
                            '<td class=\"px-4 py-3\">' + badge + '</td>' +
                            '<td class=\"px-4 py-3 text-gray-300\">' + (vm.cpu_count || 2) + ' vCPU &bull; ' + (vm.memory_mb || 2048) + ' MB</td>' +
                            '<td class=\"px-4 py-3 text-gray-500 truncate max-w-xs\" title=\"' + (vm.disk_path || '-') + '\">' + (vm.disk_path || '-') + '</td>' +
                            '<td class=\"px-4 py-3 text-right space-x-1\">' +
                                (!isRunning 
                                    ? '<button onclick=\"controlVM(\'' + vm.id + '\', \'start\')\" class=\"px-2 py-1 rounded bg-emerald-600/20 text-emerald-400 hover:bg-emerald-600/30\"><i class=\"fa-solid fa-play\"></i></button>'
                                    : '<button onclick=\"controlVM(\'' + vm.id + '\', \'stop\')\" class=\"px-2 py-1 rounded bg-amber-600/20 text-amber-400 hover:bg-amber-600/30\"><i class=\"fa-solid fa-stop\"></i></button>' +
                                      '<button onclick=\"controlVM(\'' + vm.id + '\', \'restart\')\" class=\"px-2 py-1 rounded bg-blue-600/20 text-blue-400 hover:bg-blue-600/30\"><i class=\"fa-solid fa-arrows-rotate\"></i></button>') +
                                '<button onclick=\"deleteVM(\'' + vm.id + '\')\" class=\"px-2 py-1 rounded bg-red-600/20 text-red-400 hover:bg-red-600/30\"><i class=\"fa-solid fa-trash\"></i></button>' +
                            '</td>' +
                        '</tr>';
                    });
                    tbody.innerHTML = html;
                })
                .catch(err => {
                    tbody.innerHTML = '<tr><td colspan=\"5\" class=\"px-4 py-6 text-center text-amber-400/90 font-medium\"><i class=\"fa-solid fa-triangle-exclamation mr-2 text-amber-400\"></i>Gagal memuat VM: ' + escapeHtml(err.message) + '</td></tr>';
                });
        }

        function submitNewVM(e) {
            e.preventDefault();
            if (!currentVMNodeId) return;

            const payload = {
                name: document.getElementById('vmName').value,
                switch_name: document.getElementById('vmSwitch').value,
                cpu_count: parseInt(document.getElementById('vmCores').value, 10),
                memory_mb: parseInt(document.getElementById('vmRAM').value, 10),
                disk_size_gb: parseInt(document.getElementById('vmDisk').value, 10)
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

        function escapeHtml(str) {
            return String(str || '')
                .replace(/&/g, '&amp;')
                .replace(/</g, '&lt;')
                .replace(/>/g, '&gt;')
                .replace(/"/g, '&quot;')
                .replace(/'/g, '&#39;');
        }

        connectWS();
        fetchNodesManual();
    </script>
</body>
</html>`
