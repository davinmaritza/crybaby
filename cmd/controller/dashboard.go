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

    <script>
        function openUSBModal() { document.getElementById('usbModal').classList.remove('hidden'); }
        function closeUSBModal() { document.getElementById('usbModal').classList.add('hidden'); }
        function downloadUSBPack() {
            window.location.href = '/api/v1/provision/usb-profile';
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
                const hypervBadge = n.hardware.hyperv_enabled
                    ? '<span class=\"text-emerald-400 font-semibold\"><i class=\"fa-solid fa-circle-check mr-1\"></i>Ready</span>'
                    : '<span class=\"text-amber-400 font-semibold\"><i class=\"fa-solid fa-triangle-exclamation mr-1\"></i>Disabled</span>';

                html += '<tr class=\"hover:bg-gray-800/30 transition\">' +
                    '<td class=\"px-6 py-4 font-medium text-white flex items-center gap-2\"><i class=\"fa-brands fa-windows text-blue-400\"></i>' + escapeHtml(n.name) + '</td>' +
                    '<td class=\"px-6 py-4\">' + statusBadge + '</td>' +
                    '<td class=\"px-6 py-4 font-mono text-xs\"><span class=\"text-indigo-400 font-semibold\">' + (n.mesh_ip || '-') + '</span><br><span class=\"text-gray-500\">' + (n.ip_address || '-') + '</span></td>' +
                    '<td class=\"px-6 py-4 text-xs text-gray-300\">' + escapeHtml(n.hardware.cpu_model || 'Unknown CPU') + '<br><span class=\"text-gray-500\">' + n.hardware.cpu_cores + ' Cores &bull; ' + (n.hardware.total_ram_bytes / (1024*1024*1024)).toFixed(0) + ' GB RAM</span></td>' +
                    '<td class=\"px-6 py-4 text-xs font-mono\"><span class=\"text-white font-semibold\">CPU: ' + cpu + '</span><br><span class=\"text-gray-400\">RAM: ' + ram + '</span></td>' +
                    '<td class=\"px-6 py-4 text-xs\">' + hypervBadge + '</td>' +
                    '<td class=\"px-6 py-4 text-right space-x-1.5\">' +
                        (!isOnline ? '<button onclick=\"triggerWoL(\'' + n.id + '\')\" class=\"px-2.5 py-1 rounded bg-amber-500/10 hover:bg-amber-500/20 text-amber-400 text-xs font-medium border border-amber-500/20 transition\"><i class=\"fa-solid fa-power-off mr-1\"></i>WoL</button>' : '') +
                        '<button onclick=\"openTerminal(\'' + n.id + '\')\" class=\"px-2.5 py-1 rounded bg-indigo-500/10 hover:bg-indigo-500/20 text-indigo-400 text-xs font-medium border border-indigo-500/20 transition\"><i class=\"fa-solid fa-terminal mr-1\"></i>Shell</button>' +
                    '</td>' +
                '</tr>';
            }
            tbody.innerHTML = html;
        }

        function triggerWoL(nodeId) {
            fetch('/api/v1/nodes/' + nodeId + '/wol', { method: 'POST' })
                .then(r => r.json())
                .then(d => alert('Wake-on-LAN packet terkirim ke MAC: ' + d.mac))
                .catch(err => alert('Gagal mengirim WoL: ' + err));
        }

        function openTerminal(nodeId) {
            alert('Membuka terminal shell untuk Node ID: ' + nodeId);
        }

        function fetchNodesManual() {
            fetch('/api/v1/nodes')
                .then(r => r.json())
                .then(nodes => renderNodes(nodes));
        }

        function escapeHtml(str) {
            return (str || '').replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
        }

        connectWS();
        fetchNodesManual();
    </script>
</body>
</html>`
