# Skenario Pengujian (S1–S7)

Dokumen ini mendefinisikan tujuh skenario pengujian, mekanisme fault injection,
dan format hasilnya. Detail konseptual dijelaskan pada proposal (BAB 3.4).

## Fault Injection Middleware

Fault dikonfigurasi **non-intrusive** lewat environment variable container
(tidak ada perubahan kode antar skenario):

| Variabel | Nilai | Arti |
|----------|-------|------|
| `FAIL_AT_STEP` | `order`/`payment`/`inventory`/`shipping` | Langkah mana yang gagal |
| `FAIL_AT_ATTEMPT` | int | Transaksi ke-berapa yang gagal (counter di-reset antar run) |
| `DELAY_MS` | int | Penundaan respons (simulasi timeout / jaringan lambat) |
| `FAIL_ON_COMPENSATE` | `true`/`false` | Gagalkan compensating transaction (S6) |

Saat forward step gagal, service menulis `saga_log` status `failed` dan (di
choreography) memublikasikan event kegagalan untuk memicu kompensasi berantai.
Saat kompensasi gagal, ditulis `compensate_failed`.

## Definisi Skenario

| ID | Nama | Fault Config | Target Outcome |
|----|------|--------------|----------------|
| S1 | Baseline Normal | tidak ada | `committed` |
| S2 | Kegagalan Langkah Akhir (Shipping) | `FAIL_AT_STEP=shipping`, `FAIL_AT_ATTEMPT=1` | `compensated` |
| S3 | Kegagalan Langkah Tengah (Inventory) | `FAIL_AT_STEP=inventory`, `FAIL_AT_ATTEMPT=1` | `compensated` |
| S4 | Orchestrator Crash | hentikan container `saga-orchestrator` di tengah saga | `inconsistent` (partial commit) |
| S5 | Kafka Down | hentikan container `saga-kafka` saat event dipublikasikan | `inconsistent` (partial commit) |
| S6 | Kegagalan Compensating Tx | `FAIL_AT_STEP=inventory`, `FAIL_AT_ATTEMPT=1`, `FAIL_ON_COMPENSATE=true` | `inconsistent` |
| S7 | Konkurensi 500 Transaksi | tidak ada, 500 request bersamaan | `committed` (semua) |

Catatan: S4 hanya untuk orchestration, S5 hanya untuk choreography (bergantung
komponen yang hanya ada di pendekatan tersebut).

## Menjalankan Skenario

PowerShell (dianjurkan di Windows karena `bash` di mesin ini adalah WSL tanpa Go):

```powershell
# 30 iterasi, hasil JSON di docs/runs/<Skenario>/<approach>/
.\scripts\run-scenario.ps1 -Scenario S1 -Approach choreography -Runs 30
.\scripts\run-scenario.ps1 -Scenario S3 -Approach orchestration -Runs 30
```

Lingkungan bash+Go (Linux / CI):

```bash
bash scripts/run-scenario.sh S1 choreography 30
```

Skenario S4 dan S5 tidak otomatis di `run-scenario` (butuh penghentian container
bertiming). Dijalankan manual: kirim transaksi, hentikan container pada saat yang
ditentukan, lalu hitung konsistensi dari snapshot DB. Contoh:

```bash
# S4 - hentikan orchestrator ± setelah order+payment commit
curl -s -X POST http://localhost:8080/saga -H 'Content-Type: application/json' -d '{...}'
docker stop saga-orchestrator
```

## Skenario S4 dan S5 (otomatis)

Dijalankan otomatis via `scripts/run-crash.ps1` (10+ iterasi):

```powershell
# S4 - Orchestrator Crash (orchestration): stop orchestrator setelah order+payment
#      commit, saat langkah inventory sedang berjalan (DELAY_MS=3000 menciptakan
#      window crash deterministik, proposal 3.4.1)
.\scripts\run-crash.ps1 -Scenario S4 -Runs 10

# S5 - Kafka Down (choreography): stop Kafka setelah order commit, rantai event
#      terputus sebelum payment/inventory/shipping
.\scripts\run-crash.ps1 -Scenario S5 -Runs 10
```

Hasil verifikasi (10 run): S4 dan S5 keduanya **10/10 inconsistent** (commit
parsial) — lihat `docs/runs/S4/orchestration/` dan `docs/runs/S5/choreography/`.

## Reset antar Run

`scripts/reset.sh` mengosongkan semua tabel bisnis + `saga_log` di 4 database,
`FLUSHALL` Redis, dan me-reset counter fault (`POST /fault/reset`).

## Format Hasil (per run)

```json
{
  "approach": "choreography",
  "count": 1,
  "committed": 1,
  "compensated": 0,
  "inconsistent": 0,
  "not_found": 0,
  "total_ms": 557,
  "throughput_tps": 1.79,
  "avg_latency_ms": 557,
  "results": [
    { "saga_id": "…", "outcome": "committed", "latency_ms": 557 }
  ]
}
```

File mentah disimpan di `docs/runs/` (git-ignored).

## Ringkasan Hasil Verifikasi (implementasi)

Data lengkap: [docs/REPORT.md](REPORT.md) dan `docs/runs/summary.json`.

| Skenario | Choreography | Orchestration |
|----------|--------------|---------------|
| S1 | committed (100%) | committed (100%) |
| S2 | compensated (100%) | compensated (100%) |
| S3 | compensated (100%) | compensated (100%) |
| S6 | inconsistent (100%) | inconsistent (100%) |
| S7 (500×30) | committed (100%, 15000/15000) | committed (100%, 14998/15000; 2 request unrecorded) |
| S4 | — | inconsistent (partial, manual) |
| S5 | inconsistent (partial, manual) | — |

### Metrik tambahan (kode final)

- **Recovery time** (deteksi kegagalan → final konsisten): setara antar
  pendekatan — S2: 22 vs 25 ms, S3: 18 vs 31 ms.
- **Periode inkonsistensi sementara** (saga_log pertama → terakhir): setara pada
  beban rendah (±31–49 ms); pada S7 choreography ±6238 ms vs orchestration
  ±864 ms (rantai Kafka serial).
- Latency end-to-end: orchestration lebih rendah pada S2/S3/S7; perbedaan
  berasal dari jalur eksekusi, bukan kecepatan pemulihan.
