# 1-Click Remote OTA Update Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement a secure 1-click Over-The-Air (OTA) remote self-update mechanism allowing the laptop controller to seamlessly update `crybaby-agent.exe` on all Windows 11 cluster nodes over the local LAN switch without manual flashdisk re-installation.

**Architecture:** The laptop controller serves the freshly compiled agent binary (`bin/crybaby-agent.exe`) via an authenticated HTTP endpoint. Cluster nodes report their running version during registration and periodic heartbeats. When a node's version lags behind the controller's version, the web dashboard displays a visual update notification with a 1-click update action that dispatches an RPC update command. The Windows agent downloads the new binary, validates its payload, and spawns a detached self-terminating background updater script (`apply-update.bat`) that safely handles Windows executable file-locking, stops the Windows Service, replaces the executable, restarts `CrybabyAgent`, and cleans itself up.

**Tech Stack:** Go 1.23, Windows Service API, Windows CMD batch detachment (`start /b`), HTTP/REST, Tailwind CSS dashboard, JSON telemetry.

**Spec:** `docs/superpowers/plans/2026-11-01-crybaby-ota-update.md`

## Global Constraints

- Never break backward compatibility for existing nodes running without version reporting (default to `v0.4.0`).
- Ensure Windows executable file locking is completely bypassed via detached script execution without hanging the agent process.
- Strictly validate downloaded binary size and integrity before attempting service replacement.
- All code must pass `go test -v ./...` with zero failures.
- Zero lazy placeholders (`// TODO`, `// ...`).

## Review Focus

1. Node running older version reports `""` or `v0.4.0`: Controller gracefully treats as outdated and displays update prompt.
2. Binary missing on controller (`bin/crybaby-agent.exe` not built): Controller returns clear HTTP 404 JSON error with build instructions.
3. Network blip during binary download on Windows node: Agent detects truncated download, rejects update, logs error, and leaves current service untouched.
4. Windows Service permission: Batch script runs with LocalSystem / Administrator privileges inheriting service security context.
5. Concurrent update requests: Agent rejects duplicate update triggers while an update is already in progress.

---

### Task 1: Type Definitions & Node Version Reporting

**Files:**
- Modify: `pkg/types/node.go`
- Modify: `pkg/cluster/registry.go`
- Test: `pkg/cluster/registry_test.go`

**Interfaces:**
- `types.Node`: Add `Version string json:"version"`
- `types.RegisterNodeRequest`: Add `Version string json:"version"`
- `types.HeartbeatPayload`: Add `Version string json:"version"`
- `types.AgentUpdateRequest`: Add `DownloadURL string json:"download_url"`, `TargetVersion string json:"target_version"`
- `types.AgentUpdateResponse`: Add `Status string json:"status"`, `Message string json:"message"`

- [ ] **Step 1: Write test in `pkg/cluster/registry_test.go` verifying version tracking**
- [ ] **Step 2: Run test to confirm it fails or tests new field**
- [ ] **Step 3: Update `pkg/types/node.go` and `pkg/cluster/registry.go`**
- [ ] **Step 4: Verify test passes**

---

### Task 2: Controller Binary Server & Update RPC Endpoint

**Files:**
- Modify: `cmd/controller/main.go`
- Test: `cmd/controller/ota_test.go` (or `main_test.go`)

**Interfaces:**
- `GET /api/v1/binaries/crybaby-agent.exe`: Serves compiled Windows agent binary from `./bin/crybaby-agent.exe`.
- `POST /api/v1/nodes/:id/update`: Validates node state, checks binary existence, and proxies update trigger to `http://<node_ip>:<port>/api/v1/agent/update`.

- [ ] **Step 1: Write tests for binary serving and update endpoint dispatch**
- [ ] **Step 2: Implement endpoint handlers in `cmd/controller/main.go`**
- [ ] **Step 3: Run tests to confirm PASS**

---

### Task 3: Agent Self-Update Engine (Windows & Linux)

**Files:**
- Modify: `cmd/agent/main.go`
- Create: `pkg/updater/updater.go` & `pkg/updater/updater_test.go`

**Interfaces:**
- `pkg/updater.ApplyUpdate(downloadURL, targetVersion string, isWindows bool) error`
- Agent endpoint `POST /api/v1/agent/update` handles incoming update request and invokes `pkg/updater.ApplyUpdate`.

- [ ] **Step 1: Write unit tests in `pkg/updater/updater_test.go`**
- [ ] **Step 2: Implement `pkg/updater/updater.go` with safe Windows detached batch script runner and Linux in-place rename**
- [ ] **Step 3: Wire updater into `cmd/agent/main.go` with `const AgentVersion = "0.5.0"`**
- [ ] **Step 4: Run tests to verify exit code 0**

---

### Task 4: Web Dashboard UI Integration

**Files:**
- Modify: `cmd/controller/dashboard.go`

**Interfaces:**
- Display node version badge (`v0.5.0` or outdated `v0.4.x`) in the Fleet table.
- Display Amber badge `Update: v0.5.0` and **"Update"** button for outdated nodes.
- Add JavaScript `updateNode(nodeId, nodeName)` with modal confirmation and non-blocking toast feedback.

- [ ] **Step 1: Add version column and conditional update action button to node rows in `cmd/controller/dashboard.go`**
- [ ] **Step 2: Add client-side `updateNode` and `updateAllNodes` JavaScript handlers**
- [ ] **Step 3: Verify dashboard builds and renders cleanly**

---

### Task 5: End-to-End Verification, Cross-Compilation & Dual-Truth Sync

**Files:**
- Execute: `go test -v ./...`
- Execute: Build Windows and Linux binaries into `bin/`
- Modify: `README.md`
- Sync: Notion pages (`01`, `05`, `07`, `14`, `15`, `18`, Root)
- Modify: `chat.md`

- [ ] **Step 1: Run full test suite across all packages**
- [ ] **Step 2: Compile `bin/crybaby-controller` and `bin/crybaby-agent.exe`**
- [ ] **Step 3: Commit and push changes to GitHub**
- [ ] **Step 4: Sync Notion documentation & update `chat.md`**
