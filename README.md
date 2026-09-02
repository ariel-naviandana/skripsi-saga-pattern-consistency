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
│   ├── infrastructure.yml  # Kafka, Redis, Prometheus, Grafana
│   ├── choreography.yml    # Services wired for choreography
│   └── orchestration.yml   # Services + Orchestrator
├── scripts/                # Bash helpers
│   ├── setup.sh            # Create topics + init DB
│   ├── reset.sh            # Truncate DBs, flush Redis
│   └── run-scenario.sh     # Parametric scenario runner
├── test/                   # Integration tests
├── docs/
│   ├── ARCHITECTURE.md     # Diagrams & design choices
│   ├── SCENARIOS.md        # S1-S7 definitions
│   └── REPORT.md           # Final analysis (post-implementation)
├── docker-compose.yml      # Wrapper
└── README.md
```

## Quick Start

```powershell
# Prerequisites: Docker Desktop, Go 1.21+, bash
# (Windows: jalankan scripts via Git Bash or WSL)

docker compose -f deployments/infrastructure.yml up -d   # Start infra
bash scripts/setup.sh                                     # Topics + schemas
docker compose -f deployments/choreography.yml up -d      # Start services
```

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

Detail: [docs/SCENARIOS.md](docs/SCENARIOS.md)

## Metrics

- **Consistency Rate** (primary): % of transactions ending in consistent state across services
- **Compensating Tx Success Rate**: % of compensating transactions executed fully
- **Recovery Time**: time from failure detection → final state consistency
- **Throughput / Latency**: comparative performance (secondary)

## Tech Stack

Go 1.21+, Kafka 3.x, Redis 7.x, PostgreSQL 15.x, Docker Compose 2.x, Prometheus + Grafana.
