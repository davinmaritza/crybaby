# Proxmox-Grade Advanced Hyper-V Suite (Milestone 6) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Transform Crybaby into an enterprise-grade Proxmox VE alternative for consumer Windows 11 clusters by implementing Smart ISO Catalog with Node Auto-Caching, Dynamic VM Specs Resizing (vCPU/RAM), VHDX Virtual Disk Expansion, 1-Click Fast VM Cloning, and In-Browser Live Telemetry Performance Sparklines.

**Architecture:** 
1. **ISO Catalog & Auto-Cache:** Controller scans `./iso` and serves catalog via REST API, supports background ISO downloads from remote URLs into `./iso`, and Windows Agent automatically downloads/caches ISOs over LAN into `C:\Crybaby\ISOs` when referenced by HTTP URL.
2. **Dynamic Specs & Disk Expansion:** Hyper-V engine introduces `ResizeVM` (`Set-VMProcessor`, `Set-VMMemory`) and `ExpandDisk` (`Resize-VHD`) with simulated dual-runner support for cross-platform Linux development.
3. **Rapid VM Cloning:** Hyper-V engine introduces `CloneVM` creating isolated cloned VMs from existing running/stopped instances using copied/differencing VHDX disks.
4. **Live Performance Sparklines:** Controller hub tracks rolling historical telemetry and dashboard renders responsive, zero-CDN embedded SVG/Canvas sparklines visualizing cluster CPU and RAM trends over time.

**Tech Stack:** Go 1.23, PowerShell Hyper-V WMI cmdlets, Windows Service API, HTTP/REST, Canvas/SVG telemetry, Tailwind CSS.

**Spec:** `docs/superpowers/plans/2026-11-01-crybaby-proxmox-hyperv-suite.md`

## Global Constraints

- Never break backward compatibility for existing VM creation or cluster operations.
- Zero external CDN dependencies for telemetry sparklines (use self-contained lightweight Canvas / SVG rendering).
- Prevent SSRF and command injection on ISO download URLs, VM names, and resize parameters.
- All code must pass `go test -count=1 ./...` with zero failures.
- Zero lazy placeholders (`// TODO`, `// ...`).

## Review Focus

1. ISO HTTP URL handling on Windows agent: Node validates LAN URL and caches cleanly in `C:\Crybaby\ISOs` before `Add-VMDvdDrive`.
2. VM Resize validation: Core count >= 1 and Memory >= 512 MB, with bounds checking before executing PowerShell.
3. Disk expansion validation: Target size must be strictly larger than current VHDX size.
4. VM clone collision: Cloning to an already existing VM name returns clear 409 conflict error.
5. In-flight ISO download cancellation or error: Controller removes partial file and returns descriptive JSON error.

---

### Task 1: ISO Repository Catalog & Background URL Downloader

**Files:**
- Create: `pkg/iso/catalog.go` & `pkg/iso/catalog_test.go`
- Modify: `cmd/controller/main.go`
- Test: `pkg/iso/catalog_test.go`

**Interfaces:**
- `iso.ListISOs(dir string) ([]ISOFile, error)`
- `iso.DownloadISO(url, destDir, filename string) (*DownloadJob, error)`
- `GET /api/v1/iso`: Returns array of available ISO files in `./iso`.
- `POST /api/v1/iso/download`: Starts background URL download into `./iso`.
- `GET /api/v1/iso/downloads`: Returns active download jobs with progress percentage.

- [ ] **Step 1: Write failing tests in `pkg/iso/catalog_test.go` for catalog listing and safe URL download**
- [ ] **Step 2: Implement `pkg/iso/catalog.go` with background streaming and thread-safe progress**
- [ ] **Step 3: Register ISO catalog and download endpoints in `cmd/controller/main.go`**
- [ ] **Step 4: Verify tests pass**

---

### Task 2: Windows Agent ISO Auto-Caching & SCSI Mounting

**Files:**
- Modify: `pkg/hyperv/engine.go`
- Modify: `pkg/hyperv/engine_test.go`

**Interfaces:**
- Update `CreateVM` in `pkg/hyperv/engine.go`: If `req.ISOPath` matches `^https?://`, auto-download and cache to `C:\Crybaby\ISOs\<filename>` if absent, then mount local file path.
- In `simulatedEngine`: Support simulated ISO caching and verify attachment.

- [ ] **Step 1: Add unit tests in `pkg/hyperv/engine_test.go` for HTTP ISO auto-caching**
- [ ] **Step 2: Implement auto-caching routine in `windowsEngine.CreateVM` and `simulatedEngine.CreateVM`**
- [ ] **Step 3: Verify tests pass**

---

### Task 3: Hyper-V VM Dynamic Resizing & Disk Expansion

**Files:**
- Modify: `pkg/types/vm.go`
- Modify: `pkg/hyperv/engine.go`
- Modify: `pkg/hyperv/engine_test.go`
- Modify: `cmd/agent/main.go`
- Modify: `cmd/controller/main.go`

**Interfaces:**
- `ResizeVM(vmID string, cpuCount int, memoryMB int) error`
- `ExpandDisk(vmID string, newSizeGB int) error`
- Agent endpoints:
  - `POST /api/v1/vms/:vm_id/resize` with `{"cpu_count": 4, "memory_mb": 4096}`
  - `POST /api/v1/vms/:vm_id/expand-disk` with `{"new_size_gb": 64}`
- Controller proxy routes for resize and expand-disk.

- [ ] **Step 1: Write failing tests in `pkg/hyperv/engine_test.go` for `ResizeVM` and `ExpandDisk`**
- [ ] **Step 2: Add `ResizeVM` and `ExpandDisk` to `hyperv.Engine` interface and implementations**
- [ ] **Step 3: Wire endpoints in `cmd/agent/main.go` and proxy in `cmd/controller/main.go`**
- [ ] **Step 4: Verify tests pass**

---

### Task 4: Fast VM Cloning Engine

**Files:**
- Modify: `pkg/types/vm.go`
- Modify: `pkg/hyperv/engine.go`
- Modify: `pkg/hyperv/engine_test.go`
- Modify: `cmd/agent/main.go`
- Modify: `cmd/controller/main.go`

**Interfaces:**
- `CloneVM(sourceVMID, newVMName string) (*types.VirtualMachine, error)`
- Agent endpoint: `POST /api/v1/vms/:vm_id/clone` with `{"new_name": "ubuntu-clone-01"}`
- Controller proxy route for cloning.

- [ ] **Step 1: Write failing tests in `pkg/hyperv/engine_test.go` for `CloneVM`**
- [ ] **Step 2: Implement `CloneVM` in `windowsEngine` and `simulatedEngine`**
- [ ] **Step 3: Wire clone endpoint in `cmd/agent/main.go` and `cmd/controller/main.go`**
- [ ] **Step 4: Verify tests pass**

---

### Task 5: Web Dashboard UI Integration & Telemetry Sparklines

**Files:**
- Modify: `cmd/controller/dashboard.go`
- Modify: `cmd/controller/main.go`

**Interfaces:**
- Telemetry buffer: Controller maintains rolling 30-sample history for cluster CPU and RAM.
- Dashboard UI:
  - Embedded SVG/Canvas sparklines in cluster metrics cards.
  - ISO Selector dropdown populated via `/api/v1/iso` in "New VM" modal.
  - "Download ISO from URL" modal with live progress polling.
  - "Edit Specs" button and modal on each VM row (vCPU & RAM adjustment).
  - "Expand Disk" button and modal on each VM row.
  - "Clone VM" button and modal on each VM row.

- [ ] **Step 1: Add telemetry rolling history in `cmd/controller/main.go`**
- [ ] **Step 2: Update `cmd/controller/dashboard.go` with modals, sparkline canvas, and action buttons**
- [ ] **Step 3: Verify dashboard builds and renders cleanly**

---

### Task 6: End-to-End Verification, Cross-Compilation & Dual-Truth Sync

**Files:**
- Run: `go test -count=1 ./...`
- Compile: `bin/crybaby-controller`, `bin/crybaby-agent.exe`, etc.
- Modify: `README.md`
- Sync: Notion workspace (pages `01`, `05`, `07`, `13`, `14`, `15`, `18`, Root)
- Modify: `chat.md`

- [ ] **Step 1: Execute full test suite across all packages**
- [ ] **Step 2: Compile fresh Windows AMD64 and Linux binaries**
- [ ] **Step 3: Commit and push changes to GitHub**
- [ ] **Step 4: Sync Notion workspace and update `chat.md`**
