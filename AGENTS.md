# AGENTS.md

Instructions for AI coding agents working on this project.

## Stack

- **Language**: Go 1.21+
- **Message broker**: Apache Kafka 3.x (Sarama client)
- **State store**: Redis 7.x (orchestration approach)
- **Database**: PostgreSQL 15.x (one DB per service)
- **Containerization**: Docker Compose 2.x
- **Monitoring**: Prometheus + Grafana

## Commands

```bash
# Build all binaries
go build ./cmd/...

# Run tests
go test ./...

# Start infrastructure + services (default choreography)
docker compose up -d

# Switch approach (choreography|orchestration)
#   PowerShell: $env:APPROACH = "orchestration"; docker compose up -d
#   bash:       APPROACH=orchestration docker compose up -d

# Setup (Kafka topics)
bash scripts/setup.sh

# Reset (truncate DB + flush Redis + fault counters)
bash scripts/reset.sh

# Run scenario (30 iterations) - Windows PowerShell
.\scripts\run-scenario.ps1 -Scenario <S1|S2|S3|S6|S7> -Approach <choreography|orchestration> -Runs 30

# Run scenario (30 iterations) - bash with Go installed
bash scripts/run-scenario.sh <S1|S2|S3|S6|S7> <choreography|orchestration> 30
```

## Conventions

- **JSON tags**: `snake_case` (e.g., `saga_id`, `created_at`)
- **Struct fields**: `PascalCase` for exported fields
- **Package names**: `lowercase`, single word (e.g., `common`, `choreography`)
- **Error handling**: Return error, never panic in library code. Panic only in `main.go`.
- **Saga states**: `pending`, `committed`, `compensating`, `compensated`, `failed`
- **HTTP handlers**: Standard `net/http` — no framework
- **SQL**: Use `pgxpool` with prepared statements; schema in `db.go`
- **Kafka**: Sarama; topic names hardcoded in `choreography/topics.go`
- **Redis**: Use `go-redis/v9` client wrapper in `pkg/redis`
- **Documentation**: Update README.md + AGENTS.md when conventions change

## Folder Purpose

- `cmd/`: One `main.go` per binary. Never import logic here — call `internal` or `pkg`.
- `internal/`: Private logic. Importable by `pkg/` and `cmd/`, not exposed externally.
- `pkg/`: Reusable wrappers (kafka, postgres, redis). Can be imported by any cmd.
- `scripts/`: Bash helpers (setup, reset, scenarios).
- `deployments/`: Docker Compose files.
- `docs/`: Human-readable markdown docs.

## Testing

- Integration test uses `t.Skip` if Docker is unavailable.
- Scenarios are run via bash script, not `go test`, due to long-running containers.

## Notes

- `docs/runs/` contains raw JSON results — git-ignored.
- Proposal source: `Ariel Naviandana Putra - 235150701111010 - Proposal.docx` (external, in Downloads).
