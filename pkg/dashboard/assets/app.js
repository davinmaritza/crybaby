// State variables
let authenticated = false;
let servers = [];
let clusters = [];
let currentTab = 'fleet-section';
let searchKeyword = '';
let activeTerminalServerId = null;
let activeFileServerId = null;

// DOM Elements
const loginContainer = document.getElementById('login-container');
const dashboardContainer = document.getElementById('dashboard-container');
const loginForm = document.getElementById('login-form');
const loginError = document.getElementById('login-error');
const passwordInput = document.getElementById('password');
const btnLogout = document.getElementById('btn-logout');
const connectionStatus = document.getElementById('connection-status');

const navItems = document.querySelectorAll('.nav-item');
const tabContents = document.querySelectorAll('.tab-content');

const pendingPanel = document.getElementById('pending-panel');
const pendingList = document.getElementById('pending-list');
const serverTableBody = document.getElementById('server-table-body');
const searchInput = document.getElementById('search-input');

const createClusterForm = document.getElementById('create-cluster-form');
const clusterNameInput = document.getElementById('cluster-name');
const clusterDescInput = document.getElementById('cluster-desc');
const clustersTableBody = document.getElementById('clusters-table-body');

const auditTableBody = document.getElementById('audit-table-body');
const refreshAuditBtn = document.getElementById('refresh-audit-btn');

const terminalModal = document.getElementById('terminal-modal');
const terminalOutput = document.getElementById('term-output');
const terminalForm = document.getElementById('term-form');
const terminalInput = document.getElementById('term-input');

const fileModal = document.getElementById('file-modal');
const fileDownloadForm = document.getElementById('file-download-form');
const dlPathInput = document.getElementById('dl-path');
const fileProgress = document.getElementById('file-progress');

const aiForm = document.getElementById('ai-form');
const aiInput = document.getElementById('ai-input');
const aiMessages = document.getElementById('ai-messages');

// Initialize
checkAuth();

async function checkAuth() {
    try {
        const res = await fetch('/api/servers');
        if (res.ok) {
            showDashboard();
        } else {
            showLogin();
        }
    } catch (e) {
        showLogin();
    }
}

function showLogin() {
    authenticated = false;
    loginContainer.classList.remove('hidden');
    dashboardContainer.classList.add('hidden');
}

function showDashboard() {
    authenticated = true;
    loginContainer.classList.add('hidden');
    dashboardContainer.classList.remove('hidden');
    setupNavigation();
    setupEventListeners();
    fetchFleetData();
    connectWebSocket();
}

loginForm.addEventListener('submit', async (e) => {
    e.preventDefault();
    const pass = passwordInput.value;
    try {
        const res = await fetch('/api/login', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ password: pass })
        });

        if (res.ok) {
            showDashboard();
            passwordInput.value = '';
        } else {
            loginError.textContent = 'ACCESS DENIED: INVALID PASSWORD';
        }
    } catch (err) {
        loginError.textContent = 'CONNECTION ERROR';
    }
});

btnLogout.addEventListener('click', async () => {
    await fetch('/api/logout', { method: 'POST' });
    showLogin();
});

// Navigation
function setupNavigation() {
    navItems.forEach(btn => {
        btn.addEventListener('click', () => {
            navItems.forEach(b => b.classList.remove('active'));
            tabContents.forEach(c => {
                c.classList.remove('active');
                c.classList.add('hidden');
            });

            btn.classList.add('active');
            const target = btn.dataset.target;
            const targetEl = document.getElementById(target);
            targetEl.classList.remove('hidden');
            targetEl.classList.add('active');
            currentTab = target;

            if (currentTab === 'clusters-section') {
                renderClustersTable();
            } else if (currentTab === 'audit-section') {
                fetchAuditLogs();
            }
        });
    });
}

function setupEventListeners() {
    if (searchInput) {
        searchInput.addEventListener('input', (e) => {
            searchKeyword = e.target.value.toLowerCase();
            renderDashboard();
        });
    }

    const btnCloseFile = document.getElementById('close-file-btn');
    const btnCloseTerminal = document.getElementById('close-terminal-btn');
    const backdrop = document.getElementById('modal-backdrop');

    const closeAllModals = () => {
        terminalModal.classList.add('hidden');
        fileModal.classList.add('hidden');
        backdrop.classList.add('hidden');
    };

    if (btnCloseTerminal) btnCloseTerminal.addEventListener('click', closeAllModals);
    if (btnCloseFile) btnCloseFile.addEventListener('click', closeAllModals);
    if (backdrop) backdrop.addEventListener('click', closeAllModals);

    // Create Cluster Form
    if (createClusterForm) {
        createClusterForm.addEventListener('submit', async (e) => {
            e.preventDefault();
            const name = clusterNameInput.value.trim();
            const description = clusterDescInput.value.trim();

            try {
                const res = await fetch('/api/clusters', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ name, description })
                });

                if (res.ok) {
                    clusterNameInput.value = '';
                    clusterDescInput.value = '';
                    fetchFleetData();
                }
            } catch (err) {
                console.error(err);
            }
        });
    }

    // Shell Terminal Form
    if (terminalForm) {
        terminalForm.addEventListener('submit', async (e) => {
            e.preventDefault();
            const command = terminalInput.value.trim();
            if (!command || !activeTerminalServerId) return;

            terminalInput.value = '';
            appendTerminalLine(`root@node:~# ${command}`, 'command');

            try {
                const res = await fetch(`/api/servers/${activeTerminalServerId}/exec`, {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ command })
                });

                if (res.ok) {
                    const data = await res.json();
                    if (data.error) {
                        appendTerminalLine(`[Error] ${data.error}`, 'error');
                    }
                    if (data.output) {
                        appendTerminalLine(data.output, 'output');
                    } else if (!data.error) {
                        appendTerminalLine("[Completed with no output]", 'secondary');
                    }
                } else {
                    appendTerminalLine("Execution failed: Server error", 'error');
                }
            } catch (err) {
                appendTerminalLine(`Connection failure: ${err}`, 'error');
            }
        });
    }

    // File download
    if (fileDownloadForm) {
        fileDownloadForm.addEventListener('submit', (e) => {
            e.preventDefault();
            const path = dlPathInput.value.trim();
            if (!path || !activeFileServerId) return;

            fileProgress.textContent = "Initiating download...";
            const url = `/api/servers/${activeFileServerId}/file/get?path=${encodeURIComponent(path)}`;
            
            const a = document.createElement('a');
            a.href = url;
            a.download = '';
            document.body.appendChild(a);
            a.click();
            document.body.removeChild(a);
            fileProgress.textContent = "Download triggered.";
        });
    }

    // Refresh audit
    if (refreshAuditBtn) {
        refreshAuditBtn.addEventListener('click', fetchAuditLogs);
    }

    // AI Form
    if (aiForm) {
        aiForm.addEventListener('submit', async (e) => {
            e.preventDefault();
            const text = aiInput.value.trim();
            if (!text) return;

            aiInput.value = '';
            appendAIMessage(text, 'user');

            const loadingMsg = appendAIMessage('Thinking...', 'ai');

            try {
                const res = await fetch('/api/ai/chat', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ message: text })
                });

                if (res.ok) {
                    const data = await res.json();
                    loadingMsg.textContent = data.response;
                } else {
                    loadingMsg.textContent = 'AI request failed.';
                }
            } catch (err) {
                loadingMsg.textContent = `AI Error: ${err}`;
            }
        });
    }
}

// Fetch Fleet Data
async function fetchFleetData() {
    try {
        const [resServers, resClusters] = await Promise.all([
            fetch('/api/servers'),
            fetch('/api/clusters')
        ]);

        if (resServers.ok) {
            const data = await resServers.json();
            servers = Array.isArray(data) ? data : [];
        }
        if (resClusters.ok) {
            const data = await resClusters.json();
            clusters = Array.isArray(data) ? data : [];
        }

        renderDashboard();
    } catch (err) {
        console.error("Failed to fetch fleet data:", err);
    }
}

async function fetchAuditLogs() {
    try {
        const res = await fetch('/api/logs');
        if (res.ok) {
            const logs = await res.json();
            renderLogs(Array.isArray(logs) ? logs : []);
        }
    } catch (e) {
        console.error(e);
    }
}

// WebSocket Connection
function connectWebSocket() {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/ws`;

    connectionStatus.textContent = "Connecting...";

    const socket = new WebSocket(wsUrl);

    socket.onopen = () => {
        connectionStatus.textContent = "Connected";
        connectionStatus.style.color = "#4caf50";
    };

    socket.onmessage = (event) => {
        try {
            const msg = JSON.parse(event.data);
            handleWSMessage(msg);
        } catch (e) {
            console.error("Malformed WS message:", e);
        }
    };

    socket.onclose = () => {
        connectionStatus.textContent = "Disconnected (Retrying...)";
        connectionStatus.style.color = "#ff5252";
        setTimeout(connectWebSocket, 3000);
    };
}

function handleWSMessage(msg) {
    if (msg.type === 'fleet_update' || msg.type === 'metrics' || msg.type === 'server_status') {
        fetchFleetData();
    }
}

// Render Operations
function renderDashboard() {
    const filteredServers = servers.filter(s => {
        if (s.status === 'pending_approval') return false;
        if (searchKeyword) {
            const name = (s.custom_name || '').toLowerCase();
            const host = s.hostname.toLowerCase();
            if (!name.includes(searchKeyword) && !host.includes(searchKeyword)) {
                return false;
            }
        }
        return true;
    });

    // Calculate Global Fleet Stats
    let onlineCount = 0;
    let offlineCount = 0;
    let totalCores = 0;
    let totalRAM = 0;
    let totalUsedRAM = 0;
    let totalCPULoadAcc = 0;
    
    servers.forEach(s => {
        if (s.status === 'pending_approval') return;
        if (s.is_online) onlineCount++;
        else offlineCount++;

        totalCores += s.cpu_cores || 0;
        totalRAM += s.ram_total_mb || 0;

        if (s.is_online && s.recent_metrics) {
            totalCPULoadAcc += s.recent_metrics.cpu_load_pct || 0;
            totalUsedRAM += s.recent_metrics.ram_used_mb || 0;
        }
    });

    document.getElementById('kpi-online').textContent = onlineCount;
    document.getElementById('kpi-offline').textContent = offlineCount;

    // Update CPU Gauge
    const avgCPU = onlineCount > 0 ? (totalCPULoadAcc / onlineCount) : 0;
    document.getElementById('val-cpu').textContent = avgCPU.toFixed(1) + '%';
    document.getElementById('lbl-cpu').textContent = `of ${totalCores} CPU(s)`;
    const cpuDash = (avgCPU / 100) * 126;
    document.getElementById('gauge-cpu').style.strokeDasharray = `${cpuDash} 126`;

    // Update RAM Gauge
    const ramPct = totalRAM > 0 ? (totalUsedRAM / totalRAM) * 100 : 0;
    document.getElementById('val-ram').textContent = ramPct.toFixed(1) + '%';
    document.getElementById('lbl-ram').textContent = `${(totalUsedRAM/1024).toFixed(1)} GiB of ${(totalRAM/1024).toFixed(1)} GiB`;
    const ramDash = (ramPct / 100) * 126;
    document.getElementById('gauge-ram').style.strokeDasharray = `${ramDash} 126`;

    // Render Pending Panel
    const pendingDevices = servers.filter(s => s.status === 'pending_approval');
    renderPendingPanel(pendingDevices);

    // Render Active Servers Table
    renderServersTable(filteredServers);

    if (currentTab === 'clusters-section') {
        renderClustersTable();
    }
}

function renderPendingPanel(devices) {
    if (!pendingPanel) return;

    if (devices.length === 0) {
        pendingPanel.classList.add('hidden');
        return;
    }

    pendingPanel.classList.remove('hidden');
    pendingList.innerHTML = '';

    devices.forEach(d => {
        const row = document.createElement('div');
        row.style.cssText = 'display: flex; justify-content: space-between; align-items: center; margin-bottom: 8px; background: #333; padding: 8px; border-radius: 3px;';
        
        row.innerHTML = `
            <div>
                <strong>${d.hostname}</strong> (${d.os_version}) - RAM: ${(d.ram_total_mb/1024).toFixed(1)}GB
                <div style="font-size: 11px; color: #aaa;">UUID: ${d.id}</div>
            </div>
            <div>
                <button class="btn-primary" onclick="approveAgent('${d.id}')">Approve</button>
                <button class="btn-secondary" onclick="rejectAgent('${d.id}')" style="color: #ff5252; border-color: #ff5252;">Reject</button>
            </div>
        `;
        pendingList.appendChild(row);
    });
}

async function approveAgent(id) {
    if (confirm("Approve device and grant access?")) {
        const res = await fetch(`/api/servers/${id}/approve`, { method: 'POST' });
        if (res.ok) fetchFleetData();
    }
}

async function rejectAgent(id) {
    if (confirm("Reject device access?")) {
        const res = await fetch(`/api/servers/${id}/reject`, { method: 'POST' });
        if (res.ok) fetchFleetData();
    }
}

function renderServersTable(filtered) {
    if (!serverTableBody) return;
    serverTableBody.innerHTML = '';

    if (filtered.length === 0) {
        serverTableBody.innerHTML = `<tr><td colspan="8" style="text-align: center; padding: 2rem; color: #888;">No active nodes connected</td></tr>`;
        return;
    }

    filtered.forEach(s => {
        const row = document.createElement('tr');
        
        const statusIcon = s.is_online ? `<span style="color: #4caf50;">✔</span>` : `<span style="color: #ff5252;">✖</span>`;

        let uptimeText = '—';
        if (s.is_online && s.recent_metrics) {
            const up = s.recent_metrics.uptime_seconds;
            const days = Math.floor(up / 86400);
            const hrs = Math.floor((up % 86400) / 3600);
            const mins = Math.floor((up % 3600) / 60);
            if (days > 0) uptimeText = `${days}d ${hrs}h`;
            else if (hrs > 0) uptimeText = `${hrs}h ${mins}m`;
            else uptimeText = `${mins}m`;
        }

        let cpuFill = 0;
        let ramFill = 0;
        if (s.is_online && s.recent_metrics) {
            cpuFill = s.recent_metrics.cpu_load_pct;
            const usedRAM = s.recent_metrics.ram_used_mb;
            const totalRAM = s.ram_total_mb;
            ramFill = totalRAM > 0 ? (usedRAM / totalRAM) * 100 : 0;
        }

        let clusterOpts = `<option value="" style="background: #333; color: white;">Unassigned</option>`;
        clusters.forEach(c => {
            const sel = s.cluster_id === c.id ? 'selected' : '';
            clusterOpts += `<option value="${c.id}" ${sel} style="background: #333; color: white;">${c.name}</option>`;
        });

        const nameDisplay = s.custom_name 
            ? `<strong>${s.custom_name}</strong> <span style="color: #888;">(${s.hostname})</span>`
            : `<strong>${s.hostname}</strong>`;

        row.innerHTML = `
            <td>${s.id.substring(0,6)}</td>
            <td>${statusIcon}</td>
            <td>${nameDisplay}</td>
            <td>
                <select onchange="setServerCluster('${s.id}', this.value)" style="background: transparent; color: white; border: 1px solid #555;">
                    ${clusterOpts}
                </select>
            </td>
            <td>${cpuFill.toFixed(0)}%</td>
            <td>${ramFill.toFixed(0)}% (${(s.ram_total_mb/1024).toFixed(1)}GB)</td>
            <td>${uptimeText}</td>
            <td>
                <button class="btn-secondary" onclick="renameServerPrompt('${s.id}', '${s.custom_name || ''}')">Rename</button>
                <button class="btn-primary" onclick="openTerminal('${s.id}', '${s.custom_name || s.hostname}')" ${!s.is_online ? 'disabled' : ''}>>_ Shell</button>
                <button class="btn-secondary" onclick="openFileManager('${s.id}', '${s.custom_name || s.hostname}')" ${!s.is_online ? 'disabled' : ''}>📁 File</button>
                <button class="btn-secondary" onclick="decommissionServer('${s.id}')" style="color: #ff5252; border-color: #ff5252;">Remove</button>
            </td>
        `;

        serverTableBody.appendChild(row);
    });
}

async function setServerCluster(serverId, clusterId) {
    await fetch(`/api/servers/${serverId}/cluster`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ cluster_id: clusterId })
    });
    fetchFleetData();
}

async function renameServerPrompt(id, currentName) {
    const newName = prompt("Enter custom name for this server:", currentName);
    if (newName !== null) {
        const res = await fetch(`/api/servers/${id}/rename`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ custom_name: newName.trim() })
        });
        if (res.ok) fetchFleetData();
    }
}

async function decommissionServer(id) {
    if (confirm("Are you sure you want to remove this server?")) {
        const res = await fetch(`/api/servers/${id}/remove`, { method: 'POST' });
        if (res.ok) fetchFleetData();
    }
}

// Cluster management
function renderClustersTable() {
    if (!clustersTableBody) return;
    clustersTableBody.innerHTML = '';
    
    if (clusters.length === 0) {
        clustersTableBody.innerHTML = `<tr><td colspan="4" style="text-align: center; padding: 2rem; color: #888;">No clusters created yet</td></tr>`;
        return;
    }

    clusters.forEach(c => {
        const row = document.createElement('tr');
        row.innerHTML = `
            <td><strong>${c.name}</strong></td>
            <td>${c.description || '—'}</td>
            <td>${c.server_count}</td>
            <td>
                <button class="btn-secondary" onclick="deleteCluster('${c.id}')" style="color: #ff5252; border-color: #ff5252;">Delete</button>
            </td>
        `;
        clustersTableBody.appendChild(row);
    });
}

async function deleteCluster(id) {
    if (confirm("Delete this cluster? Servers inside will be unassigned.")) {
        const res = await fetch(`/api/clusters/${id}`, { method: 'DELETE' });
        if (res.ok) fetchFleetData();
    }
}

// Logs Rendering
function renderLogs(logs) {
    if (!auditTableBody) return;
    auditTableBody.innerHTML = '';
    
    if (!logs || logs.length === 0) {
        auditTableBody.innerHTML = `<tr><td colspan="5" style="text-align: center; padding: 2rem; color: #888;">No audit logs found</td></tr>`;
        return;
    }

    logs.forEach(l => {
        const row = document.createElement('tr');
        let resultSnippet = '—';
        if (l.result) {
            resultSnippet = l.result.length > 60 ? l.result.slice(0, 60) + '...' : l.result;
        }

        row.innerHTML = `
            <td>${new Date(l.issued_at).toLocaleString()}</td>
            <td>${l.server_name || 'Global'}</td>
            <td>${l.issued_by}</td>
            <td><code>${l.command}</code></td>
            <td style="white-space: pre-wrap; font-family: monospace; font-size: 11px;">${resultSnippet}</td>
        `;
        auditTableBody.appendChild(row);
    });
}

// Terminal Shell Modal
function openTerminal(serverId, name) {
    activeTerminalServerId = serverId;
    document.getElementById('term-server-name').textContent = `Shell - ${name}`;
    terminalOutput.innerHTML = `<div style="color: #888;">[Connected to PowerShell/Shell Session]</div>`;
    terminalModal.classList.remove('hidden');
    document.getElementById('modal-backdrop').classList.remove('hidden');
    terminalInput.focus();
}

function appendTerminalLine(text, type) {
    const div = document.createElement('div');
    div.style.margin = '2px 0';
    if (type === 'command') div.style.color = '#fff';
    else if (type === 'error') div.style.color = '#ff5252';
    else if (type === 'output') div.style.color = '#0f0';
    else div.style.color = '#888';
    
    div.textContent = text;
    terminalOutput.appendChild(div);
    terminalOutput.scrollTop = terminalOutput.scrollHeight;
}

// File Manager Modal
function openFileManager(serverId, name) {
    activeFileServerId = serverId;
    document.getElementById('file-server-name').textContent = `File Download - ${name}`;
    fileProgress.textContent = '';
    dlPathInput.value = '';
    fileModal.classList.remove('hidden');
    document.getElementById('modal-backdrop').classList.remove('hidden');
}

// AI Message Helper
function appendAIMessage(text, sender) {
    const div = document.createElement('div');
    div.style.cssText = `margin-bottom: 6px; padding: 6px; border-radius: 3px; font-size: 12px; ${sender === 'user' ? 'background: #2196F3; color: white; align-self: flex-end;' : 'background: #333; color: #e0e0e0;'}`;
    div.textContent = `${sender === 'user' ? 'You' : 'AI'}: ${text}`;
    aiMessages.appendChild(div);
    aiMessages.scrollTop = aiMessages.scrollHeight;
    return div;
}
