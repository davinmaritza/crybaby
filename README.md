# Crybaby

Distributed Windows 11 cluster orchestrator and client Hyper-V virtualization manager.

Crybaby pools standard consumer Windows 11 PCs into a unified compute cluster controlled from a single laptop web dashboard. It eliminates the need for Windows Server licenses, dedicated server hardware, or complex hypervisor installations like Proxmox and ESXi. Nodes connect through an encrypted WireGuard point-to-point overlay network without port forwarding, provision with a single USB flashdisk, and stay active using automated Windows power profiles.

---

## The Problem and the Solution

Most homelabs and development teams have powerful consumer desktop PCs running Windows 11 Home or Pro. These machines sit idle for hours because running them as headless server nodes presents three friction points:

1. Traditional hypervisor clusters require dedicated OS installations that wipe existing desktop setups.
2. Windows 11 Home hides the Hyper-V management interface and aggressive Modern Standby settings drop network connections after minutes of idle time.
3. Managing nodes across home routers normally demands manual port forwarding, dynamic DNS, or third-party cloud relays.

Crybaby resolves these issues:
- Runs as a lightweight Go Windows Service consuming under 20 MB of RAM.
- Unlocks the underlying Hyper-V hypervisor on both Windows 11 Home and Pro using DISM packages and WMI v2 cmdlets.
- Automates power settings through `powercfg` to keep background services awake while monitors stay off.
- Provisions nodes with a one-time USB flashdisk script that allows the drive to be removed immediately after installation.
- Connects nodes over a private Curve25519 WireGuard overlay mesh (`10.42.0.0/16`) for direct point-to-point traffic without open firewall ports.

---

## System Architecture

Crybaby separates cluster control from node execution through a two-tier Go monorepo:

```
+-------------------------------------------------------------+
|                  Laptop Controller Hub                      |
|                                                             |
|  +--------------------+        +-------------------------+  |
|  | Web Dashboard UI   |        | REST and WS Dispatcher  |  |
|  | (Tailwind + xterm) |        | (Telemetry and Proxy)   |  |
|  +---------+----------+        +------------+------------+  |
|            |                                |               |
|            +---------------+----------------+               |
|                            |                                |
|                            v                                |
|             Cluster Registry & State Engine                 |
|             WireGuard Controller (10.42.0.1)                |
+----------------------------+--------------------------------+
                             |
         Private WireGuard Mesh Layer (10.42.0.0/16)
                             |
             +---------------+---------------+
             |                               |
             v                               v
+-------------------------+     +-------------------------+
|   Node A (Win 11 Pro)   |     |   Node B (Win 11 Home)  |
| +---------------------+ |     | +---------------------+ |
| | Crybaby Windows Svc | |     | | Crybaby Windows Svc | |
| | (Agent + Local RPC) | |     | | (Agent + Local RPC) | |
| +----------+----------+ |     | +----------+----------+ |
|            v            |     |            v            |
|  Hyper-V WMI Subsystem  |     |  Hyper-V WMI Subsystem  |
| +---------------------+ |     | +---------------------+ |
| | Virtual Machines    | |     | | Virtual Machines    | |
| +---------------------+ |     | +---------------------+ |
+-------------------------+     +-------------------------+
```

### Components

- `cmd/controller`: Portable single binary running the HTTP server, WebSocket telemetry hub, VM reverse-proxy, and dynamic USB zip pack generator.
- `cmd/agent`: Windows Service binary (`crybaby-agent.exe`) that gathers hardware telemetry, manages Hyper-V virtual machines, and hosts the interactive PTY shell endpoint.
- `pkg/hyperv`: Dual-engine virtualization driver. Uses native PowerShell WMI cmdlets on Windows hosts and an in-memory simulated hypervisor for Linux cross-development.
- `pkg/mesh`: WireGuard key generator and peer configuration manager implementing Curve25519 X25519 cryptography.
- `pkg/terminal`: Bidirectional pseudo-terminal (PTY) session multiplexer streaming shell process I/O across WebSocket frames.
- `pkg/cluster`: Thread-safe in-memory node registry with periodic offline node reaper and write-through JSON persistence.

---

## Web Dashboard and Design System

The controller delivers an embedded browser interface served directly from memory without Node.js runtime dependencies or external static file hosting.

### Design Tokens

- Background: Obsidian Slate (`#0B0F19`)
- Card Surfaces: Deep Charcoal (`#111827`) with 1px border (`#1F2937`)
- Primary Accent: Indigo (`#6366F1`)
- Status Semantics: Emerald (`#10B981`) for online nodes and active VMs, Amber (`#F59E0B`) for warnings and offline status
- Typography: System sans-serif for UI labels, paired with fixed-width monospace (`Menlo`, `Monaco`, `Courier New`) for IP addresses, hardware specifications, and terminal output
- Terminal Canvas: `xterm.js` v5.3.0 with `xterm-addon-fit`, configured with high-contrast text and a non-blocking WebSocket stream

### Interface Capabilities

- Live Node Fleet Table: Real-time status badges, mesh and physical IP mappings, hardware specs, and CPU and RAM allocation meters.
- Virtual Machine Manager Modal: Forms to allocate generation-2 VMs with customizable vCPU count, memory (MB), VHDX storage size (GB), and virtual switch assignment.
- Power Actions: Instant Start, Stop, and Restart controls with live confirmation feedback.
- Remote PTY Shell: Full interactive PowerShell terminal inside the browser with ANSI color support, keyboard navigation, and automatic window resizing.
- Wake-on-LAN Trigger: One-click UDP magic packet dispatch to power on sleeping machines using their physical MAC addresses.

---

## Quickstart Guide

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

Run the controller on your laptop:

```bash
./bin/crybaby-controller -port 8080 -token my-cluster-secret -password adminpass
```

Open your browser and navigate to `http://localhost:8080`.

### 3. Generate the USB Installer

1. In the top right corner of the dashboard, click "Generate Node USB".
2. Click "Unduh Paket USB (.zip)".
3. The controller dynamically generates `crybaby-usb-pack.zip` containing:
   - `crybaby-agent.exe`: Compiled Windows service binary.
   - `install.bat`: Automated administrator installation script.
   - `enable-hyperv.ps1`: DISM Hyper-V package activator.
   - `crybaby-config.json`: Configuration pre-populated with controller URL and cluster join token.
4. Extract the contents directly to the root of a USB flashdisk.

### 4. Install on a Windows 11 Node

1. Plug the flashdisk into your target Windows 11 computer.
2. Right-click `install.bat` and select "Run as Administrator".
3. The script executes the following steps:
   - Copies files to `C:\Program Files\Crybaby\`.
   - Enables Hyper-V packages via DISM if not already installed.
   - Configures Windows power plan to prevent AC standby and sleep.
   - Registers and starts `CrybabyAgent` as an automatic Windows Service.
4. Unplug the flashdisk. The node appears in the controller dashboard within three seconds.

---

## API Reference

### Controller HTTP Endpoints

| Method | Endpoint | Description |
| :--- | :--- | :--- |
| `GET` | `/` | Serves the embedded single-page management dashboard. |
| `POST` | `/api/v1/nodes/register` | Registers a new agent node and returns mesh configuration. |
| `GET` | `/api/v1/nodes` | Returns all registered nodes with their latest telemetry. |
| `GET` | `/api/v1/nodes/:id` | Returns details for a single node. |
| `POST` | `/api/v1/nodes/:id/wol` | Dispatches a Wake-on-LAN magic packet to the node MAC address. |
| `GET` | `/api/v1/nodes/:id/vms` | Proxies request to node agent to list all Hyper-V virtual machines. |
| `POST` | `/api/v1/nodes/:id/vms` | Proxies request to node agent to create a new Hyper-V VM. |
| `POST` | `/api/v1/nodes/:id/vms/:vm_id/:action` | Executes power action (`start`, `stop`, `restart`) on target VM. |
| `DELETE` | `/api/v1/nodes/:id/vms/:vm_id` | Deletes the virtual machine and deletes its VHDX disk file. |
| `GET` | `/api/v1/nodes/:id/switches` | Returns available Hyper-V virtual network switches on the node. |
| `GET` | `/api/v1/provision/usb-zip` | Downloads complete zip package ready for flashdisk extraction. |
| `GET` | `/api/v1/provision/usb-profile` | Returns JSON provisioning profile for automated deployments. |

### WebSocket Endpoints

- `GET /api/v1/ws/telemetry`: Real-time JSON stream of cluster health, average CPU load, total memory usage, and node counts.
- `GET /api/v1/ws/terminal/:node_id`: Bidirectional PTY stream connecting the browser xterm.js instance to the node PowerShell session.

---

## Testing and Verification

Run the full automated test suite:

```bash
go test -v ./...
```

Test coverage includes:
- `pkg/auth`: Password salt hashing, constant-time verification, and HMAC-SHA256 JWT validation.
- `pkg/cluster`: Node registration, MAC deduplication, summary aggregation, and offline node reaper.
- `pkg/hyperv`: VM creation, duplicate name collision handling, power state transitions, and switch querying.
- `pkg/mesh`: Curve25519 key generation and client WireGuard configuration formatting.
- `pkg/terminal`: Shell process spawning, WebSocket pipe streaming, input forwarding, and process cleanup.

---

## Project Knowledge System

Crybaby enforces a dual single source of truth architecture:
- Implementation Source of Truth: This Git repository holds all executable source code, build scripts, tests, and configurations.
- Knowledge Source of Truth: The Notion workspace maintains architecture decision records (ADRs), product context, issue trackers, and operational memory.

Notion Root Page: `Crybaby` (`40a13039-c767-8090-a003-c96f7c8c6a28`)  
Persistent Local Memory: `/home/suzirz/Documents/Project/chat/LinuxMint/chat.md`

---

## License

MIT License. Copyright (c) 2026 Davin Maritza.
