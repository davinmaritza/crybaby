# Crybaby - Persistent Project & Knowledge Rules

> **Project Name:** Crybaby
> **Notion Root Page:** `Crybaby` (ID: `40a13039-c767-8090-a003-c96f7c8c6a28`)
> **Knowledge Engine:** Dual-Plane Architecture (Repository Implementation + Notion Knowledge System)

---

## 1. Operating Principles & Source of Truth

1. **Repository Authority:** The Git repository is the absolute Single Source of Truth for actual code implementation, configurations, and running systems.
2. **Notion Knowledge Authority:** Notion is the Single Source of Truth for high-level project knowledge, architecture, decision records (ADRs), issues, tasks, and persistent agent memory.
3. **Conflict Resolution:** If a discrepancy arises between Notion and the repository, the repository implementation always takes precedence. Synchronize Notion immediately after verification.
4. **Standalone Context:** Crybaby is strictly self-contained. Never import context, preferences, or memory from other projects.
5. **No Synthetic / Hallucinated Tech:** Only document and implement what has been explicitly verified in the repository or mandated by user requirements.

---

## 2. Standard Development Workflow

Every task executed by Claude Code must strictly follow this 9-step cycle:

```
Read Context ──► Inspect Repository ──► Understand Task ──► Plan ──► Implement
                                                                        │
Update Context ◄── Sync Notion ◄── Update Knowledge ◄── Validate ◄──────┘
```

1. **Read Context:** Consult `.claude/CLAUDE.md` and relevant Notion child pages (`18. Claude Code Context`, `01. Current State`, `05. Project Decisions`).
2. **Inspect Repository:** Verify local file structure, Git status, and actual implementation state before making assumptions.
3. **Understand Task:** Clarify constraints, requirements, and edge cases.
4. **Plan:** Formulate minimal blast-radius architectural plan. Consult `05. Project Decisions` before proposing architectural changes.
5. **Implement:** Execute surgical, full implementations (zero `// TODO` or lazy truncation).
6. **Validate:** Run automated tests, linters, and type checkers. Require evidence before assertions.
7. **Update Knowledge:** Determine affected knowledge areas (architecture, API, database, security, decisions).
8. **Sync Notion:** Push updates to corresponding Notion child pages via Notion MCP.
9. **Update Context:** Update `18. Claude Code Context` with the latest objective, completed work, active files, and `Last Context Sync` timestamp.

---

## 3. Notion Synchronization Rules

Claude Code must enforce the following synchronization checklist after every meaningful task:

- [ ] **Current State (`01. Current State`):** Update when development status, priority, or blockers shift.
- [ ] **Project Memory (`15. Project Memory`):** Update when long-term technical discoveries, workarounds, or lessons learned emerge.
- [ ] **Project Decisions (`05. Project Decisions`):** Record new architectural decisions. Never delete old decisions; mark replaced ones as `SUPERSEDED`.
- [ ] **Features & Modules (`07. Features & Modules`):** Update when behavior, API contracts, or files change.
- [ ] **Issues & Debt (`12. Known Issues & Technical Debt`):** Update when bugs or technical debts are identified, worked around, or resolved.
- [ ] **Tasks & Progress (`17. Tasks & Next Steps` / `13. Progress`):** Update task statuses (`TODO`, `IN PROGRESS`, `DONE`) and milestone achievements.
- [ ] **Claude Code Context (`18. Claude Code Context`):** Always refresh after finishing work to ensure instant session resumption.

---

## 4. Context Recovery Protocol

If session history or context is lost, Claude Code must recover state without prompting the user to repeat information:

1. Read `/home/suzirz/Documents/crybaby/.claude/CLAUDE.md`.
2. Access Notion Root Page `Crybaby` (`40a13039-c767-8090-a003-c96f7c8c6a28`).
3. Read `18. Claude Code Context`.
4. Read `01. Current State` and `15. Project Memory`.
5. Read relevant active items from `05. Project Decisions`, `07. Features & Modules`, and `17. Tasks & Next Steps`.
6. Inspect repository via `git status` and file tools.
7. Reconcile any divergence between Notion and repository.
8. Resume execution directly from `Next Action`.

---

## 5. Security & Quarantine Rules

- **Zero Secret Storage:** Never write API keys, passwords, bearer tokens, private keys, database credentials, or sensitive personal data to Notion or Git.
- **Placeholder Pattern:** Use explicit sanitization placeholders (e.g., `API_KEY=<PLACEHOLDER_API_KEY>`).
- **Local Isolation:** Ensure all active secrets reside exclusively in local `.env` files protected by `.gitignore`.

---

## 6. Coding & Engineering Standards

- **Zero Lazy Truncation:** Never output placeholders, ellipses (`...`), or incomplete code blocks.
- **Surgical Edits:** Keep changes localized and maintain codebase idioms.
- **Strict Error Handling:** Explicit typing and defensive error handling; never swallow exceptions.
- **Evidence Before Assertions:** All completion claims must be backed by real command executions and clean exit codes.
