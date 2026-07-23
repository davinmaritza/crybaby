// State variables
let authenticated = false;
let servers = [];
let clusters = [];
let currentTab = 'fleet-section';
let selectedClusterFilter = 'all';
let searchKeyword = '';
let activeTerminalServerId = null;
let activeFileServerId = null;

// DOM Elements
const loginContainer = document.getElementById('login-container');
const dashboardContainer = document.getElementById('dashboard-container');
const loginForm = document.getElementById('login-form');
const loginError = document.getElementById('login-error');
const passwordInput = document.getElementById('password');

const navButtons = document.querySelectorAll('.nav-btn');
const tabContents = document.querySelectorAll('.tab-content');
const sectionTitle = document.getElementById('section-title');

const kpiOnline = document.getElementById('kpi-online');
const kpiOffline = document.getElementById('kpi-offline');
const kpiPending = document.getElementById('kpi-pending');
const kpiSpecs = document.getElementById('kpi-specs');
const kpiResources = document.getElementById('kpi-resources');

const coreMapGrid = document.getElementById('core-map-grid');
const pendingPanel = document.getElementById('pending-panel');
const pendingList = document.getElementById('pending-list');
const serverTableBody = document.getElementById('server-table-body');
const clusterFilter = document.getElementById('cluster-filter');
const searchInput = document.getElementById('search-input');

const createClusterForm = document.getElementById('create-cluster-form');
const clusterNameInput = document.getElementById('cluster-name');
const clusterDescInput = document.getElementById('cluster-desc');
const clustersTableBody = document.getElementById('clusters-table-body');

const auditTableBody = document.getElementById('audit-table-body');

const serverDrawer = document.getElementById('server-drawer');
const drawerBodyContent = document.getElementById('drawer-body-content');
const drawerServerName = document.getElementById('drawer-server-name');
const btnCloseDrawer = document.getElementById('btn-close-drawer');

const terminalModal = document.getElementById('terminal-modal');
const terminalTitle = document.getElementById('terminal-title');
const terminalOutput = document.getElementById('terminal-output');
const terminalForm = document.getElementById('terminal-form');
const terminalInput = document.getElementById('terminal-input');
const btnCloseTerminal = document.getElementById('btn-close-terminal');

const fileModal = document.getElementById('file-modal');
const fileTitle = document.getElementById('file-title');
const fileDownloadForm = document.getElementById('file-download-form');
const fileUploadForm = document.getElementById('file-upload-form');
const dlPathInput = document.getElementById('dl-path');
const ulPathInput = document.getElementById('ul-path');
const ulFileInput = document.getElementById('ul-file');
const fileProgress = document.getElementById('file-progress');
const btnCloseFile = document.getElementById('btn-close-file');

const aiForm = document.getElementById('ai-form');
const aiInput = document.getElementById('ai-input');
const aiMessages = document.getElementById('ai-messages');
const btnLogout = document.getElementById('btn-logout');

// Initial Setup
document.addEventListener('DOMContentLoaded', () => {
    checkAuth();
    setupNavigation();
    setupEventListeners();
    // Poll data every 5 seconds
    setInterval(() => {
        if (authenticated) {
            fetchFleetData();
        }
    }, 5000);
});

// Authentication
async function checkAuth() {
    try {
        const res = await fetch('/api/auth/check');
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
    fetchFleetData();
}

loginForm.addEventListener('submit', async (e) => {
    e.preventDefault();
    loginError.textContent = '';
    const password = passwordInput.value;

    try {
        const res = await fetch('/api/login', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ password })
        });

        if (res.ok) {
            showDashboard();
            passwordInput.value = '';
        } else {
            loginError.textContent = 'ACCESS DENIED: INVALID KEY';
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
    navButtons.forEach(btn => {
        btn.addEventListener('click', () => {
            navButtons.forEach(b => b.classList.remove('active'));
            tabContents.forEach(c => c.classList.remove('active'));

            btn.classList.add('active');
            const target = btn.dataset.target;
            document.getElementById(target).classList.add('active');
            currentTab = target;

            // Change title
            if (target === 'fleet-section') {
                sectionTitle.textContent = 'FLEET OVERVIEW';
            } else if (target === 'clusters-section') {
                sectionTitle.textContent = 'CLUSTER CONFIGURATION';
            } else if (target === 'audit-section') {
                sectionTitle.textContent = 'COMMAND AUDIT LOGS';
            }
        });
    });
}

function setupEventListeners() {
    clusterFilter.addEventListener('change', (e) => {
        selectedClusterFilter = e.target.value;
        renderDashboard();
    });

    searchInput.addEventListener('input', (e) => {
        searchKeyword = e.target.value.toLowerCase();
        renderDashboard();
    });

    btnCloseDrawer.addEventListener('click', () => serverDrawer.classList.remove('open'));
    btnCloseTerminal.addEventListener('click', () => terminalModal.classList.remove('open'));
    btnCloseFile.addEventListener('click', () => fileModal.classList.remove('open'));

    // Create Cluster Form
    createClusterForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        const name = clusterNameInput.value;
        const description = clusterDescInput.value;

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

    // Shell Terminal Form
    terminalForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        const command = terminalInput.value.trim();
        if (!command || !activeTerminalServerId) return;

        terminalInput.value = '';
        appendTerminalLine(`PS C:\\> ${command}`, 'command');

        try {
            const res = await fetch(`/api/servers/${activeTerminalServerId}/exec`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ command })
            });

            if (res.ok) {
                const data = await res.json();
                if (data.error) {
                    appendTerminalLine(data.error, 'error');
                }
                if (data.output) {
                    appendTerminalLine(data.output, 'output');
                } else if (!data.error) {
                    appendTerminalLine("[Command completed with no output]", 'secondary');
                }
                appendTerminalLine(`Exit Code: ${data.exit_code}`, 'exit-code');
            } else {
                appendTerminalLine("Failed to execute command: Server error", 'error');
            }
        } catch (err) {
            appendTerminalLine(`Connection failure: ${err}`, 'error');
        }
    });

    // File download
    fileDownloadForm.addEventListener('submit', (e) => {
        e.preventDefault();
        const path = dlPathInput.value.trim();
        if (!path || !activeFileServerId) return;

        fileProgress.textContent = "Initiating chunked download...";
        const url = `/api/servers/${activeFileServerId}/file/get?path=${encodeURIComponent(path)}`;
        
        // Use standard window location or anchor element to trigger browser file download
        const a = document.createElement('a');
        a.href = url;
        a.download = '';
        document.body.appendChild(a);
        a.click();
        document.body.removeChild(a);

        fileProgress.textContent = "Download request triggered. Check browser downloads.";
        setTimeout(() => fileProgress.textContent = '', 5000);
    });

    // File upload
    fileUploadForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        const path = ulPathInput.value.trim();
        const file = ulFileInput.files[0];
        if (!path || !file || !activeFileServerId) return;

        fileProgress.textContent = "Uploading file in chunks...";

        const formData = new FormData();
        formData.append('file', file);

        try {
            const res = await fetch(`/api/servers/${activeFileServerId}/file/put?path=${encodeURIComponent(path)}`, {
                method: 'POST',
                body: formData
            });

            if (res.ok) {
                fileProgress.textContent = "Upload completed successfully.";
                ulFileInput.value = '';
            } else {
                const txt = await res.text();
                fileProgress.textContent = `Upload failed: ${txt}`;
            }
        } catch (err) {
            fileProgress.textContent = `Upload failed: ${err}`;
        }
    });

    // AI Chat Form
    aiForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        const prompt = aiInput.value.trim();
        if (!prompt) return;

        aiInput.value = '';
        appendAIMessage(prompt, 'user');

        const typingMsg = appendAIMessage('Thinking...', 'ai typing');

        try {
            const res = await fetch('/api/ai/chat', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ message: prompt })
            });

            typingMsg.remove();

            if (res.ok) {
                const data = await res.json();
                appendAIMessage(data.reply, 'ai');
                // Refresh data in case AI changed something
                fetchFleetData();
            } else {
                appendAIMessage("Sorry, I encountered an error communicating with the fleet agent layer.", 'ai');
            }
        } catch (err) {
            typingMsg.remove();
            appendAIMessage(`Error connecting: ${err}`, 'ai');
        }
    });
}

// Data Fetching
async function fetchFleetData() {
    try {
        // Fetch KPIs
        const kpiRes = await fetch('/api/status');
        if (kpiRes.ok) {
            const kpis = await kpiRes.json();
            kpiOnline.textContent = kpis.online_servers;
            kpiOffline.textContent = kpis.offline_servers;
            kpiPending.textContent = kpis.pending_servers;
            kpiSpecs.textContent = `${kpis.total_cores}c / ${kpis.total_threads}t`;
            kpiResources.textContent = `${(kpis.total_ram_mb / 1024).toFixed(1)} GB`;
        }

        // Fetch Servers
        const serversRes = await fetch('/api/servers');
        if (serversRes.ok) {
            servers = await serversRes.json();
        }

        // Fetch Clusters
        const clustersRes = await fetch('/api/clusters');
        if (clustersRes.ok) {
            clusters = await clustersRes.json();
            updateClusterSelects();
        }

        // Fetch Logs if on logs tab
        if (currentTab === 'audit-section') {
            const logsRes = await fetch('/api/logs');
            if (logsRes.ok) {
                const logs = await logsRes.json();
                renderLogs(logs);
            }
        }

        renderDashboard();
    } catch (e) {
        console.error("Error fetching fleet data", e);
    }
}

function updateClusterSelects() {
    // Save selection
    const val = clusterFilter.value;
    
    // Clear select, keep defaults
    clusterFilter.innerHTML = `
        <option value="all">ALL FLEET SECTORS</option>
        <option value="unassigned">UNASSIGNED SECTOR</option>
    `;

    clusters.forEach(c => {
        const opt = document.createElement('option');
        opt.value = c.id;
        opt.textContent = `${c.name.toUpperCase()} (${c.server_count})`;
        clusterFilter.appendChild(opt);
    });

    clusterFilter.value = val;
}

// Render Operations
function renderDashboard() {
    // 1. Filter servers
    const filteredServers = servers.filter(s => {
        // Filter by Sector
        if (selectedClusterFilter === 'unassigned') {
            if (s.cluster_id) return false;
        } else if (selectedClusterFilter !== 'all') {
            if (s.cluster_id !== selectedClusterFilter) return false;
        }

        // Ignore pending ones from main table
        if (s.status === 'pending_approval') return false;

        // Search text
        if (searchKeyword) {
            const name = (s.custom_name || '').toLowerCase();
            const host = s.hostname.toLowerCase();
            const os = s.os_version.toLowerCase();
            const cpu = s.cpu_model.toLowerCase();
            if (!name.includes(searchKeyword) && !host.includes(searchKeyword) && !os.includes(searchKeyword) && !cpu.includes(searchKeyword)) {
                return false;
            }
        }

        return true;
    });

    // 2. Render Core Map
    renderCoreMap(filteredServers);

    // 3. Render Pending Panel
    const pendingDevices = servers.filter(s => s.status === 'pending_approval');
    renderPendingPanel(pendingDevices);

    // 4. Render Active Servers Table
    renderServersTable(filteredServers);

    // 5. Render Clusters Config Table
    if (currentTab === 'clusters-section') {
        renderClustersTable();
    }
}

function renderCoreMap(filtered) {
    coreMapGrid.innerHTML = '';
    
    filtered.forEach(s => {
        const totalThreads = s.cpu_threads || 1;
        const load = s.recent_metrics ? s.recent_metrics.cpu_load_pct : 0;
        
        let colorClass = 'bg-gray'; // Offline
        if (s.is_online) {
            if (load < 50) colorClass = 'bg-teal';
            else if (load < 80) colorClass = 'bg-amber';
            else colorClass = 'bg-red';
        }

        const name = s.custom_name || s.hostname;

        for (let i = 0; i < totalThreads; i++) {
            const cell = document.createElement('div');
            cell.className = `core-cell ${colorClass}`;
            cell.dataset.tooltip = `${name} | Core #${i + 1} | Load: ${s.is_online ? load.toFixed(1) + '%' : 'OFFLINE'}`;
            cell.addEventListener('click', () => openServerDetail(s.id));
            coreMapGrid.appendChild(cell);
        }
    });

    if (coreMapGrid.children.length === 0) {
        coreMapGrid.innerHTML = `<div style="grid-column: 1/-1; text-align: center; color: var(--text-secondary); font-family: var(--font-display); font-size: 0.8rem; padding: 2rem;">NO CPU THREADS REPORTED IN THIS SECTOR</div>`;
    }
}

function renderPendingPanel(devices) {
    if (devices.length === 0) {
        pendingPanel.classList.add('hidden');
        return;
    }

    pendingPanel.classList.remove('hidden');
    pendingList.innerHTML = '';

    devices.forEach(d => {
        const row = document.createElement('div');
        row.className = 'pending-row';
        
        row.innerHTML = `
            <div class="pending-details">
                <h4>${d.hostname}</h4>
                <p>OS: ${d.os_version} | CPU: ${d.cpu_model} (${d.cpu_cores}c/${d.cpu_threads}t) | RAM: ${(d.ram_total_mb/1024).toFixed(1)}GB</p>
                <p style="margin-top: 0.25rem;">UUID: ${d.id}</p>
            </div>
            <div class="pending-actions">
                <button class="btn-action btn-approve" onclick="approveAgent('${d.id}')">APPROVE ACCESS</button>
                <button class="btn-action btn-reject" onclick="rejectAgent('${d.id}')">REJECT</button>
            </div>
        `;
        pendingList.appendChild(row);
    });
}

async function approveAgent(id) {
    if (confirm("Approve connection and issue cryptographic token for this device?")) {
        const res = await fetch(`/api/servers/${id}/approve`, { method: 'POST' });
        if (res.ok) fetchFleetData();
    }
}

async function rejectAgent(id) {
    if (confirm("Reject device access? The agent will self-uninstall.")) {
        const res = await fetch(`/api/servers/${id}/reject`, { method: 'POST' });
        if (res.ok) fetchFleetData();
    }
}

function renderServersTable(filtered) {
    serverTableBody.innerHTML = '';

    if (filtered.length === 0) {
        serverTableBody.innerHTML = `<tr><td colspan="8" style="text-align: center; color: var(--text-secondary); padding: 2rem;">NO ACTIVE DEVICES REGISTERED IN THIS SECTOR</td></tr>`;
        return;
    }

    filtered.forEach(s => {
        const row = document.createElement('tr');
        
        // Status Dot
        const statusDot = s.is_online 
            ? `<span class="status-dot bg-teal" title="Online"></span>` 
            : `<span class="status-dot bg-red" title="Offline"></span>`;

        // Uptime Display
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

        // Metrics formatting
        let cpuFill = 0;
        let ramFill = 0;
        let ramText = '—';
        let cpuText = '—';

        if (s.is_online && s.recent_metrics) {
            cpuFill = s.recent_metrics.cpu_load_pct;
            cpuText = `${cpuFill.toFixed(0)}%`;

            const usedRAM = s.recent_metrics.ram_used_mb;
            const totalRAM = s.ram_total_mb;
            ramFill = totalRAM > 0 ? (usedRAM / totalRAM) * 100 : 0;
            ramText = `${(usedRAM / 1024).toFixed(1)} / ${(totalRAM / 1024).toFixed(1)} GB`;
        }

        // Progress bar colors
        const cpuBarColor = cpuFill > 80 ? 'bg-red' : (cpuFill > 50 ? 'bg-amber' : 'bg-teal');
        const ramBarColor = ramFill > 80 ? 'bg-red' : (ramFill > 50 ? 'bg-amber' : 'bg-teal');

        // Sector Select options
        let clusterOpts = `<option value="">Unassigned</option>`;
        clusters.forEach(c => {
            const sel = s.cluster_id === c.id ? 'selected' : '';
            clusterOpts += `<option value="${c.id}" ${sel}>${c.name}</option>`;
        });

        const nameDisplay = s.custom_name 
            ? `<div class="custom-name text-teal">${s.custom_name}</div><div class="hostname">${s.hostname}</div>`
            : `<div class="custom-name">${s.hostname}</div><div class="hostname" style="color: var(--text-secondary);">No label</div>`;

        row.innerHTML = `
            <td>${statusDot}</td>
            <td class="server-name-cell">${nameDisplay}</td>
            <td>
                <select class="btn-action" onchange="setServerCluster('${s.id}', this.value)" style="background: transparent;">
                    ${clusterOpts}
                </select>
            </td>
            <td>
                <div class="spec-badge" title="${s.os_version} | ${s.cpu_model} (${s.cpu_cores}c/${s.cpu_threads}t) | RAM: ${(s.ram_total_mb/1024).toFixed(1)}GB | C: ${(s.disk_total_mb/1024).toFixed(1)}GB">
                    ${s.os_version} | ${s.cpu_model} | RAM: ${(s.ram_total_mb/1024).toFixed(1)}GB
                </div>
            </td>
            <td>
                <div class="progress-bar-container">
                    <div class="progress-bar-fill ${cpuBarColor}" style="width: ${cpuFill}%"></div>
                </div>
                <span class="table-progress">${cpuText}</span>
            </td>
            <td>
                <div class="progress-bar-container">
                    <div class="progress-bar-fill ${ramBarColor}" style="width: ${ramFill}%"></div>
                </div>
                <span class="table-progress">${ramText}</span>
            </td>
            <td style="font-family: var(--font-code); font-size: 0.75rem;">${uptimeText}</td>
            <td>
                <div class="row-actions">
                    <button class="btn-action" onclick="openServerDetail('${s.id}')">INFO</button>
                    <button class="btn-action" onclick="renameServerPrompt('${s.id}', '${s.custom_name || ''}')">RENAME</button>
                    <button class="btn-action" onclick="openTerminal('${s.id}', '${s.custom_name || s.hostname}')" ${!s.is_online ? 'disabled' : ''}>SHELL</button>
                    <button class="btn-action" onclick="openFileManager('${s.id}', '${s.custom_name || s.hostname}')" ${!s.is_online ? 'disabled' : ''}>FILE</button>
                    <button class="btn-action btn-reject" onclick="decommissionServer('${s.id}')">REMOVE</button>
                </div>
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
    const newName = prompt("Enter custom name for this server (leave empty to use hostname):", currentName);
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
    if (confirm("🚨 WARNING: Are you sure you want to decommission this server?\nThe agent connection will be terminated and instructed to self-uninstall.")) {
        const res = await fetch(`/api/servers/${id}/remove`, { method: 'POST' });
        if (res.ok) fetchFleetData();
    }
}

// Cluster management
function renderClustersTable() {
    clustersTableBody.innerHTML = '';
    
    if (clusters.length === 0) {
        clustersTableBody.innerHTML = `<tr><td colspan="4" style="text-align: center; color: var(--text-secondary); padding: 2rem;">NO SECTORS CREATED YET</td></tr>`;
        return;
    }

    clusters.forEach(c => {
        const row = document.createElement('tr');
        row.innerHTML = `
            <td style="font-family: var(--font-display); font-weight: 600; font-size: 0.95rem;">${c.name}</td>
            <td>${c.description || '—'}</td>
            <td style="font-family: var(--font-code);">${c.server_count}</td>
            <td>
                <button class="btn-action btn-reject" onclick="deleteCluster('${c.id}')">DELETE</button>
            </td>
        `;
        clustersTableBody.appendChild(row);
    });
}

async function deleteCluster(id) {
    if (confirm("Are you sure you want to delete this sector? Servers inside will be marked as unassigned.")) {
        const res = await fetch(`/api/clusters/${id}`, { method: 'DELETE' });
        if (res.ok) fetchFleetData();
    }
}

// Logs Rendering
function renderLogs(logs) {
    auditTableBody.innerHTML = '';
    
    if (logs.length === 0) {
        auditTableBody.innerHTML = `<tr><td colspan="6" style="text-align: center; color: var(--text-secondary); padding: 2rem;">NO COMMAND LOGS FOUND</td></tr>`;
        return;
    }

    logs.forEach(l => {
        const row = document.createElement('tr');
        const completed = l.completed_at ? new Date(l.completed_at).toLocaleString() : 'Running...';
        
        let resultSnippet = '—';
        if (l.result) {
            resultSnippet = l.result.length > 50 ? l.result.slice(0, 50) + '...' : l.result;
        }

        row.innerHTML = `
            <td style="font-family: var(--font-code); font-size: 0.7rem;">${l.id}</td>
            <td style="font-family: var(--font-display); font-weight: 600;">${l.server_name}</td>
            <td><span class="badge-live" style="background-color: transparent; border-color: var(--gray-color); color: var(--text-secondary); box-shadow: none;">${l.issued_by}</span></td>
            <td style="font-family: var(--font-code); font-size: 0.75rem; color: var(--teal-color);">${l.command}</td>
            <td style="font-family: var(--font-code); font-size: 0.7rem; white-space: pre-wrap; max-width: 250px; overflow: hidden; text-overflow: ellipsis;">${resultSnippet}</td>
            <td style="font-family: var(--font-code); font-size: 0.75rem;">${new Date(l.issued_at).toLocaleString()}</td>
        `;
        auditTableBody.appendChild(row);
    });
}

// Details Drawer
async function openServerDetail(id) {
    try {
        const res = await fetch(`/api/servers/${id}`);
        if (res.ok) {
            const data = await res.json();
            const s = data.server;
            const metrics = data.metrics || [];
            
            drawerServerName.textContent = (s.custom_name || s.hostname).toUpperCase();
            
            // Format first & last seen
            const first = new Date(s.first_seen_at).toLocaleString();
            const last = new Date(s.last_seen_at).toLocaleString();

            let metricsSection = '<div style="color: var(--text-secondary); text-align: center; padding: 2rem;">OFFLINE — NO REAL-TIME METRICS</div>';
            if (data.is_online && metrics.length > 0) {
                // CPU load list
                let sparklinesCPU = '';
                let sparklinesRAM = '';
                metrics.slice().reverse().forEach(m => {
                    const cpuH = m.cpu_load_pct;
                    const ramUsedPct = s.ram_total_mb > 0 ? (m.ram_used_mb / s.ram_total_mb) * 100 : 0;
                    
                    sparklinesCPU += `<div class="sparkline-bar" style="height: ${cpuH}%;" data-tooltip="CPU: ${cpuH.toFixed(0)}% at ${new Date(m.timestamp).toLocaleTimeString()}"></div>`;
                    sparklinesRAM += `<div class="sparkline-bar" style="height: ${ramUsedPct}%; background-color: var(--amber-color);" data-tooltip="RAM: ${(m.ram_used_mb/1024).toFixed(1)} GB (${ramUsedPct.toFixed(0)}%)"></div>`;
                });

                metricsSection = `
                    <div class="chart-container">
                        <div class="chart-title">CPU LOAD (HISTORICAL SAMPLE)</div>
                        <div class="sparkline-wrapper">${sparklinesCPU}</div>
                    </div>
                    <div class="chart-container mt-4">
                        <div class="chart-title">RAM USAGE (HISTORICAL SAMPLE)</div>
                        <div class="sparkline-wrapper">${sparklinesRAM}</div>
                    </div>
                `;
            }

            drawerBodyContent.innerHTML = `
                <div class="detail-grid">
                    <div class="spec-list">
                        <div class="spec-item">
                            <span class="spec-label">SYSTEM HOSTNAME</span>
                            <span class="spec-val">${s.hostname}</span>
                        </div>
                        <div class="spec-item">
                            <span class="spec-label">CUSTOM IDENTIFIER</span>
                            <span class="spec-val text-teal">${s.custom_name || '—'}</span>
                        </div>
                        <div class="spec-item">
                            <span class="spec-label">OPERATING SYSTEM</span>
                            <span class="spec-val">${s.os_version}</span>
                        </div>
                        <div class="spec-item">
                            <span class="spec-label">CPU ARCHITECTURE</span>
                            <span class="spec-val">${s.cpu_model}</span>
                        </div>
                        <div class="spec-item">
                            <span class="spec-label">CORES / THREADS</span>
                            <span class="spec-val">${s.cpu_cores} Cores / ${s.cpu_threads} Threads</span>
                        </div>
                        <div class="spec-item">
                            <span class="spec-label">TOTAL PHYSICAL MEMORY</span>
                            <span class="spec-val">${(s.ram_total_mb/1024).toFixed(1)} GB</span>
                        </div>
                        <div class="spec-item">
                            <span class="spec-label">SYSTEM DRIVE C: SIZE</span>
                            <span class="spec-val">${(s.disk_total_mb/1024).toFixed(1)} GB</span>
                        </div>
                        <div class="spec-item">
                            <span class="spec-label">RMM AGENT VERSION</span>
                            <span class="spec-val">${s.agent_version}</span>
                        </div>
                        <div class="spec-item">
                            <span class="spec-label">FIRST DETECTED TIME</span>
                            <span class="spec-val">${first}</span>
                        </div>
                        <div class="spec-item">
                            <span class="spec-label">LAST ACTIVE HEARTBEAT</span>
                            <span class="spec-val">${last}</span>
                        </div>
                    </div>
                    
                    ${metricsSection}
                </div>
            `;
            serverDrawer.classList.add('open');
        }
    } catch (e) {
        console.error(e);
    }
}

// Terminal Shell Modal
function openTerminal(serverId, name) {
    activeTerminalServerId = serverId;
    terminalTitle.textContent = `SHELL CONSOLE — ${name.toUpperCase()}`;
    terminalOutput.innerHTML = `<div class="msg secondary">[CONNECTION ESTABLISHED WITH POWERSHELL SERVICE IN AGENT]</div>`;
    terminalModal.classList.add('open');
    terminalInput.focus();
}

function appendTerminalLine(text, type) {
    const div = document.createElement('div');
    div.className = `term-line ${type}`;
    div.textContent = text;
    terminalOutput.appendChild(div);
    terminalOutput.scrollTop = terminalOutput.scrollHeight;
}

// File Manager Modal
function openFileManager(serverId, name) {
    activeFileServerId = serverId;
    fileTitle.textContent = `FILE SYSTEM MODULE — ${name.toUpperCase()}`;
    fileProgress.textContent = '';
    dlPathInput.value = '';
    ulPathInput.value = '';
    ulFileInput.value = '';
    fileModal.classList.add('open');
}

// AI Message Helper
function appendAIMessage(text, sender) {
    const div = document.createElement('div');
    div.className = `msg ${sender}`;
    div.textContent = text;
    aiMessages.appendChild(div);
    aiMessages.scrollTop = aiMessages.scrollHeight;
    return div;
}
