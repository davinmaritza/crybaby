# Crybaby ⚡

> **Distributed Windows 11 Cluster Orchestrator & Hyper-V Remote Manager**

Crybaby transforms standard consumer Windows 11 workstations into a unified, high-performance computing cluster managed seamlessly from your laptop dashboard. It features zero-config P2P WireGuard mesh networking, 1-click USB flashdisk node provisioning, and in-browser hybrid remote operations (PowerShell web terminal & Web RDP).

---

## 🏗️ Architecture Blueprint

- **Controller Core (`cmd/controller`):** Lightweight Go daemon with embedded React (Vite) + Tailwind CSS web dashboard (`:8080`), WireGuard mesh coordinator, and session multiplexer.
- **Node Agent (`cmd/agent`):** Background Windows Service (`crybaby-agent.exe`) compiled in Go (<20 MB RAM footprint), interfacing with Hyper-V WMI v2 (`root\virtualization\v2`) and PowerShell cmdlets.
- **Overlay Networking:** Point-to-Point WireGuard mesh (`10.42.0.0/16`) for zero-NAT, firewall-traversing cluster communication.
- **Provisioning Engine:** 1-Click USB generator producing an automated service installer with anti-sleep policy and Wake-on-LAN (WoL) registration.

```
           ┌────────────────────────────────────────┐
           │        Crybaby Laptop Controller       │
           │  (Go Daemon + React Vite Web UI :8080) │
           └──────────────────┬─────────────────────┘
                              │
            ┌─────────────────┴─────────────────┐
            │   WireGuard Encrypted Mesh Layer  │
            │   (Zero-NAT, P2P End-to-End TLS)  │
            └─────────┬─────────────────┬───────┘
                      │                 │
                      ▼                 ▼
         ┌──────────────────────┐   ┌──────────────────────┐
         │  Windows 11 Node #1  │   │  Windows 11 Node #2  │
         │  (crybaby-agent.exe) │   │  (crybaby-agent.exe) │
         ├──────────────────────┤   ├──────────────────────┤
         │ • Win Service Core   │   │ • Win Service Core   │
         │ • Hyper-V WMI Engine │   │ • Hyper-V WMI Engine │
         │ • Telemetry / WoL    │   │ • Telemetry / WoL    │
         │ • Web Terminal PTY   │   │ • Web Terminal PTY   │
         │ • Local VHDX Storage │   │ • Local VHDX Storage │
         └──────────────────────┘   └──────────────────────┘
```

---

## 📖 Project Knowledge System

This project strictly adheres to a **Dual-Plane Single Source of Truth**:
1. **Implementation Source of Truth:** This Git repository holds all actual code, configurations, schemas, and tests.
2. **Knowledge Source of Truth:** The Notion workspace maintains all architectural decisions (ADRs), product requirements, technical context, issue tracking, and persistent memory.

- **Notion Root Page:** `Crybaby` (`40a13039-c767-8090-a003-c96f7c8c6a28`)
- **Agent Directives:** [`.claude/CLAUDE.md`](.claude/CLAUDE.md)

---

## 🔄 Development Cycle

All engineering tasks follow the 9-step closed-loop cycle:

`Read Context -> Inspect Repository -> Understand Task -> Plan -> Implement -> Validate -> Update Knowledge -> Sync Notion -> Update Context`
