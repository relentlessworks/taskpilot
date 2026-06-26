# TaskPilot

> Agentic-first task manager. The agent IS the interface.

TaskPilot is a task management service designed for AI agents to drive over plain HTTP. No UI, no SDK. The API is the product.

## Quick Start

```bash
make build
./taskpilot
```

The server starts on `:8080` with zero config. Data is persisted to a JSON file automatically.

## How It Works

1. **Get a token**: `POST /auth/request` → `POST /auth/verify` → bearer token
2. **Create a task**: `POST /api/tasks` with `title=Fix the bug&priority=high`
3. **List tasks**: `GET /api/tasks` or `GET /api/tasks?status=todo`
4. **Update a task**: `PATCH /api/tasks/task_a1b2c` with `status=done`
5. **Delete a task**: `DELETE /api/tasks/task_a1b2c`

## Principles

- **Plain text by default** — one labeled, grepable line per record. JSON on demand via `Accept: application/json` or `?format=json`.
- **Instructive errors** — every 4xx includes a hint telling the agent what to do next.
- **Self-documenting** — `GET /help` returns a one-page operating manual.
- **Simple auth** — OTP via email → long-lived bearer token.
- **Single static binary** — Go, zero external dependencies, deploys as one file.
- **Zero config defaults** — runs out of the box. Config: defaults < env < flags.
- **Multi-tenant** — workspaces isolate tasks per tenant.
- **Short stable handles** — every task gets a workspace-scoped handle like `task_k7m2q`.
- **Flexible input** — status and priority accept common aliases (e.g. `completed` → `done`, `urgent` → `high`).

## Configuration

| Flag | Env | Default | Description |
|------|-----|---------|-------------|
| `-addr` | `TASKPILOT_ADDR` | `:8080` | Listen address |
| `-db` | `TASKPILOT_DB` | `taskpilot.json` | Data file path |
| `-secret` | `TASKPILOT_SECRET` | random | Token signing secret |

## Build

```bash
make build    # CGO_ENABLED=0, single static binary
make test     # go test ./... -race
make vet      # go vet ./...
```

## API Reference

### Authentication

```
POST /auth/request   email=<email>&workspace=<handle>  → OTP code
POST /auth/verify     email=<email>&code=<code>          → Bearer token
```

### Tasks (requires Bearer token)

```
POST   /api/tasks          title=<summary>&priority=<low|medium|high>&description=<optional>  → handle=task_xxx
GET    /api/tasks          [?status=todo|in_progress|done]                                  → list tasks
GET    /api/tasks/<handle>                                                                 → task details
PATCH  /api/tasks/<handle>  status=...&priority=...&title=...&description=...               → updated task
DELETE /api/tasks/<handle>                                                                 → deleted
GET    /api/workspace                                                                       → workspace info
```

### Status Values

`todo`, `in_progress`, `done`

Aliases accepted on create/update: `open`, `new`, `pending` → `todo`; `doing`, `started`, `active` → `in_progress`; `completed`, `closed`, `finished` → `done`

### Priority Values

`low`, `medium`, `high`

Aliases accepted: `p3` → `low`; `p2`, `normal` → `medium`; `p1`, `urgent`, `critical` → `high`

### Formats

- **Plain text** (default): `handle=task_a1b2c title=Fix bug status=todo priority=high`
- **JSON**: add `Accept: application/json` or `?format=json`

## License

MIT
