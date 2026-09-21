# Saga Pattern: Choreography vs Orchestration — Skripsi

Analisis perbandingan konsistensi data transaksi pada Saga Pattern (Choreography vs Orchestration) menggunakan Fault Injection Testing di Arsitektur Microservices Containerized.

**Author**: Ariel Naviandana Putra (NIM 235150701111010)
**Universitas**: Fakultas Ilmu Komputer, Universitas Brawijaya — 2026

---

## Struktur Proyek

```
projek-skripsi/
├── cmd/                    # Entry point binaries (6 services)
│   ├── order-service/      # Initiates saga
│   ├── payment-service/    # Process payment
│   ├── inventory-service/  # Reserve stock
│   ├── shipping-service/   # Schedule shipping
│   ├── orchestrator/       # Saga Orchestrator (orchestration approach)
│   └── workload-generator/ # CLI for running scenarios S1-S7
├── internal/               # Shared, not exported
│   ├── common/             # Handlers, DB helpers, fault middleware, DTOs
│   ├── choreography/       # Kafka producer/consumer
│   ├── orchestration/      # HTTP client for orchestrator
│   ├── snapshot/           # Snapshot capture → saga_log
│   ├── consistency/        # Consistency checker
│   └── metrics/            # Prometheus metrics
├── pkg/                    # Reusable library wrappers
│   ├── kafka/              # Sarama-based
│   ├── postgres/           # pgxpool
│   └── redis/              # go-redis
├── deployments/            # Docker Compose variants
│   ├── infrastructure.yml  # Kafka, Redis, Prometheus, Grafana, 4x PostgreSQL
│   └── services.yml        # 4 services + orchestrator (APPROACH env)
├── scripts/                # Helpers
│   ├── setup.sh            # Create topics
│   ├── reset.sh            # Truncate DBs, flush Redis, reset fault counters
│   ├── run-scenario.sh     # Parametric scenario runner (bash/CI)
│   └── run-scenario.ps1    # Parametric scenario runner (Windows PowerShell)
├── test/                   # Integration tests
├── docs/
│   ├── ARCHITECTURE.md     # Diagrams & design choices
│   ├── SCENARIOS.md        # S1-S7 definitions
│   ├── REPORT.md           # Final analysis (post-implementation)
│   ├── EXPLANATION.md      # Experiment explanation & FAQ
│   └── HOW-TO-RUN.md       # Execution guide
├── docker-compose.yml      # Wrapper
└── README.md
```

## Quick Start

```powershell
# Prerequisites: Docker Desktop, Go 1.21+, bash (Windows: via WSL for scripts)
docker compose up -d              # infra + services (default choreography)
bash scripts/setup.sh             # create Kafka topics
```

Setiap service membaca `APPROACH` (`choreography`/`orchestration`). Untuk
berpindah pendekatan, set environment lalu `docker compose up -d`:

```powershell
$env:APPROACH = "orchestration"
docker compose up -d order-service payment-service inventory-service shipping-service orchestrator
```

## Menjalankan Skenario

```powershell
.\scripts\run-scenario.ps1 -Scenario S1 -Approach choreography -Runs 30
.\scripts\run-scenario.ps1 -Scenario S3 -Approach orchestration -Runs 30
.\scripts\run-scenario.ps1 -Scenario S8 -Approach choreography -Runs 10
```

Skenario crash (S4/S5) otomatis — stop container di tengah transaksi
(`DELAY_MS=3000` menciptakan window crash deterministik):

```powershell
.\scripts\run-crash.ps1 -Scenario S4 -Runs 10
.\scripts\run-crash.ps1 -Scenario S5 -Runs 10
```

Hasil JSON per iterasi tersimpan di `docs/runs/<Skenario>/<approach>/`.

## Scenarios

| ID | Name | Approach | Description |
|----|------|----------|-------------|
| S1 | Baseline Normal | Both | All steps succeed |
| S2 | Last Step Failure | Both | Shipping fails after Order+Payment+Inventory succeed |
| S3 | Mid Step Failure | Both | Inventory fails after Order+Payment succeed |
| S4 | Orchestrator Crash | Orchestration | Orchestrator forcibly stopped mid-transaction |
| S5 | Kafka Down | Choreography | Kafka forcibly stopped during event publish |
| S6 | Compensating Tx Failure | Both | Compensating transaction itself fails during rollback |
| S7 | Concurrency 500 | Both | 500 concurrent transactions |
| S8 | Event Loss (partial) | Choreography | First `order.created` event silently dropped (dual-write) |
| S9 | Response Loss (in-doubt) | Orchestration | Step commits but HTTP response withheld → needless compensation |

Detail: [docs/SCENARIOS.md](docs/SCENARIOS.md)

## Metrics

- **Consistency Rate** (primary): % of transactions ending in consistent state across services
- **Compensating Tx Success Rate**: % of compensating transactions executed fully
- **Recovery Time**: from failure detection → final state consistency, computed
  from `saga_log` timestamps (proposal 3.5.3)
- **Inconsistency window**: transient inconsistency period per saga (first →
  last `saga_log` write, proposal 3.7)
- **Throughput / Latency**: comparative performance (secondary)

Agregasi + statistik deskriptif (mean/min/max/std) via `go run ./cmd/analyze`.

## Tech Stack

Go 1.21+, Kafka 3.x, Redis 7.x, PostgreSQL 15.x, Docker Compose 2.x, Prometheus + Grafana.
