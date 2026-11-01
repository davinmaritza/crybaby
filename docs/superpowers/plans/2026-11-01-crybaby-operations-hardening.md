# Crybaby Advanced Operations, Resilience & VM Checkpoints Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver Proxmox-grade virtualization operations for Crybaby cluster, including Hyper-V Gen 2 DVD & SecureBoot compatibility, agent startup self-healing virtual switch, VM checkpoint/snapshot lifecycle, per-VM live resource metrics (CPU/RAM demand), 1-click RDP launcher, Linux Mint systemd autostart service, and controller HTTP ISO repository sharing.

**Architecture:** Monorepo Go extension across `pkg/hyperv`, `pkg/types`, `cmd/agent`, and `cmd/controller`. Hyper-V PowerShell cmdlets (`Checkpoint-VM`, `Restore-VMSnapshot`, `Add-VMDvdDrive`, `Get-VMSwitch`, `New-VMSwitch`) with simulated dual-runner for non-Windows platforms. Reverse-proxy REST API with real-time UI modals on the Tailwind/xterm.js dashboard.

**Tech Stack:** Go 1.22+, Windows PowerShell Hyper-V WMI module, Systemd, Tailwind CSS, FontAwesome, Vanilla JS.

**Spec:** User mandate for items 2 (Self-Healing External Switch), 4 (VM Checkpoint Management), 5 (Realtime VM Metrics), 6 (Systemd Autostart Service), 7 (HTTP ISO Repository), plus prerequisite Item 1 (Gen 2 DVD fix) and Item 3 (1-Click RDP launcher).

## Global Constraints
- Single Go Monorepo (`github.com/davinmaritza/crybaby`).
- Zero Windows Server dependencies (runs on stock Windows 11 Home/Pro client).
- Dual Source of Truth: Repository is implementation source of truth; Notion is knowledge source of truth.
- Dual-runner engine: Real PowerShell WMI on Windows, thread-safe in-memory simulation on Linux.
- Zero lazy placeholders (`// TODO`, `// ... rest of code`). Provide complete, runnable code.

## Review Focus
1. **Gen 2 Hyper-V DVD Drive:** Generation 2 VMs have no DVD drive by default; must use `Add-VMDvdDrive`, not `Set-VMDvdDrive`.
2. **UEFI Secure Boot:** Linux ISOs on Gen 2 fail unless Secure Boot is disabled or configured with `MicrosoftUEFICertificateAuthority`.
3. **Agent Self-Healing Switch:** If Hyper-V was just enabled and the node rebooted, `Crybaby-LAN` must be auto-created on agent startup without manual intervention.
4. **Checkpoint Restoration Safety:** Restoring a checkpoint while VM is running must handle confirmation prompt (`-Confirm:$false`) and state synchronization safely.
5. **Systemd Service Permissions:** Working directory and executable paths must be absolute and runnable under standard user privileges.

---

### Task 1: Hyper-V Gen 2 DVD Drive & Linux SecureBoot Compatibility

**Files:**
- Modify: `pkg/hyperv/engine.go:135-165`
- Test: `pkg/hyperv/engine_test.go`

**Interfaces:**
- Consumes: `types.CreateVMRequest`
- Produces: Corrected PowerShell script for Gen 2 DVD mounting (`Add-VMDvdDrive`) and Secure Boot policy.

- [ ] **Step 1: Write test for ISO drive mounting and SecureBoot command generation**
Update `pkg/hyperv/engine_test.go` to assert that simulated engine correctly registers ISO path and test script generation.

- [ ] **Step 2: Run test to verify current state**
Run: `go test -v ./pkg/hyperv`

- [ ] **Step 3: Implement minimal code in `pkg/hyperv/engine.go`**
Update `CreateVM` in `windowsEngine`:
```go
	isoCommand := ""
	if req.ISOPath != "" {
		isoCommand = fmt.Sprintf(`
		Add-VMDvdDrive -VMName "%s" -Path "%s"
		$dvd = Get-VMDvdDrive -VMName "%s"
		Set-VMFirmware -VMName "%s" -FirstBootDevice $dvd -EnableSecureBoot $false
		`, req.Name, req.ISOPath, req.Name, req.Name)
	}
```

- [ ] **Step 4: Run test to verify it passes**
Run: `go test -v ./pkg/hyperv`

- [ ] **Step 5: Commit**
```bash
git add pkg/hyperv/engine.go pkg/hyperv/engine_test.go
git commit -m "fix(hyperv): use Add-VMDvdDrive for Gen 2 VMs and disable secure boot for ISO compatibility"
```

---

### Task 2: Self-Healing External Switch on Agent Startup

**Files:**
- Modify: `pkg/hyperv/engine.go` (add `EnsureVirtualSwitch(name string) error`)
- Modify: `cmd/agent/main.go` (invoke on startup)
- Test: `pkg/hyperv/engine_test.go`

**Interfaces:**
- Consumes: `Engine.EnsureVirtualSwitch(switchName string) error`
- Produces: Automatic external switch creation bridged to active physical Ethernet NIC.

- [ ] **Step 1: Define `EnsureVirtualSwitch` in `Engine` interface**
Add method signature to `pkg/hyperv/engine.go` and implement in `simulatedEngine` and `windowsEngine`.

- [ ] **Step 2: Implement `EnsureVirtualSwitch` for Windows**
```powershell
$ext = Get-VMSwitch -Name "%s" -ErrorAction SilentlyContinue
if (-not $ext) {
    $eth = Get-NetAdapter -Physical -ErrorAction SilentlyContinue | Where-Object { $_.Status -eq "Up" } | Select-Object -First 1
    if ($eth) {
        New-VMSwitch -Name "%s" -NetAdapterName $eth.Name -AllowManagementOS $true -ErrorAction SilentlyContinue | Out-Null
    }
}
```

- [ ] **Step 3: Call `EnsureVirtualSwitch("Crybaby-LAN")` on Agent Startup**
In `cmd/agent/main.go`, invoke in a background goroutine so it doesn't block agent registration if Hyper-V is busy.

- [ ] **Step 4: Run test suite to verify**
Run: `go test -v ./...`

- [ ] **Step 5: Commit**
```bash
git add pkg/hyperv/ cmd/agent/
git commit -m "feat(hyperv): add self-healing external switch Crybaby-LAN on agent startup"
```

---

### Task 3: VM Checkpoint / Snapshot Data Model & Engine

**Files:**
- Modify: `pkg/types/vm.go` (add `VMCheckpoint`, `CreateCheckpointRequest`)
- Modify: `pkg/hyperv/engine.go` (add checkpoint methods to `Engine` interface & implementations)
- Test: `pkg/hyperv/engine_test.go`

**Interfaces:**
- Produces:
  - `CreateCheckpoint(vmID, name string) (*types.VMCheckpoint, error)`
  - `ListCheckpoints(vmID string) ([]types.VMCheckpoint, error)`
  - `RestoreCheckpoint(vmID, name string) error`
  - `DeleteCheckpoint(vmID, name string) error`

- [ ] **Step 1: Add types in `pkg/types/vm.go`**
```go
type VMCheckpoint struct {
	ID        string    `json:"id"`
	VMID      string    `json:"vm_id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type CreateCheckpointRequest struct {
	Name string `json:"name"`
}
```

- [ ] **Step 2: Add interface methods and simulated engine implementation**
Implement thread-safe in-memory checkpoints in `simulatedEngine`.

- [ ] **Step 3: Implement Windows PowerShell cmdlets in `windowsEngine`**
Using `Checkpoint-VM`, `Get-VMSnapshot`, `Restore-VMSnapshot`, and `Remove-VMSnapshot`.

- [ ] **Step 4: Write tests in `pkg/hyperv/engine_test.go`**
Test checkpoint creation, listing, restoration, and deletion.

- [ ] **Step 5: Run tests to verify**
Run: `go test -v ./pkg/hyperv`

- [ ] **Step 6: Commit**
```bash
git add pkg/types/vm.go pkg/hyperv/engine.go pkg/hyperv/engine_test.go
git commit -m "feat(hyperv): add VM checkpoint and snapshot management engine"
```

---

### Task 4: Checkpoint REST API Endpoints & Controller Proxy

**Files:**
- Modify: `cmd/agent/main.go` (register checkpoint HTTP handlers)
- Modify: `cmd/controller/main.go` (register controller proxy handlers)

**Interfaces:**
- Produces:
  - `POST /api/v1/nodes/:id/vms/:vm_id/checkpoints`
  - `GET /api/v1/nodes/:id/vms/:vm_id/checkpoints`
  - `POST /api/v1/nodes/:id/vms/:vm_id/checkpoints/restore`
  - `DELETE /api/v1/nodes/:id/vms/:vm_id/checkpoints/:name`

- [ ] **Step 1: Implement agent endpoints in `cmd/agent/main.go`**
- [ ] **Step 2: Implement controller proxy endpoints in `cmd/controller/main.go`**
- [ ] **Step 3: Test endpoints with `go test` and integration probe**
- [ ] **Step 4: Commit**
```bash
git add cmd/agent/main.go cmd/controller/main.go
git commit -m "feat(api): expose REST endpoints for VM checkpoint lifecycle"
```

---

### Task 5: Real-Time Per-VM Resource Metrics (CPU% & Active RAM Demand)

**Files:**
- Modify: `pkg/types/vm.go` (add `CPUUsagePct int`, `MemoryDemandMB uint64`)
- Modify: `pkg/hyperv/engine.go` (extract in `ListVMs`)
- Modify: `cmd/controller/dashboard.go` (display in VM table)

**Interfaces:**
- Produces: Real-time CPU % badge and RAM demand indicator in dashboard VM table.

- [ ] **Step 1: Add fields to `types.VirtualMachine`**
- [ ] **Step 2: Extract `CPUUsage` and `MemoryDemand` in PowerShell query**
- [ ] **Step 3: Update `cmd/controller/dashboard.go` table rendering**
- [ ] **Step 4: Run tests and verify**
- [ ] **Step 5: Commit**
```bash
git add pkg/types/vm.go pkg/hyperv/engine.go cmd/controller/dashboard.go
git commit -m "feat(telemetry): expose per-VM CPU usage and memory demand metrics"
```

---

### Task 6: 1-Click RDP Launcher & Checkpoint Modal UI

**Files:**
- Modify: `cmd/controller/dashboard.go`

**Interfaces:**
- Produces:
  - 1-Click `.rdp` download button in VM table.
  - Snapshot management modal (List, Create, Rollback, Delete).

- [ ] **Step 1: Add `downloadRdp(vmName, ip)` JavaScript function**
Generates data URI / blob download for `.rdp` file with standard RDP settings.
- [ ] **Step 2: Add Snapshots modal and control buttons**
- [ ] **Step 3: Verify dashboard UI rendering and interactions**
- [ ] **Step 4: Commit**
```bash
git add cmd/controller/dashboard.go
git commit -m "feat(dashboard): add 1-click RDP launcher and VM snapshot modal"
```

---

### Task 7: Linux Mint Systemd Autostart Service

**Files:**
- Create: `scripts/systemd/crybaby-controller.service`
- Create: `scripts/systemd/install-service.sh`

**Interfaces:**
- Produces: Always-on controller background daemon on Linux Mint.

- [ ] **Step 1: Write `crybaby-controller.service` unit file**
- [ ] **Step 2: Write `install-service.sh` with systemctl enable & start instructions**
- [ ] **Step 3: Verify service syntax**
- [ ] **Step 4: Commit**
```bash
git add scripts/systemd/
git commit -m "feat(systemd): add systemd autostart service for controller on Linux Mint"
```

---

### Task 8: Controller HTTP ISO Repository Sharing

**Files:**
- Modify: `cmd/controller/main.go` (add static file server `/iso/` pointing to `./iso`)
- Modify: `cmd/controller/dashboard.go` (display ISO repository link & helper in USB/VM modal)

**Interfaces:**
- Produces: HTTP endpoint `http://192.168.100.168:8080/iso/` serving OS installation ISO files directly to LAN nodes.

- [ ] **Step 1: Create local directory `iso/` with `.gitkeep`**
- [ ] **Step 2: Add static file handler in `cmd/controller/main.go`**
- [ ] **Step 3: Update dashboard documentation and VM modal helper**
- [ ] **Step 4: Run test suite**
- [ ] **Step 5: Commit**
```bash
git add cmd/controller/ iso/
git commit -m "feat(iso): add controller HTTP ISO repository server for LAN distribution"
```

---

### Task 9: Cross-Compilation, Verification, and Notion Sync

**Files:**
- Binary outputs: `bin/crybaby-controller`, `bin/crybaby-agent.exe`, `bin/crybaby-controller.exe`
- Documentation: `README.md`, `chat.md`
- Notion Sync: Pages 01, 05 (ADR-011, ADR-012), 07, 13, 14, 15, 18

- [ ] **Step 1: Build all binaries for Linux and Windows**
- [ ] **Step 2: Run all unit and integration tests**
- [ ] **Step 3: Update persistent workspace memory in `chat.md`**
- [ ] **Step 4: Sync all updated features to Notion child pages via Notion MCP**
- [ ] **Step 5: Commit and push to GitHub repository**
