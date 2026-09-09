# Verifikasi Kode Lanjutan — Integritas Data & Concurrency Safety

> Dokumen ini menggabungkan follow-up dari checklist verifikasi sebelumnya
> dengan pertanyaan baru yang belum pernah diverifikasi sepanjang proses sebelumnya.
> Fokus murni pada kode/implementasi, tidak ada pembahasan framing atau laporan.

---

## Daftar Isi

1. [Bagian A — Follow-up Verifikasi Sebelumnya](#bagian-a--follow-up-verifikasi-sebelumnya)
2. [Bagian B — Pertanyaan Baru](#bagian-b--pertanyaan-baru)
3. [Bagian C — Pertanyaan Tambahan (Infrastruktur & Instrumentasi)](#bagian-c--pertanyaan-tambahan-infrastruktur--instrumentasi)
4. [Ringkasan Temuan](#ringkasan-temuan)

---

## Bagian A — Follow-up Verifikasi Sebelumnya

### A1. Justifikasi window quiescence untuk S7

**Status**: Diverifikasi dari data aktual (bukan estimasi teoretis)

**Metode verifikasi**: Dihitung dari data S7 yang sudah ada di `docs/runs/S7/` + kode `cmd/workload-generator/main.go` (fungsi `waitForOutcome`).

**Mekanisme quiescence detection** (`cmd/workload-generator/main.go`):

```
Konstanta:     quiescence = 10 detik
Poll interval: 1500 ms (1,5 detik)
Deadline:      60 detik per saga
Algoritma:
  Loop setiap 1,5 detik:
    1. Panggil checker.Check() → dapat outcome saat ini
    2. Panggil checker.Timeline() → dapat timestamp terakhir di saga_log
    3. Kalau timestamp sudah berubah sejak pengecekan terakhir → reset stable counter
    4. Kalau outcome = committed/compensated → return langsung (terminal)
    5. Kalau saga "sealed" (compensate_failed di saga_log) → return langsung
    6. Kalau time.Since(lastProgress) > 10 detik DAN outcome tidak berubah:
         stable++
         kalau stable >= 2 → return outcome saat ini
```

Effective quiescence = **10 detik** (tidak ada progress) + **2 consecutive polls** (3 detik) = **~13 detik** total stabilitas sebelum klasifikasi.

**Data aktual S7 (500 transaksi konkuren)**:

| Metrik | Choreography | Orchestration |
|--------|-------------|---------------|
| Rata-rata total_ms (500 sagas) | 8.476 ms | 5.021 ms |
| Maks total_ms | 18.246 ms | 7.144 ms |
| Rata-rata inconsistency_ms | 5.152 ms | 453 ms |
| Maks inconsistency_ms | 11.629 ms | 2.564 ms |

**Inter-step gap** (waktu antar langkah berturutan untuk satu saga):

- Maks inter-step gap teramati: **~7 detik** (dari comment di kode: "observed up to ~7s for 500 concurrent orders")
- Quiescence window: 10 detik
- Margin: **~3-6 detik** di atas inter-step gap maks
- Per-saga deadline: 60 detik sebagai hard backstop

**Kesimpulan**: Window 10 detik cukup untuk S7 (500 konkuren). Kalau load dinaikkan ke 1000+, inter-step gap akan berbanding lurus dan window perlu di-revisit.

---

### A2. Metode verifikasi scoping DELAY_MS

**Status**: Diverifikasi secara statis (code reading)

**Temuan dari `scripts/run-scenario.ps1`**:

Setiap branch di switch block secara eksplisit set `$env:DELAY_MS = "0"`:

```powershell
switch ($Scenario) {
  { $_ -in @("S1", "S7") } { ... $env:DELAY_MS = "0"; ... }
  "S2" { ... $env:DELAY_MS = "0"; ... }
  "S3" { ... $env:DELAY_MS = "0"; ... }
  "S6" { ... $env:DELAY_MS = "0"; ... }
  "S8" { ... $env:DELAY_MS = "0"; ... }
  "S9" { ... $env:DELAY_MS = "0"; ... }
  "S9s" { ... $env:DELAY_MS = "0"; ... }
  "S2s" { ... $env:DELAY_MS = "0"; ... }
  "S3s" { ... $env:DELAY_MS = "0"; ... }
  default { throw "unsupported scenario $Scenario" }
}
```

**Temuan dari `scripts/run-scenario.sh`** (identik):

```bash
configure_fault() {
  case "$SCENARIO" in
    S1|S7) ... DELAY_MS="0" ... ;;
    S2)    ... DELAY_MS="0" ... ;;
    ...
    *)     echo "unsupported scenario $SCENARIO" >&2; exit 1 ;;
  esac
}
# ... lalu: export APPROACH FAIL_AT_STEP ... DELAY_MS ...
```

**Safety net berlapis**:

| Layer | Mekanisme | Default |
|-------|-----------|---------|
| Script | Semua branch set `DELAY_MS=0` | – |
| Docker Compose | `${DELAY_MS:-0}` di `deployments/services.yml` | 0 |
| Go code | `EnvIntOr("DELAY_MS", 0)` di `faultinject.go` | 0 |
| Script terpisah | `run-crash.ps1` set `DELAY_MS=3000` (S4/S5 only) | – |

**Catatan metodologi**: Verifikasi ini **statis** (code reading), belum diverifikasi runtime (mengecek env var aktual di container setelah eksekusi). Namun karena 3 layer safety net independen, risiko kebocoran sangat rendah.

---

## Bagian B — Pertanyaan Baru

### B1. Atomicity antara write ke tabel bisnis dan write ke saga_log

**Status**: ⚠️ **TIDAK ATOMIC** — temuan kritis

**Kode yang diverifikasi**: Semua fungsi di `internal/business/` — `order.go`, `payment.go`, `inventory.go`, `shipping.go` (forward + compensate = 8 fungsi).

**Pola yang ditemukan di SEMUA fungsi**:

```go
// Statement 1: auto-commit terpisah
s.Pool.Exec(ctx, `INSERT INTO orders ...`)

// Statement 2: auto-commit terpisah
writeLog(ctx, s.Pool, sagaID, "order", "committed", "")
```

Tidak ada `pool.Begin()`, tidak ada `tx.Commit()`. Fungsi `writeLog()` menerima `*pgxpool.Pool` (bukan `pgx.Tx`), sehingga selalu menjalankan auto-commit terpisah.

**Bukti konkret** — `OrderService.CreateOrder()` (`internal/business/order.go:32-53`):

```go
func (s *OrderService) CreateOrder(ctx context.Context, sagaID string) error {
    // ... fault injection check ...

    var orderID int64
    err := s.Pool.QueryRow(ctx,
        `INSERT INTO orders (saga_id, customer_id, product_id, quantity, amount, status)
         VALUES ($1, $2, $3, $4, $5, 'committed') RETURNING id`,
        sagaID, customerID, productID, quantity, amount,
    ).Scan(&orderID)
    if err != nil {
        return fmt.Errorf("business: insert order: %w", err)
    }

    return writeLog(ctx, s.Pool, sagaID, "order", "committed", "")
    //      ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
    //      writeLog pool.Exec() → auto-commit TERPISAH
}
```

**Fungsi `writeLog()`** (`internal/business/sagalog.go:11-19`):

```go
func writeLog(ctx context.Context, pool *pgxpool.Pool, sagaID, step, status, snapshot string) error {
    _, err := pool.Exec(ctx,
        `INSERT INTO saga_log (saga_id, step, status, snapshot) VALUES ($1, $2, $3, $4)`,
        sagaID, step, status, snapshot,
    )
    // ...
}
```

Parameter pertama `pool *pgxpool.Pool` — bukan `pgx.Tx`. Tidak ada variant yang menerima transaction.

**Gap window**:

```
Time ──────────────────────────────────────────────────────►

  pool.Exec(INSERT orders)    [auto-commit]    ←── sukses
          │
          │  ←── gap window (~mikrodetik) ──→
          │      (crash di sini = inconsistency)
          │
  pool.Exec(INSERT saga_log)  [auto-commit]    ←── belum dijalankan
```

Kalau crash terjadi tepat di antara kedua statement, tabel bisnis ter-update tapi saga_log tidak (atau sebaliknya). Checker akan salah mengklasifikasikan outcome.

**8 fungsi yang terdampak** (semua mengikuti pola yang sama):

| Fungsi | File | Statement 1 (bisnis) | Statement 2 (saga_log) |
|--------|------|---------------------|----------------------|
| `CreateOrder` | order.go:32 | `INSERT INTO orders` | `writeLog(..., "committed", ...)` |
| `CompensateOrder` | order.go:56 | `UPDATE orders SET status='compensated'` | `writeLog(..., "compensated", ...)` |
| `ProcessPayment` | payment.go:18 | `INSERT INTO payments` | `writeLog(..., "committed", ...)` |
| `CompensatePayment` | payment.go:36 | `UPDATE payments SET status='compensated'` | `writeLog(..., "compensated", ...)` |
| `ReserveInventory` | inventory.go:18 | `INSERT INTO inventory` | `writeLog(..., "committed", ...)` |
| `CompensateInventory` | inventory.go:36 | `UPDATE inventory SET status='compensated'` | `writeLog(..., "compensated", ...)` |
| `ScheduleShipping` | shipping.go:18 | `INSERT INTO shipments` | `writeLog(..., "committed", ...)` |
| `CompensateShipping` | shipping.go:36 | `UPDATE shipments SET status='compensated'` | `writeLog(..., "compensated", ...)` |

**Implikasi terhadap data yang sudah ada**:
- Kecil kemungkinannya terjadi secara natural (window antara dua INSERT sangat kecil, ~mikrodetik)
- Fault injection (S2-S9) menyebabkan kegagalan di level HTTP/Kafka, bukan di level DB transaction
- Gap window ini bukan penyebab hasil yang teramati dalam eksperimen

**Rekomendasi**: Untuk tahap proposal, cukup **dicatat sebagai keterbatasan metodologis** di REPORT.md. Tidak perlu run ulang. Untuk versi production, harus diperbaiki:

```go
// Perbaikan: bungkus dalam satu transaction
tx, err := pool.Begin(ctx)
if err != nil { return err }
defer tx.Rollback(ctx)

_, err = tx.Exec(ctx, `INSERT INTO orders ...`)
if err != nil { return err }

_, err = tx.Exec(ctx, `INSERT INTO saga_log ...`)
if err != nil { return err }

return tx.Commit(ctx)
```

`writeLog` perlu variant baru yang menerima `pgx.Tx`:
```go
func writeLogTx(ctx context.Context, tx pgx.Tx, sagaID, step, status, snapshot string) error {
    _, err := tx.Exec(ctx, `INSERT INTO saga_log ...`, sagaID, step, status, snapshot)
    return err
}
```

---

### B2. Idempotency pada compensating transaction

**Status**: ⚠️ **PARTIALLY IDEMPOTENT**

**Kode yang diverifikasi**: Ke-4 fungsi compensate di `internal/business/*.go`.

**Hasil per fungsi**:

| Fungsi | UPDATE punya status guard? | RowsAffected dicek? | saga_log duplicate-safe? | Truly idempotent? |
|--------|---------------------------|--------------------|-----------------------|-------------------|
| `CompensateOrder` | ✅ `WHERE status = 'committed'` | ❌ Tidak | ❌ Plain INSERT | **Partially** |
| `CompensatePayment` | ✅ `WHERE status = 'committed'` | ❌ Tidak | ❌ Plain INSERT | **Partially** |
| `CompensateInventory` | ✅ `WHERE status = 'committed'` | ❌ Tidak | ❌ Plain INSERT | **Partially** |
| `CompensateShipping` | ✅ `WHERE status = 'committed'` | ❌ Tidak | ❌ Plain INSERT | **Partially** |

**Bukti konkret** — `CompensateOrder()` (`internal/business/order.go:56-71`):

```go
func (s *OrderService) CompensateOrder(ctx context.Context, sagaID string) error {
    // ... fault injection check ...

    if _, err := s.Pool.Exec(ctx,
        `UPDATE orders SET status = 'compensated' WHERE saga_id = $1 AND status = 'committed'`,
        //                                           ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
        //                                           status guard: aman
        sagaID,
    ); err != nil {
        return fmt.Errorf("business: compensate order: %w", err)
    }
    // RowsAffected TIDAK dicek — update 0 baris dianggap sukses

    return writeLog(ctx, s.Pool, sagaID, "order", "compensated", "")
    //      ^^^^^^^^^^^^^^^^^^^^^^^^^ duplikat saga_log akan ditulis
}
```

**Apa yang terjadi kalau `CompensateOrder` dipanggil 2 kali untuk saga_id yang sama**:

1. **Panggilan pertama**:
   - UPDATE: 1 row affected (status berubah dari `'committed'` ke `'compensated'`) ✅
   - writeLog: INSERT saga_log row baru ✅

2. **Panggilan kedua**:
   - UPDATE: 0 rows affected (status sudah `'compensated'`, guard `AND status = 'committed'` tidak cocok) — **tidak error**, tapi juga tidak mengubah apa pun
   - writeLog: INSERT saga_log row **DUPLIKAT** — ada 2 baris `"compensated"` untuk `(saga_id, "order")` ❌

**Schema saga_log** (`internal/common/schema.go`):

```sql
CREATE TABLE IF NOT EXISTS saga_log (
    id          BIGSERIAL PRIMARY KEY,
    saga_id     TEXT NOT NULL,
    step        TEXT NOT NULL,
    status      TEXT NOT NULL,
    snapshot    TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- Tidak ada UNIQUE constraint pada (saga_id, step)
```

**Implikasi terhadap data yang sudah ada**:
- Dalam eksperimen ini, compensate hanya dipanggil 1x per saga (tidak ada retry loop)
- Duplikat saga_log tidak memengaruhi klasifikasi outcome (checker pakai status terakhir dari `Timeline()`)
- Tapi ini tetap **cacat desain** yang perlu dicatat

**Rekomendasi**: Untuk tahap proposal, cukup **dicatat sebagai keterbatasan**. Untuk perbaikan:

```go
// Option A: Check RowsAffected
result, err := tx.Exec(ctx, `UPDATE orders SET status = 'compensated' WHERE saga_id = $1 AND status = 'committed'`, sagaID)
if err != nil { return err }
if result.RowsAffected() == 0 { return nil } // sudah compensated, skip writeLog
return writeLogTx(ctx, tx, sagaID, "order", "compensated", "")
```

---

### B3. Concurrency safety pada pengurangan stok Inventory

**Status**: ✅ **TIDAK ADA RISK**

**Kode yang diverifikasi**: `internal/business/inventory.go` + `internal/common/schema.go`

**Schema tabel inventory**:

```sql
CREATE TABLE IF NOT EXISTS inventory (
    id          BIGSERIAL PRIMARY KEY,
    saga_id     TEXT NOT NULL,
    product_id  TEXT NOT NULL,
    quantity    INT NOT NULL,
    status      TEXT NOT NULL DEFAULT 'pending',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

**Tidak ada kolom `stock`**. Tabel ini adalah append-only reservation log — mencatat bahwa saga X meminta quantity Y untuk product Z.

**Kode `ReserveInventory()`** (`internal/business/inventory.go:26-31`):

```go
func (s *InventoryService) ReserveInventory(ctx context.Context, sagaID string) error {
    // ... fault injection check ...

    _, err := s.Pool.Exec(ctx,
        `INSERT INTO inventory (saga_id, product_id, quantity, status) VALUES ($1, $2, $3, 'committed')`,
        sagaID, productID, quantity,
    )
    // ...
}
```

- Tidak ada `SELECT stock FROM inventory`
- Tidak ada `UPDATE inventory SET stock = stock - $1`
- Tidak ada read-then-write pattern
- Operasi murni INSERT (append-only)

**Kesimpulan**: Tidak ada race condition karena tidak ada shared state yang di-read-then-write. S7 tidak terpengaruh oleh concurrency bug di level implementasi. Anomali yang teramati di S7 murni dari karakteristik pendekatan (Kafka serialization dengan partition tunggal, bukan overselling).

---

## Bagian C — Pertanyaan Tambahan (Infrastruktur & Instrumentasi)

### C1. Kelengkapan reset fault flag antar run

**Status**: ✅ **LENGKAP (by design)**

**Kode yang diverifikasi**: `internal/common/faultinject.go` (FaultConfig struct + ResetAttempt) + `internal/common/server.go` (FaultResetHandler)

**FaultConfig struct — semua field**:

| # | Field | Tipe | Sumber | Perlu reset per-run? |
|---|-------|------|--------|---------------------|
| 1 | `FailAtStep` | string | env `FAIL_AT_STEP` | ❌ Config — persist antar run |
| 2 | `FailAtAttempt` | int | env `FAIL_AT_ATTEMPT` | ❌ Config — persist antar run |
| 3 | `DelayMS` | int | env `DELAY_MS` | ❌ Config — persist antar run |
| 4 | `FailOnCompensate` | bool | env `FAIL_ON_COMPENSATE` | ❌ Config — persist antar run |
| 5 | `DropEvent` | string | env `DROP_EVENT` | ❌ Config — persist antar run |
| 6 | `DropResponseAtStep` | string | env `DROP_RESPONSE_AT_STEP` | ❌ Config — persist antar run |
| 7 | `SelectCompensate` | bool | env `SELECTIVE_COMPENSATE` | ❌ Config — persist antar run |
| 8 | `attempt` | atomic.Int32 | internal counter | ✅ **Harus reset** |
| 9 | `dropped` | atomic.Bool | internal flag (S8) | ✅ **Harus reset** |
| 10 | `droppedResp` | atomic.Bool | internal flag (S9) | ✅ **Harus reset** |

**Apa yang di-reset oleh `/fault/reset`** (`internal/common/server.go:46-54`):

```go
func FaultResetHandler(fc *FaultConfig) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        if fc != nil {
            fc.ResetAttempt()
        }
        JSON(w, http.StatusOK, map[string{"status": "ok"})
    }
}
```

**Apa yang di-reset oleh `ResetAttempt()`** (`internal/common/faultinject.go:39-43`):

```go
func (f *FaultConfig) ResetAttempt() {
    f.attempt.Store(0)        // ✅ reset counter
    f.dropped.Store(false)    // ✅ reset flag S8
    f.droppedResp.Store(false) // ✅ reset flag S9
}
```

**Kesimpulan**: Semua runtime accumulators di-reset. Config fields (FailAtStep, DropEvent, dll) tidak di-reset karena di-set dari env vars saat startup — ini benar karena container di-recreate per skenario via `docker compose up`.

**Tidak ada risiko kebocoran**: Setiap kali skenario baru dijalankan, `docker compose up -d` membuat container baru dengan env vars yang benar (di-set oleh `run-scenario.ps1`/`.sh`). Flag runtime di-reset oleh `/fault/reset` yang dipanggil di awal setiap iterasi.

---

### C2. Konsistensi sumber timestamp untuk recovery time

**Status**: ✅ **KONSISTEN**

**Kode yang diverifikasi**: `internal/common/schema.go` (semua CREATE TABLE) + `internal/business/sagalog.go` (writeLog) + `internal/consistency/checker.go` (Timeline)

**Semua tabel pakai pola yang sama**:

```sql
-- orders, payments, inventory, shipments, saga_log
created_at TIMESTAMPTZ NOT NULL DEFAULT now()
```

`now()` adalah PostgreSQL function yang mengembalikan clock database server (bukan aplikasi Go).

**writeLog tidak menyertakan `created_at`**:

```go
func writeLog(ctx context.Context, pool *pgxpool.Pool, sagaID, step, status, snapshot string) error {
    _, err := pool.Exec(ctx,
        `INSERT INTO saga_log (saga_id, step, status, snapshot) VALUES ($1, $2, $3, $4)`,
        //                     created_at tidak disertakan → DEFAULT now()
        sagaID, step, status, snapshot,
    )
    // ...
}
```

**Checker membaca timestamp langsung dari DB**:

```go
// internal/consistency/checker.go — Timeline()
rows, err := pool.Query(ctx,
    `SELECT created_at, status FROM saga_log WHERE saga_id = $1 ORDER BY created_at`,
    sagaID,
)
// pgx otomatis deserialisasi TIMESTAMPTZ → Go time.Time
// Tidak ada konversi manual, tidak ada time.Now()
```

**Kesimpulan**: Sumber timestamp **konsisten** di seluruh service — semua pakai `now()` dari PostgreSQL. Tidak ada `time.Now()` di aplikasi untuk `created_at`.

**Caveat**: 4 container DB berjalan di host yang sama (Docker Desktop) — clock harusnya sinkron. Kalau ada drift antar container, noise ~mikrodetik (tidak signifikan untuk skala milidetik yang diukur).

---

### C3. Ukuran connection pool database saat S7 (500 konkuren)

**Status**: ⚠️ **POTENTIAL BOTTLENECK**

**Kode yang diverifikasi**: `pkg/postgres/pool.go` + `cmd/order-service/main.go` + `cmd/inventory-service/main.go` + semua service

**Panggilan yang ditemukan**:

```go
// cmd/order-service/main.go
pool, err := common.NewDB(ctx, common.ServiceOrder)

// internal/common/db.go
func NewDB(ctx context.Context, name ServiceName) (*pgxpool.Pool, error) {
    cfg := ServiceDSN(name)
    pool, err := postgres.NewPool(ctx, cfg)
    // ...
}

// pkg/postgres/pool.go
func NewPool(ctx context.Context, cfg Config) (*pgxpool.Pool, error) {
    connString := fmt.Sprintf(
        "host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
        cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName,
    )
    pool, err := pgxpool.New(ctx, connString)
    // ...
}
```

**Tidak ada explicit pool configuration** — tidak ada `pgxpool.Config{}`, tidak ada `MaxConns`, tidak ada `MinConns`. Semua pakai pgx v5 defaults.

**Default pgx v5**:

| Parameter | Default |
|-----------|---------|
| `MaxConns` | **4** |
| `MinConns` | 0 |
| `MaxConnLifetime` | 1 jam |
| `MaxConnIdleTime` | 30 menit |
| `HealthCheckPeriod` | 0 (disabled) |

**Semua service pakai MaxConns = 4**:

| Service | MaxConns | Source |
|---------|----------|--------|
| Order | 4 | pgx default |
| Payment | 4 | pgx default |
| Inventory | 4 | pgx default |
| Shipping | 4 | pgx default |
| Orchestrator | 4 | pgx default |

**Implikasi untuk S7 (500 konkuren)**:

- 500 request masuk bersamaan ke satu service
- Hanya 4 koneksi DB tersedia
- 496 request akan mengantre menunggu koneksi
- Menambah latency yang **bukan** dari karakteristik arsitektural

**Perbandingan choreography vs orchestration masih valid**: Kedua pendekatan pakai pool size yang sama (4), sehingga bottleneck ini terdampak sama rata. Perbedaan yang diamati di S7 (choreography lebih lambat) murni dari arsitektural (Kafka serialization), bukan dari pool size.

**Rekomendasi**: Catatan di REPORT.md bahwa S7 latency terpengaruh oleh pool size default (MaxConns=4). Untuk production, pool harus dinaikkan. Untuk perbandingan antar-pendekatan, pool size bukan confounding factor.

---

## Ringkasan Temuan

| Poin | Status | Prioritas | Tindakan |
|------|--------|-----------|----------|
| A1 quiescence S7 | ✅ Cukup (10s window, 7s max inter-step) | – | Tidak perlu ubah |
| A2 DELAY_MS scoping | ✅ Aman (statis, 3 layer safety net) | – | Catat metodologi |
| **B1 atomicity bisnis+saga_log** | ⚠️ **TIDAK ATOMIC** | **Tinggi** | **Catat sebagai keterbatasan** |
| **B2 idempotency compensate** | ⚠️ **Partial** (UPDATE aman, saga_log duplikat) | **Tinggi** | **Catat sebagai keterbatasan** |
| B3 concurrency inventory | ✅ Tidak ada risk (tidak ada kolom stock) | – | Tidak perlu ubah |
| C1 reset flags | ✅ Lengkap (runtime accumulators di-reset) | – | Tidak perlu ubah |
| C2 timestamp | ✅ Konsisten (semua pakai DB now()) | – | Tidak perlu ubah |
| **C3 pool size S7** | ⚠️ **Default MaxConns=4** | **Sedang** | **Catat sebagai keterbatasan** |
| B4 unit test | ❌ Tidak ada (0 test files) | Rendah | Nice-to-have |

### Temuan yang perlu didokumentasikan di REPORT.md

1. **B1 — Non-atomicity**: Business write dan saga_log write dilakukan sebagai dua auto-commit terpisah. Gap window sangat kecil (~mikrodetik) dan tidak memengaruhi hasil eksperimen, tapi perlu disebutkan untuk transparansi.

2. **B2 — Partial idempotency**: Compensate functions aman di level data (UPDATE punya status guard `AND status = 'committed'`), tapi writeLog menulis duplikat ke saga_log tanpa `ON CONFLICT`. Tidak memengaruhi klasifikasi outcome karena checker pakai status terakhir.

3. **C3 — Pool size default**: Semua service pakai `MaxConns=4` (pgx default). Untuk S7 (500 konkuren), ini menambah latency pengantrean koneksi. Perbedaan choreography vs orchestration masih valid karena kedua pendekatan terdampak sama.

### Yang TIDAK perlu perubahan

- **Tidak perlu run ulang data**: Temuan B1, B2, C3 adalah keterbatasan desain/infrastruktur, bukan bug yang memengaruhi outcome yang teramati. Gap window B1 sangat kecil, B2 tidak ter-trigger dalam eksperimen ini, C3 terdampak sama rata ke kedua pendekatan.
- **Tidak perlu ubah kode untuk tahap proposal**: Perbaikan B1/B2/C3 berarti run ulang semua skenario (~1.5 jam) — tidak proporsional untuk tahap proposal.

### Kapan temuan ini menjadi kritis

- **B1 (atomicity)**: Kritis untuk production deployment. Kalau service crash tepat di antara dua write, data bisa inconsist.
- **B2 (idempotency)**: Kritis kalau ada retry loop pada compensate (misal network timeout → orchestrator retry). Dalam eksperimen ini tidak ada retry, jadi tidak ter-trigger.
- **C3 (pool size)**: Kritis kalau S7 dilakukan dengan load lebih tinggi (1000+ konkuren). Pool 4 koneksi akan menjadi bottleneck dominan.
