# Crybaby

Autonomous software engineering workspace for project **Crybaby**.

## Project Knowledge System

This project operates on a dual-plane Single Source of Truth architecture:

1. **Implementation Source of Truth:** This Git repository holds all actual code, configurations, schemas, and tests.
2. **Knowledge Source of Truth:** The Notion workspace maintains all architecture decision records (ADRs), product requirements, technical context, issue logs, and persistent memory.

- **Notion Root Page:** `Crybaby` (`40a13039-c767-8090-a003-c96f7c8c6a28`)
- **Agent Enforcement Layer:** [`.claude/CLAUDE.md`](.claude/CLAUDE.md)

## Development Workflow

All contributors and AI agents follow the 9-step closed-loop cycle:

`Read Context -> Inspect Repository -> Understand Task -> Plan -> Implement -> Validate -> Update Knowledge -> Sync Notion -> Update Context`
