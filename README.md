# Crybaby

Distributed Windows 11 PC cluster orchestrator and client Hyper-V remote virtualization manager.

[![Go Version](https://img.shields.io/badge/go-1.23+-00ADD8.svg)](https://golang.org)
[![Platform](https://img.shields.io/badge/platform-Windows%2011%20%7C%20Linux%20Mint-blue.svg)]()
[![Virtualization](https://img.shields.io/badge/hypervisor-Client%20Hyper--V-0078D4.svg)]()
[![Overlay Network](https://img.shields.io/badge/mesh-WireGuard%20(Curve25519)-88171A.svg)]()
[![Release](https://img.shields.io/badge/release-v0.5.0-success.svg)]()
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

Crybaby pools standard consumer Windows 11 PCs (Home and Pro) into a unified compute and virtualization cluster controlled from a single laptop web dashboard over a local LAN switch. It eliminates the need for expensive Windows Server licensing, dedicated server hardware, or destructive bare-metal hypervisor installations like Proxmox VE and VMware ESXi. 

Nodes connect through an encrypted WireGuard point-to-point overlay network without port forwarding, provision in seconds with a 1-click USB flashdisk pack, bridge virtual machines directly to physical LAN interfaces, and stay resilient against sleep, reboots, and power interruptions.

---

## 💡 The Problem and the Solution

Modern homelabs, software agencies, and development teams often have idle consumer desktop PCs running Windows 11 Home or Pro. Running these machines as headless server nodes traditionally presents severe operational friction:

1. **Destructive Bare-Metal Installs:** Tools like Proxmox or ESXi require wiping existing desktop operating systems, destroying local files and hardware utility.
2. **Hidden Hypervisor on Windows 11 Home:** Windows 11 Home hides the Hyper-V management GUI, while Modern Standby sleep policies drop network interfaces after minutes of idle time.
3. **Hardware Fragility:** Consumer PC power buttons are easily pressed by accident, Windows Update enforces midnight auto-restarts, and Hyper-V VMs remain stopped after a host reboot.
4. **Networking Complexity:** Managing nodes across local LAN switches normally demands manual static IP assignments, port forwarding, or third-party cloud relays.
5. **Generation 2 UEFI Gotchas:** Hyper-V Generation 2 VMs lack default virtual DVD drives and default to Microsoft Windows Secure Boot, causing Linux installer ISOs (Ubuntu, Debian) to fail on boot.

### Crybaby Solves Every Layer:

- **Lightweight Windows Service:** Runs as a compiled Go binary (`crybaby-agent.exe`) consuming under 20 MB of RAM with zero external runtime dependencies.
- **Hyper-V Unlocked via DISM:** Injects underlying Hyper-V packages (`Microsoft-Hyper-V-*.mum`) and controls VMs via PowerShell WMI cmdlets.
- **Power & Auto-Recovery Hardening:** Remaps physical casing buttons to *Do Nothing* (`PBUTTONACTION 0`), disables Modern Standby sleep, blocks midnight Windows Update auto-reboots, and sets VMs to auto-resume on boot (`StartIfRunning`).
- **Self-Healing External Virtual Switch:** Automatically detects the active physical Ethernet NIC and creates a bridged virtual switch (`Crybaby-LAN`) so guest VMs receive router DHCP IPs.
- **Flawless Gen 2 Linux & Windows ISO Boot:** Uses SCSI `Add-VMDvdDrive` and auto-disables Secure Boot (`-EnableSecureBoot $false`) for seamless UEFI boot on any OS.
- **Proxmox-Grade Checkpoints / Snapshots:** Instant point-in-time VM snapshots, rollback restoration, and deletion directly from the dashboard modal.
- **Live VM Telemetry & 1-Click RDP:** Real-time CPU% utilization and dynamic Memory Demand indicators, plus an instant `.rdp` profile launcher button.
- **1-Click Clean Lifecycle:** Self-contained USB flashdisk installation, complete uninstaller (`scripts/uninstall.bat`), and a single-button dashboard cluster reset.

---

## 🏛️ System Architecture

Crybaby separates cluster control from node execution through a two-tier Go monorepo:

```
+-----------------------------------------------------------------------------------+
|                        Laptop Controller Hub (Linux Mint)                         |
|                                                                                   |
|  +---------------------------+        +----------------------------------------+  |
|  | Embedded Web Dashboard    |        | REST & WebSocket API Dispatcher        |  |
|  | - Tailwind CSS UI         |        | - Node Registry & Heartbeat Reaper     |  |
|  | - xterm.js PTY Shell      |        | - VM Checkpoint & Lifecycle Proxy      |  |
|  | - Live Metrics & 1-Click  |        | - Static HTTP ISO Repo (/iso/)         |  |
|  |   RDP Launcher Profile    |        | - Dynamic USB Pack Generator (.zip)    |  |
|  +-------------+-------------+        +-------------------+--------------------+  |
|                |                                          |                       |
|                +--------------------+---------------------+                       |
|                                     |                                             |
|                                     v                                             |
|                     Cluster Registry & State Engine                               |
|                     WireGuard Controller Hub (10.42.0.1)                          |
+-------------------------------------+---------------------------------------------+
                                      |
              Private WireGuard Mesh Layer (10.42.0.0/16)
              + Physical Gigabit LAN Switch Interconnect
                                      |
              +-----------------------+-----------------------+
              |                                               |
              v                                               v
+---------------------------+                   +---------------------------+
|    Node 1 (Win 11 Pro)    |                   |    Node 2 (Win 11 Home)   |
| +-----------------------+ |                   | +-----------------------+ |
| | Crybaby Windows Svc   | |                   | | Crybaby Windows Svc   | |
| | (crybaby-agent.exe)   | |                   | | (crybaby-agent.exe)   | |
| +-----------+-----------+ |                   | +-----------+-----------+ |
|             |             |                   |             |             |
|   +---------+---------+   |                   |   +---------+---------+   |
|   v                   v   |                   |   v                   v   |
| Hyper-V WMI        Local  |                   | Hyper-V WMI        Local  |
| Engine (PowerShell) PTY   |                   | Engine (PowerShell) PTY   |
|   |                   |   |                   |   |                   |   |
|   v                   v   |                   |   v                   v   |
| +-----------------------+ |                   | +-----------------------+ |
| | VMs (Bridged to LAN)  | |                   | | VMs (Bridged to LAN)  | |
| | Checkpoints & Metrics | |                   | | Checkpoints & Metrics | |
| +-----------------------+ |                   | +-----------------------+ |
+---------------------------+                   +---------------------------+
```

### Module Responsibilities

- `cmd/controller`: Single binary HTTP server, WebSocket telemetry hub, VM reverse-proxy, HTTP ISO repository, and dynamic USB zip pack generator.
- `cmd/agent`: High-performance Windows Service (`crybaby-agent.exe`) that gathers telemetry, manages Hyper-V virtual machines, handles snapshots, and hosts the interactive PTY shell endpoint.
- `pkg/hyperv`: Dual-engine virtualization driver. Uses native PowerShell WMI cmdlets on Windows hosts and an in-memory thread-safe simulated hypervisor for Linux cross-development.
- `pkg/mesh`: WireGuard key generator and peer configuration manager implementing Curve25519 X25519 cryptography.
- `pkg/terminal`: Bidirectional pseudo-terminal (PTY) session multiplexer streaming shell process I/O across WebSocket frames.
- `pkg/cluster`: Thread-safe in-memory node registry with periodic offline node reaper and write-through JSON persistence (`cluster-state.json`).
- `scripts/systemd`: Linux Mint systemd unit file and installer script for automatic background daemon operation on boot.

---

## 🎨 Web Dashboard & Design System

The controller delivers an agency-grade embedded browser interface served directly from memory without Node.js runtime dependencies or external asset CDNs.

### Design Tokens

- **Background:** Obsidian Slate (`#0B0F19`)
- **Card Surfaces:** Deep Charcoal (`#111827`) with subtle border framing (`#1F2937`)
- **Primary Accent:** Indigo (`#6366F1`) & Violet (`#8B5CF6`)
- **Semantic Statuses:**
  - Emerald (`#10B981`): Online nodes, active running VMs, healthy cluster state.
  - Amber (`#F59E0B`): Stopped VMs, warning states, offline heartbeat indicators.
  - Rose (`#EF4444`): Destructive actions (VM deletion, host reboot, cluster reset).
  - Sky (`#0EA5E9`): Network badges, IP indicators, and 1-click RDP launchers.
- **Typography:** System Sans-Serif paired with monospaced accents (`Menlo`, `Monaco`, `Courier New`) for IP addresses, specifications, and telemetry values.
- **Terminal Canvas:** `xterm.js` v5.3.0 with `xterm-addon-fit`, featuring ANSI color rendering, dark slate terminal background, and non-blocking WebSocket I/O.

### Interface Capabilities

- **Live Fleet Table:** Real-time health badges, physical and WireGuard IP addresses, hardware specs (CPU model, RAM, Disk), and dynamic allocation meters.
- **Virtual Machine Modal:** Generation 2 VM allocation with vCPU count, RAM (MB), disk size (GB), virtual switch assignment, and boot ISO selection.
- **Per-VM Metrics Badges:** Real-time CPU% utilization badge and active dynamic RAM demand (`MemoryDemandMB`) displayed for each VM.
- **1-Click RDP Launcher:** Blue "RDP" button next to guest VM IP address that instantly downloads a standard `.rdp` connection profile.
- **Checkpoint / Snapshot Modal:** Dedicated snapshot manager to create instant restore points, view chronological snapshot trees with timestamps, rollback state, and delete snapshots.
- **Remote Host Controls:** Power management dropdown for each node (Graceful Host Reboot, Host Shutdown, and Wake-on-LAN).
- **Interactive PTY Shell:** In-browser PowerShell terminal with full ANSI colors, keyboard shortcuts, and dynamic window resize.
- **Cluster Reset / Fresh Purge:** Red "Reset / Fresh" button in header to clear dead nodes and reset cluster persistence in one click.

---

## 🚀 Quickstart Guide

### 1. Build Binaries

Prerequisites: Go 1.23 or newer.

```bash
# Clone the repository
git clone https://github.com/davinmaritza/crybaby.git
cd crybaby

# Build the Linux controller
go build -o bin/crybaby-controller ./cmd/controller

# Cross-compile the Windows agent binary
GOOS=windows GOARCH=amd64 go build -o bin/crybaby-agent.exe ./cmd/agent
```

### 2. Start the Controller

Run directly from the terminal:

```bash
./bin/crybaby-controller -port 8080 -token my-cluster-secret -password crybaby123
```

Or install as a **Linux Mint systemd background service** (survives reboots):

```bash
sudo ./scripts/systemd/install-service.sh
```

Access the dashboard at `http://localhost:8080` (or `http://<LAPTOP_LAN_IP>:8080`).

### 3. Generate the 1-Click USB Installer

1. On the dashboard header, click **"Generate Node USB"**.
2. The controller automatically detects its LAN IP (e.g. `192.168.100.168`).
3. Click **"Unduh Paket USB (.zip)"**.
4. The controller packages:
   - `crybaby-agent.exe`: Compiled Windows service binary (~8.8 MB).
   - `install.bat`: 1-click automated administrator installer script.
   - `uninstall.bat`: 1-click clean uninstaller script.
   - `enable-hyperv.ps1`: DISM Hyper-V enabler for Windows 11 Home & Pro.
   - `crybaby-config.json`: Pre-populated configuration with controller LAN URL and join token.
5. Extract the ZIP contents to the root of a USB flashdisk.

### 4. Deploy to Windows 11 Node

1. Plug the flashdisk into any target Windows 11 computer connected to the same LAN switch.
2. Right-click `install.bat` and select **"Run as Administrator"**.
3. The script automatically executes:
   - Enables Hyper-V packages via DISM if absent.
   - Creates bridged External Virtual Switch (`Crybaby-LAN`) to physical Ethernet.
   - Opens Windows Defender Firewall rules (TCP 8081, UDP 51820, ICMPv4 ping).
   - Sets network profile to `Private`.
   - Prevents Modern Standby AC sleep (`powercfg`).
   - Remaps physical power button to *Do Nothing* (`PBUTTONACTION 0`).
   - Injects Windows Update anti-auto-reboot policies.
   - Registers and starts `CrybabyAgent` as an automatic Windows Service.
4. Unplug the flashdisk. The node appears on your controller dashboard within 3 seconds.

### 5. Serving ISOs via HTTP Repository

Place your OS installer ISO files into the `./iso` directory on the controller host:

```bash
cp ubuntu-24.04-live-server-amd64.iso ./iso/
```

Nodes and VMs can access these files over the LAN switch via:
`http://<CONTROLLER_LAN_IP>:8080/iso/ubuntu-24.04-live-server-amd64.iso`

---

## 📡 API Reference

### Controller HTTP Endpoints

| Method | Endpoint | Description |
| :--- | :--- | :--- |
| `GET` | `/` | Serves the embedded single-page management dashboard. |
| `GET` | `/iso/*` | Static HTTP ISO repository file server. |
| `POST` | `/api/v1/auth/login` | Authenticates administrator session and issues token. |
| `POST` | `/api/v1/cluster/reset` | Purges all nodes from cluster registry and clears state. |
| `POST` | `/api/v1/nodes/register` | Registers a new agent node and returns mesh configuration. |
| `GET` | `/api/v1/nodes` | Returns all registered nodes with their latest telemetry. |
| `GET` | `/api/v1/nodes/:id` | Returns status and hardware details for a single node. |
| `DELETE` | `/api/v1/nodes/:id` | Removes an offline/decommissioned node from cluster. |
| `POST` | `/api/v1/nodes/:id/wol` | Dispatches Wake-on-LAN magic packet to node MAC address. |
| `POST` | `/api/v1/nodes/:id/reboot` | Triggers graceful remote reboot on physical Windows host. |
| `POST` | `/api/v1/nodes/:id/shutdown` | Triggers graceful remote shutdown on physical Windows host. |
| `GET` | `/api/v1/nodes/:id/vms` | Lists all Hyper-V virtual machines with metrics & IP. |
| `POST` | `/api/v1/nodes/:id/vms` | Creates a new Generation 2 Hyper-V VM with optional ISO. |
| `POST` | `/api/v1/nodes/:id/vms/:vm_id/:action` | Executes power action (`start`, `stop`, `restart`) on VM. |
| `DELETE` | `/api/v1/nodes/:id/vms/:vm_id` | Deletes the VM and purges its attached VHDX virtual disk. |
| `GET` | `/api/v1/nodes/:id/vms/:vm_id/checkpoints` | Lists all Hyper-V checkpoints/snapshots for a VM. |
| `POST` | `/api/v1/nodes/:id/vms/:vm_id/checkpoints` | Creates a new point-in-time checkpoint for a VM. |
| `POST` | `/api/v1/nodes/:id/vms/:vm_id/checkpoints/restore` | Restores a VM to a selected checkpoint snapshot. |
| `DELETE` | `/api/v1/nodes/:id/vms/:vm_id/checkpoints/:name` | Deletes a checkpoint snapshot from a VM. |
| `GET` | `/api/v1/nodes/:id/switches` | Returns available Hyper-V virtual network switches. |
| `GET` | `/api/v1/provision/usb-zip` | Generates and downloads dynamic 1-click USB deployment pack. |
| `GET` | `/api/v1/provision/usb-profile` | Returns JSON provisioning profile with auto-detected LAN IP. |

### WebSocket Endpoints

- `GET /api/v1/ws/telemetry`: Real-time JSON stream of cluster health, CPU/RAM utilization, and node telemetry.
- `GET /api/v1/ws/terminal/:node_id`: Bidirectional PTY stream connecting the browser xterm.js instance to the node PowerShell session.

---

## 🧪 Testing & Verification

Run the complete automated test suite:

```bash
go test -v ./...
```

Test coverage includes:
- `pkg/auth`: Password hashing with salt, constant-time verification, and token authentication.
- `pkg/cluster`: Node registration, MAC deduplication, summary aggregation, and offline node reaper.
- `pkg/hyperv`: VM creation, name collision handling, power state transitions, virtual switch auto-creation, and full checkpoint/snapshot lifecycle (Create, List, Restore, Delete).
- `pkg/mesh`: Curve25519 key generation and WireGuard client peer configuration.
- `pkg/terminal`: Shell process spawning, WebSocket pipe streaming, input forwarding, and clean session termination.

---

## 🧹 Complete Uninstallation

Crybaby respects host machines and leaves no orphaned services or configuration debris. To cleanly uninstall a node:

1. Right-click `scripts/uninstall.bat` on the Windows 11 machine and select **"Run as Administrator"**.
2. The uninstaller:
   - Stops and deletes the `CrybabyAgent` Windows Service.
   - Deletes all inbound firewall rules (Crybaby Agent, WireGuard, Ping).
   - Restores the physical casing power button back to default Shutdown (`PBUTTONACTION 1`).
   - Cleans Windows Update registry override policies.
   - Deletes `C:\Program Files\Crybaby`.
   - Optionally asks if existing Hyper-V VM virtual disks should be preserved or purged.

---

## 📚 Project Knowledge System

Crybaby enforces a **Dual Single Source of Truth** architecture:
- **Implementation Source of Truth:** This Git repository holds all executable source code, build scripts, tests, systemd units, and configurations.
- **Knowledge Source of Truth:** The Notion workspace maintains Architecture Decision Records (ADRs), product context, issue trackers, and operational memory.

- **Notion Root Page:** `Crybaby` (`40a13039-c767-8090-a003-c96f7c8c6a28`)
- **Persistent Local Memory:** `/home/suzirz/Documents/Project/chat/LinuxMint/chat.md`

---

## 📄 License

MIT License. Copyright (c) 2026 Davin Maritza.
