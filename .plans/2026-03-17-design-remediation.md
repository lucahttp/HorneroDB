# Design Remediation Plan
**Date:** 2026-03-17
**Goal:** Fix architectural flaws and design holes identified in the project against the `INSTRUCTIONS.md` guidelines.

## Progress

| Status | Task | Description | Risk Level |
|---|---|---|---|
| [x] | 1. Implement Proper Service Layer | Extract business logic from `record.go` into a reusable `RecordService`. | High |
| [x] | 2. DDL Database Transactions | Ensure `CreateTable` and `DeleteTable` in `table.go` run inside GORM transactions for atomicity. | High |
| [x] | 3. Authentication DB Optimization | Refactor `auth.go` middleware to eliminate the per-request `_hornero_users` database lookup if using JWT. | Medium |
| [x] | 4. Webhook Reliability (Outbox) | Refactor `DispatchWebhookAsync` to use a DB-backed outbox or persistent queue instead of fire-and-forget goroutines. | Medium |
| [x] | 5. Frontend Global State | Introduce `WorkspaceContext` and `AuthContext` to eliminate prop drilling across the app. | Medium |
| [x] | 6. Modularize TableView.jsx | Extract data fetching to hooks and sub-components (like CSV Wizard) out of `TableView.jsx`. | Low |

## Implementation Details

### highest-risk tasks first:
1. **Proper Service Layer & Handlers decoupling** is the highest risk because it touches the core API endpoints that all connectors and the UI rely on. Breaking this breaks everything.
2. **DDL Transactions** is critical for data integrity. If the schema gets out of sync with physical tables, the workspace is corrupted.

### Notes
- We must ensure we follow "DX First", so if we touch APIs, we ensure the JS frontend consumes the changes perfectly without breaking the user experience.
- The plan will evolve as we touch the code. If extracting the Service Layer proves too complex in one shot, we will break it down by Endpoint (e.g., `CreateRecord` first, then `UpdateRecord`).
