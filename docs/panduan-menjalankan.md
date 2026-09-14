# Panduan Menjalankan Program Skripsi

> Panduan lengkap dan detail untuk menjalankan seluruh sistem skripsi dari nol
> (instalasi) sampai melihat hasil akhir, tanpa menggunakan opencode atau AI
> assistant. Semua perintah di-copy-paste langsung ke terminal (PowerShell di Windows
> atau bash di Linux/WSL).

---

## Daftar Isi

1. [Prasyarat](#1-prasyarat)
2. [Struktur Direktori](#2-struktur-direktori)
3. [Persiapan Awal (Sekali)](#3-persiapan-awal-sekali)
4. [Menjalankan Stack (Setiap Sesi)](#4-menjalankan-stack-setiap-sesi)
5. [Verifikasi Service Hidup](#5-verifikasi-service-hidup)
6. [Menjalankan Skenario](#6-menjalankan-skenario)
7. [Melihat Hasil](#7-melihat-hasil)
8. [Membersihkan (Reset/Stop)](#8-membersihkan-resetstop)
9. [Troubleshooting](#9-troubleshooting)
10. [Referensi Cepat](#10-referensi-cepat)

---

## 1. Prasyarat

Pastikan semua ini terinstall di mesin:

| Komponen | Versi Minimum | Cara Cek | Cara Install (Windows) |
|----------|----------------|----------|----------------------|
| **Docker Desktop** | 4.x | `docker --version` | Download dari https://www.docker.com/products/docker-desktop |
| **Go** | 1.21+ | `go version` | Download dari https://go.dev/dl/ |
| **Git Bash / WSL** | – | – | Sudah termasuk di Git for Windows; atau aktifkan WSL |

**Port yang harus bebas** (tidak dipakai proses lain):
- `8080`, `8081`, `8082`, `8083`, `8084` — service HTTP
- `9092`, `9094` — Kafka
- `5431`, `5432`, `5433`, `5434` — PostgreSQL (mapped ke 5432 internal)
- `6379` — Redis
- `9090` — Prometheus
- `3000` — Grafana

Cek port Windows (PowerShell):
```powershell
Get-NetTCPConnection -LocalPort 8080,8081,8082,8083,8084,9092,9094,5431,5432,5433,5434,6379,9090,3000 -State Listen -ErrorAction SilentlyContinue
```
Kalau ada baris yang muncul → port sudah dipakai, hentikan prosesnya dulu.

---

## 2. Struktur Direktori

Pastikan direktori kerja adalah root projek. Setelah `git clone` (atau dari folder projek yang sudah ada):
```
projek-skripsi/
├── cmd/                    # Binary entry points (6 service + 2 tools)
├── internal/               # Private logic
├── pkg/                    # Reusable wrappers (kafka, postgres, redis)
├── deployments/            # Docker Compose files
├── scripts/                # run-scenario, run-crash, setup, reset
├── docs/                   # REPORT, SCENARIOS, slide deck
├── docs/runs/              # Output eksperimen (git-ignored)
│   └── S{1-9}{,s}/<approach>/run-N.json
├── Dockerfile
├── docker-compose.yml
├── go.mod
└── README.md
```

---

## 3. Persiapan Awal (Sekali)

### 3.1. Clone / masuk ke direktori projek
```bash
cd /path/to/projek-skripsi
```

### 3.2. Pull dependency Go
```bash
go mod download
```

### 3.3. Build semua image Docker (pertama kali ~5-10 menit)
```powershell
# PowerShell (Windows)
docker compose build
```
atau:
```bash
# Bash / WSL
docker compose build
```

Tunggu sampai selesai. Image yang dibuat: `saga-order-service`, `saga-payment-service`, `saga-inventory-service`, `saga-shipping-service`, `saga-orchestrator`.

### 3.4. Jalankan seluruh stack
```powershell
docker compose up -d
```

Tunggu ~30 detik (pertama kali butuh inisialisasi DB + Kafka).

### 3.5. Buat Kafka topics (sekali per sesi, atau setelah re-create stack)
```bash
bash scripts/setup.sh
```

Output yang diharapkan:
```
Creating Kafka topics...
  Created: saga.order.created
  Created: saga.payment.processed
  Created: saga.payment.failed
  Created: saga.inventory.reserved
  Created: saga.inventory.failed
  Created: saga.shipping.scheduled
  Created: saga.shipping.failed
  Created: saga.closed
Setup complete.
```

### 3.6. (Sekali) Build binary workload-generator dan analyze untuk dijalankan langsung
```bash
go build -o bin/workload-generator ./cmd/workload-generator
go build -o bin/analyze ./cmd/analyze
```
Setelah ini, Anda bisa pakai `./bin/workload-generator` (atau `bin/workload-generator.exe` di Windows) langsung tanpa `go run`.

---

## 4. Menjalankan Stack (Setiap Sesi)

Setiap kali Anda ingin menjalankan ulang eksperimen (misal besok pagi):

### 4.1. Start container (kalau mati)
```powershell
docker compose up -d
```

### 4.2. Verifikasi service hidup
```powershell
$ports = 8080,8081,8082,8083,8084
foreach ($p in $ports) {
    try {
        $r = Invoke-RestMethod -Uri "http://localhost:$p/health" -TimeoutSec 5
        Write-Host "port $p -> $($r.service) $($r.status)"
    } catch {
        Write-Host "port $p -> FAIL"
    }
}
```
Output yang diharapkan:
```
port 8080 -> orchestrator ok
port 8081 -> order ok
port 8082 -> payment ok
port 8083 -> inventory ok
port 8084 -> shipping ok
```
Kalau ada `FAIL`, tunggu 5-10 detik lagi (container mungkin masih booting) lalu ulangi.

### 4.3. (Opsional) Cek Kafka topics sudah ada
```bash
docker exec saga-kafka /opt/kafka/bin/kafka-topics.sh --bootstrap-server localhost:9092 --list
```
Harus muncul 8 topik: `saga.order.created`, `saga.payment.*`, `saga.inventory.*`, `saga.shipping.*`, `saga.closed`.

---

## 5. Verifikasi Service Hidup

### 5.1. Cek semua container
```powershell
docker compose ps
```
Status harus `running` atau `healthy` untuk semua service.

### 5.2. Cek log real-time (kalau ada masalah)
```powershell
# Semua service
docker compose logs -f

# Service tertentu
docker compose logs -f saga-orchestrator
docker compose logs -f saga-order-service
```
Tekan `Ctrl+C` untuk keluar dari log.

---

## 6. Menjalankan Skenario

Ada 12 skenario total (S1-S9 + S9s + S2s + S3s). Default run count = 30 (sesuai proposal 3.6). S7 pakai 500 transaksi per run (bukan 30 run terpisah).

### 6.1. Panduan run-scenario

**Windows PowerShell:**
```powershell
.\scripts\run-scenario.ps1 -Scenario S<nama> -Approach <choreography|orchestration> -Runs 30
```

**Linux/WSL:**
```bash
bash scripts/run-scenario.sh S<nama> <choreography|orchestration> 30
```

**Argumen:**
- `-Scenario S<nama>` — salah satu dari: S1, S2, S3, S6, S7, S8, S9, S9s, S2s, S3s
- `-Approach` — `choreography` atau `orchestration` (tidak applicable untuk S4/S5 yang pakai run-crash)
- `-Runs` — jumlah iterasi (default 30; S7 selalu 500 transaksi per run)

### 6.2. Skrip otomatis vs run manual

**Skrip otomatis (S1-S3, S6-S9, S9s, S2s, S3s):** `run-scenario.ps1`/`.sh` handle reset + fault injection + workload generation + JSON output per run. Tinggal tunggu.

**Skenario crash (S4, S5):** pakai `run-crash.ps1` (hanya untuk Windows):
```powershell
.\scripts\run-crash.ps1 -Scenario S4 -Runs 30
.\scripts\run-crash.ps1 -Scenario S5 -Runs 30
```

### 6.3. Contoh urutan lengkap

**Eksekusi standar (semua skenario kecuali S4/S5):**

Buka PowerShell, jalankan:

```powershell
# S1 baseline (kedua pendekatan)
.\scripts\run-scenario.ps1 -Scenario S1 -Approach choreography -Runs 30
.\scripts\run-scenario.ps1 -Scenario S1 -Approach orchestration  -Runs 30

# S2 Shipping gagal
.\scripts\run-scenario.ps1 -Scenario S2 -Approach choreography -Runs 30
.\scripts\run-scenario.ps1 -Scenario S2 -Approach orchestration  -Runs 30

# S3 Inventory gagal
.\scripts\run-scenario.ps1 -Scenario S3 -Approach choreography -Runs 30
.\scripts\run-scenario.ps1 -Scenario S3 -Approach orchestration  -Runs 30

# S6 Kompensasi gagal
.\scripts\run-scenario.ps1 -Scenario S6 -Approach choreography -Runs 30
.\scripts\run-scenario.ps1 -Scenario S6 -Approach orchestration  -Runs 30

# S7 Konkurensi 500 transaksi (~5-10 menit/run)
.\scripts\run-scenario.ps1 -Scenario S7 -Approach choreography -Runs 30
.\scripts\run-scenario.ps1 -Scenario S7 -Approach orchestration  -Runs 30

# S8 Event loss (choreography only)
.\scripts\run-scenario.ps1 -Scenario S8 -Approach choreography -Runs 30

# S9 Response loss + S9s selective compensate (orchestration only)
.\scripts\run-scenario.ps1 -Scenario S9  -Approach orchestration -Runs 30
.\scripts\run-scenario.ps1 -Scenario S9s -Approach orchestration -Runs 30

# S2s/S3s Selective compensate counterfactual (orchestration only)
.\scripts\run-scenario.ps1 -Scenario S2s -Approach orchestration -Runs 30
.\scripts\run-scenario.ps1 -Scenario S3s -Approach orchestration -Runs 30
```

**S4/S5 (crash):**
```powershell
.\scripts\run-crash.ps1 -Scenario S4 -Runs 30
.\scripts\run-crash.ps1 -Scenario S5 -Runs 30
```

### 6.4. Output yang diharapkan per skrip

Misal `S2 orchestration`:
```
Scenario: S2 / orchestration / 30 runs
Fault: FAIL_AT_STEP=shipping ...
Starting services (approach=orchestration)...
  run 1 done -> docs\runs\S2\orchestration\run-1.json
  run 2 done -> docs\runs\S2\orchestration\run-2.json
  ...
  run 30 done -> docs\runs\S2\orchestration\run-30.json
Done. Results in docs/runs/S2/orchestration
```

Total waktu: S1-S3 ~2-3 menit per run, S6 ~1 menit, S7 ~5-10 menit per run, S8/S9/S9s/S2s/S3s ~10 detik per run. **Total semua skenario: ~1-1,5 jam.**

### 6.5. Output file yang dihasilkan

Setiap skrip membuat:
- `docs/runs/<S>/<approach>/run-1.json` sampai `run-30.json` (untuk S4/S5: `run-crash.json` format)
- Format per file JSON (contoh untuk S2 orch):
```json
{
  "approach": "orchestration",
  "count": 1,
  "committed": 0,
  "compensated": 1,
  "inconsistent": 0,
  "not_found": 0,
  "results": [
    {
      "saga_id": "abc123...",
      "outcome": "compensated",
      "latency_ms": 163,
      "recovery_time_ms": 25,
      "inconsistency_ms": 49
    }
  ],
  ...
}
```

---

## 7. Melihat Hasil

### 7.1. Generate summary agregat
```powershell
go run ./cmd/analyze
```
Atau kalau sudah di-build:
```powershell
.\bin\analyze.exe
```

**Output:** tabel di terminal + dua file JSON.

### 7.2. Output yang diharapkan (terminal)

```
Scenario | Approach | Runs | Txns | Committed | Compensated | Inconsistent | NotFound | Unrecorded | Needless | Needless% | Consistency% | CompSuccess% | Rec# | AvgRec(ms) | MinRec | MaxRec | StdRec | Inc# | AvgInc(ms) | MinInc | MaxInc | StdInc | AvgLat(ms) | MinLat | MaxLat | StdLat
-------- | -------- | ---- | ---- | --------- | ----------- | ------------ | -------- | ---------- | -------- | --------- | ------------ | ------------ | ---- | ---------- | ------ | ------ | ------ | ---- | ---------- | ------ | ------ | ------ | ---------- | ------ | ------ | ------
S1 | choreography | 30 | 30 | 30 | 0 | 0 | 0 | 0 | 0 | 0 | 100.0 | 0.0 | 0 | N/A | N/A | N/A | N/A | 30 | 42 | 20 | 124 | 23 | 427 | 82 | 1724 | 633
S1 | orchestration | 30 | 30 | 30 | 0 | 0 | 0 | 0 | 0 | 0 | 100.0 | 0.0 | 0 | N/A | N/A | N/A | N/A | 30 | 26 | 18 | 51 | 7 | 128 | 83 | 236 | 33
S2 | choreography | 30 | 30 | 0 | 30 | 0 | 0 | 0 | 0 | 0 | 100.0 | 100.0 | 30 | 22 | 12 | 42 | 7 | 30 | 49 | 26 | 114 | 18 | 1639 | 1591 | 1743 | 35
S2 | orchestration | 30 | 30 | 0 | 30 | 0 | 0 | 0 | 0 | 0 | 100.0 | 100.0 | 30 | 29 | 19 | 45 | 6 | 30 | 52 | 37 | 74 | 9 | 163 | 127 | 222 | 26
...
S2s | orchestration | 30 | 30 | 0 | 30 | 0 | 0 | 0 | 0 | 0 | 100.0 | 100.0 | 30 | 28 | 22 | 46 | 6 | 30 | 56 | 42 | 125 | 16 | 166 | 141 | 231 | 23
S3 | choreography | 30 | 30 | 0 | 30 | 0 | 0 | 0 | 0 | 0 | 100.0 | 100.0 | 30 | 13 | 8 | 20 | 3 | 30 | 27 | 16 | 42 | 6 | 1624 | 1595 | 1665 | 19
S3 | orchestration | 30 | 30 | 0 | 30 | 0 | 0 | 0 | 0 | 0 | 100.0 | 100.0 | 30 | 26 | 12 | 45 | 7 | 30 | 40 | 20 | 67 | 10 | 150 | 84 | 212 | 33
S3s | orchestration | 30 | 30 | 0 | 30 | 0 | 0 | 0 | 0 | 0 | 100.0 | 100.0 | 30 | 24 | 17 | 60 | 8 | 30 | 42 | 31 | 80 | 9 | 145 | 122 | 224 | 20
...
S9 | orchestration | 30 | 30 | 0 | 30 | 0 | 0 | 0 | 30 | 100 | 100.0 | 100.0 | 0 | N/A | N/A | N/A | N/A | 30 | 10048 | 10029 | 10075 | 14 | 10159 | 10105 | 10312 | 42
S9s | orchestration | 30 | 30 | 0 | 30 | 0 | 0 | 0 | 30 | 100 | 100.0 | 100.0 | 0 | N/A | N/A | N/A | N/A | 30 | 10035 | 10023 | 10061 | 10 | 10130 | 10102 | 10160 | 18

Summary written to docs/runs/summary.json

Mann-Whitney U (choreography vs orchestration, per-run means, alpha=0.05)
Scenario | Metric | Choreo mean(ms) | Choreo std | Orch mean(ms) | Orch std | U | p-value | Significant
-------- | ------ | --------------- | ---------- | ------------- | -------- | ------ | ------- | -----------
S1 | latency | 426.8 | 633.1 | 127.7 | 33.4 | 473.5 | 0.7337 | false
S1 | inconsistency_window | 41.5 | 22.7 | 26.3 | 7.3 | 769.5 | 0.0000 | true
S2 | latency | 1639.2 | 35.5 | 163.1 | 26.0 | 900.0 | 0.0000 | true
S2 | inconsistency_window | 49.4 | 17.9 | 52.0 | 9.0 | 346.5 | 0.1275 | false
S2 | recovery_time | 22.0 | 6.7 | 28.7 | 6.4 | 200.0 | 0.0002 | true
S3 | latency | 1623.9 | 19.5 | 149.6 | 33.5 | 900.0 | 0.0000 | true
S3 | inconsistency_window | 27.2 | 6.4 | 40.1 | 9.8 | 124.5 | 0.0000 | true
S3 | recovery_time | 12.8 | 3.0 | 26.0 | 6.5 | 35.5 | 0.0000 | true
S6 | latency | 122.9 | 22.7 | 156.2 | 30.0 | 151.5 | 0.0000 | true
S6 | inconsistency_window | 27.7 | 6.8 | 41.7 | 8.2 | 66.5 | 0.0000 | true
S7 | latency | 8476.1 | 1541.5 | 5021.1 | 681.3 | 893.0 | 0.0000 | true
S7 | inconsistency_window | 5152.0 | 1428.9 | 454.3 | 282.6 | 900.0 | 0.0000 | true

Mann-Whitney U (selective vs call-all, within orchestration, alpha=0.05)
Pair | Metric | CallAll mean | CallAll std | Selective mean | Selective std | U | p-value | Significant
----- | ------ | ------------ | ----------- | -------------- | -------------- | ------ | ------- | -----------
S2 vs S2s | recovery_time | 28.7 | 6.4 | 27.8 | 6.4 | 495.5 | 0.5046 | false
S3 vs S3s | recovery_time | 26.0 | 6.5 | 24.2 | 7.6 | 592.0 | 0.0359 | true
S9 vs S9s | latency | 10158.9 | 41.6 | 10130.1 | 18.5 | 667.5 | 0.0013 | true

Significance (choreo-vs-orch + selective-vs-call-all) written to docs/runs/significance.json
```

### 7.3. File output JSON

**`docs/runs/summary.json`** — agregat per skenario per pendekatan:
- Count, committed, compensated, inconsistent, not_found, unrecorded, needless, recovery_count, latency (avg/min/max/std), inconsistency_window (avg/min/max/std), recovery_time (avg/min/max/std), consistency%, CTSR%, needless_rate

**`docs/runs/significance.json`** — hasil Mann-Whitney U:
- Per skenario per metrik: choreo-vs-orch comparison + selective-vs-call-all comparison (S2/S2s, S3/S3s, S9/S9s)

### 7.4. Verifikasi data di DB (manual)

Misal cek data S2:
```powershell
docker exec saga-order-db psql -U order_user -d order_db -c "SELECT saga_id, status FROM orders;"
docker exec saga-payment-db psql -U payment_user -d payment_db -c "SELECT saga_id, status FROM payments;"
docker exec saga-inventory-db psql -U inventory_user -d inventory_db -c "SELECT saga_id, status FROM inventory;"
docker exec saga-shipping-db psql -U shipping_user -d shipping_db -c "SELECT saga_id, status FROM shipments;"
```

Untuk S8 (stuck saga):
```powershell
# Ambil saga_id dari run terakhir
$run = Get-Content docs\runs\S8\choreography\run-30.json | ConvertFrom-Json
$sid = $run.results[0].saga_id
Write-Host "saga: $sid"
# Cek saga_log per service
foreach ($db in @("saga-order-db","saga-payment-db","saga-inventory-db","saga-shipping-db")) {
    $user = $db -replace 'saga-','' -replace '-db','_user'
    docker exec $db psql -U $user -d ($user -replace '_user','_db') -c "SELECT step, status FROM saga_log WHERE saga_id = '$sid';"
}
```

---

## 8. Membersihkan (Reset/Stop)

### 8.1. Stop container (data masih tersimpan)
```powershell
docker compose stop
```

### 8.2. Hapus container (data masih tersimpan di volume)
```powershell
docker compose down
```

### 8.3. Hapus semua (container + data — destructive!)
```powershell
# Stop + hapus volume (semua data DB hilang)
docker compose down -v

# Atau hapus image juga
docker compose down -v --rmi all
```

### 8.4. Reset data S1-S9 saja tanpa hapus container (berguna saat iterasi)

`scripts/reset.sh` truncate semua tabel DB, flush Redis, dan reset fault counters:
```bash
bash scripts/reset.sh
```
Ini dijalankan otomatis oleh `run-scenario.ps1` di setiap iterasi, tapi bisa dipakai manual.

---

## 9. Troubleshooting

### 9.1. "port is already allocated"

**Gejala:** `docker compose up` gagal dengan `Bind for 0.0.0.0:8080 failed: port is already allocated`

**Penyebab:** port dipakai container lain atau proses host.

**Solusi:**
```powershell
# Cari container lain yang pakai port itu
docker ps -a --format "{{.Names}} | {{.Status}} | {{.Ports}}" | Select-String "8080|5432|5433|6379|9092"
# Stop container yang konflik
docker stop <nama-container>
```

Atau cek proses Windows:
```powershell
Get-NetTCPConnection -LocalPort 8080 -State Listen
# Dapatkan PID di kolom OwningProcess
Get-Process -Id <PID>
# Stop jika bukan proses yang relevan
Stop-Process -Id <PID> -Force
```

### 9.2. "Cannot connect to the Docker daemon"

**Gejala:** `docker compose up` error dengan `Cannot connect to the Docker daemon`

**Solusi:** Start Docker Desktop (pada Windows ada di system tray, klik kanan → Start).

### 9.3. Skenario S7 stuck / hang

**Gejala:** S7 orchestration run sangat lama (>5 menit per run).

**Penyebab:** Docker Desktop vpnkit port-forwarder menolak koneksi di bawah burst 500 simultan (lihat `error_detail` di JSON per-run — string `connectex: No connection could be made`).

**Solusi:**
- Tunggu — container akan pulih setelah ~10-30 detik
- Run ulang S7 — fluktuasi run-to-run tinggi (24-82 unrecorded antar run)
- Untuk sidang: dokumentasikan sebagai keterbatasan lingkungan, bukan desain sistem

### 9.4. Recovery time = 0 di tabel

**Gejala:** kolom AvgRec menampilkan "0" alih-alih angka valid.

**Status:** Sudah diperbaiki di commit #15 — tabel sekarang menampilkan "N/A" untuk skenario tanpa recovery time (S1, S6, S7, S9, S9s). Kalau masih melihat "0", pastikan sudah `go build -o bin/analyze` ulang dengan kode terbaru.

### 9.5. Kafka topics tidak ada

**Gejala:** S8/S9 error, atau `saga.order.created` not found.

**Solusi:**
```bash
bash scripts/setup.sh
# Atau manual
docker exec saga-kafka /opt/kafka/bin/kafka-topics.sh --bootstrap-server localhost:9092 --list
```

### 9.6. Database connection refused dari service

**Gejala:** service log: `failed to connect to host=...: dial tcp ...`

**Solusi:**
```powershell
# Cek DB containers hidup
docker compose ps saga-order-db saga-payment-db saga-inventory-db saga-shipping-db
# Tunggu sampai healthy (cek healthcheck)
docker compose ps --format json | ConvertFrom-Json | Where-Object { $_.Health -eq "unhealthy" }
```

### 9.7. `bash` tidak ditemukan di PowerShell

**Solusi:** Install Git Bash (`https://git-scm.com/download/win`) atau aktifkan WSL. Semua script `*.sh` butuh bash environment.

Atau jalankan script `*.sh` langsung dari Git Bash / WSL terminal:
```bash
cd /c/coding/projek-skripsi
bash scripts/reset.sh
```

---

## 10. Referensi Cepat

### 10.1. Daftar command esensial (cheatsheet)

```powershell
# Setup awal (sekali)
docker compose build
docker compose up -d
bash scripts/setup.sh

# Reset + cek
bash scripts/reset.sh
docker compose ps

# Jalankan skenario (contoh)
.\scripts\run-scenario.ps1 -Scenario S1 -Approach choreography -Runs 30
.\scripts\run-scenario.ps1 -Scenario S9s -Approach orchestration -Runs 30
.\scripts\run-crash.ps1 -Scenario S4 -Runs 10

# Lihat hasil
go run ./cmd/analyze

# Cleanup
docker compose down       # stop container
docker compose down -v    # + hapus data
```

### 10.2. File penting

| File | Isi |
|------|-----|
| `docs/REPORT.md` | Laporan lengkap (tabel, analisis, catatan metodologi) |
| `docs/SCENARIOS.md` | Definisi 12 skenario + fault config |
| `docs/panduan-menjalankan.md` | File ini |
| `docs/final-results-prompt.md` | Self-contained prompt untuk review independen |
| `docs/ai-review-prompt.md` | Self-contained prompt untuk review awal |
| `README.md` | Quick start + ringkasan skenario |
| `cmd/analyze/main.go` | CLI summary + MWU |
| `cmd/workload-generator/main.go` | CLI generate transaksi |
| `cmd/orchestrator/main.go` | Central coordinator + selective compensate |
| `internal/orchestration/orchestrator.go` | Core orchestrator logic |
| `internal/common/faultinject.go` | Fault injection middleware |
| `scripts/run-scenario.ps1` / `.sh` | Skenario runner |
| `scripts/run-crash.ps1` | S4/S5 runner |
| `scripts/setup.sh` | Kafka topics creator |
| `scripts/reset.sh` | DB truncate + Redis flush + fault reset |

### 10.3. Daftar 12 skenario

| Skenario | Approach | Tujuan | Fault |
|----------|-----------|--------|-------|
| S1 | choreo + orch | Baseline normal | – |
| S2 | choreo + orch | Shipping gagal | FAIL_AT_STEP=shipping, ATTEMPT=1 |
| S3 | choreo + orch | Inventory gagal | FAIL_AT_STEP=inventory, ATTEMPT=1 |
| S4 | orch only | Orchestrator crash | DELAY_MS=3000 + docker stop orchestrator |
| S5 | choreo only | Kafka down | DELAY_MS=3000 + docker stop kafka |
| S6 | choreo + orch | Kompensasi gagal | FAIL_AT_STEP=inventory, FAIL_ON_COMPENSATE=true |
| S7 | choreo + orch | 500 tx/konkuren | – (count=500) |
| S8 | choreo only | Event loss | DROP_EVENT=saga.order.created |
| S9 | orch only | Response loss | DROP_RESPONSE_AT_STEP=inventory |
| S9s | orch only | S9 + selective compensate | + SELECTIVE_COMPENSATE=true |
| S2s | orch only | S2 + selective | + SELECTIVE_COMPENSATE=true |
| S3s | orch only | S3 + selective | + SELECTIVE_COMPENSATE=true |

### 10.4. Total estimasi waktu

| Fase | Durasi |
|-------|--------|
| Setup awal (build + first start + setup.sh) | 10-15 menit |
| S1-S3, S6, S8 (×2 approach) | ~25 menit |
| S7 (×2 approach, 500 tx/run) | ~25 menit |
| S9, S9s, S2s, S3s | ~10 menit |
| S4, S5 (crash, 30 runs each) | ~30 menit |
| Regenerate summary + analyze | <1 menit |
| **TOTAL** | **~105-120 menit** (~2 jam) |

### 10.5. Commit history (untuk konteks git)

15 commits total, dari setup awal sampai perbaikan metodologis terakhir:
- `#1-#6`: setup, eksperimen S1-S7, perbaikan recovery time + S7
- `#7`: Mann-Whitney U + S8 event loss
- `#8`: S9 response loss
- `#9`: dokumentasi caveats (call-all confounding, recovery time S9)
- `#10`: re-run semua dengan kode final
- `#11`: S9s selective compensate counterfactual
- `#12`: S2s/S3s + penjelasan-8-tahap.md
- `#13`: update Caveat & Keterbatasan + Tahap 7 table
- `#14`: n=30 untuk S8/S9/S9s/S2s/S3s + needless_compensation_rate + error logging + stepCommitted fallback log
- `#15`: MWU selective-vs-call-all (n=30) + guard display N/A + S7 re-run + error_detail

---

## Lampiran: Cara Kerja Singkat (TL;DR)

```powershell
# 1. Setup awal (sekali)
docker compose build && docker compose up -d && bash scripts/setup.sh

# 2. Jalankan semua skenario (1.5 jam)
foreach ($s in @("S1","S2","S3","S6","S7")) {
    foreach ($a in @("choreography","orchestration")) {
        .\scripts\run-scenario.ps1 -Scenario $s -Approach $a -Runs 30
    }
}
.\scripts\run-scenario.ps1 -Scenario S8 -Approach choreography -Runs 30
foreach ($s in @("S9","S9s","S2s","S3s")) {
    .\scripts\run-scenario.ps1 -Scenario $s -Approach orchestration -Runs 30
}
.\scripts\run-crash.ps1 -Scenario S4 -Runs 30
.\scripts\run-crash.ps1 -Scenario S5 -Runs 30

# 3. Lihat hasil
go run ./cmd/analyze

# 4. Cleanup (opsional, akhir sesi)
docker compose down
```

**Itu saja. Hasil lengkap ada di `docs/REPORT.md` dan `docs/final-results-prompt.md`.**
