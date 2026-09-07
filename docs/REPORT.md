# Laporan Hasil Eksperimen

Laporan hasil eksperimen skenario S1–S7 untuk pendekatan **choreography**
dan **orchestration**. Data mentah per iterasi tersimpan di `docs/runs/` dan
agregat di `docs/runs/summary.json` (dihasilkan oleh `go run ./cmd/analyze`).

## Metodologi Ringkas

- 4 service (Order, Payment, Inventory, Shipping), database-per-service (PostgreSQL 15).
- Choreography: koordinasi via Apache Kafka (event-based, 8 topik, partisi tunggal).
- Orchestration: koordinasi via Saga Orchestrator (HTTP request-reply) + state di Redis.
- Setiap skenario diulang **30 iterasi**; di antara iterasi dilakukan reset penuh
  (truncate DB, flush Redis, reset counter fault).
- Metrik (sesuai proposal 3.5): **Consistency Rate**, **Compensating Transaction
  Success Rate (CTSR)**, **Recovery Time** (deteksi kegagalan → final state
  konsisten, dari timestamp `saga_log`), plus **periode inkonsistensi sementara**
  (saga_log pertama → terakhir, proposal 3.7) dan latency end-to-end.
- Statistik deskriptif per skenario: rata-rata, min, max, standar deviasi (proposal 3.7).

## Hasil Agregat (30 iterasi, kode final)

### Skenario 1 transaksi (S1, S2, S3, S6)

| Skenario | Approach | Txns | Committed | Compensated | Inconsistent | Consistency% | CTSR% | Recovery (ms) | Inconsistency window (ms) | Latency (ms) |
|----------|----------|------|-----------|-------------|--------------|--------------|-------|---------------|---------------------------|--------------|
| S1 | choreography | 30 | 30 | 0 | 0 | 100.0 | — | — | 31 | 165 |
| S1 | orchestration | 30 | 30 | 0 | 0 | 100.0 | — | — | 31 | 154 |
| S2 | choreography | 30 | 0 | 30 | 0 | 100.0 | 100.0 | 22 | 48 | 1639 |
| S2 | orchestration | 30 | 0 | 30 | 0 | 100.0 | 100.0 | 25 | 49 | 160 |
| S3 | choreography | 30 | 0 | 30 | 0 | 100.0 | 100.0 | 18 | 37 | 1670 |
| S3 | orchestration | 30 | 0 | 30 | 0 | 100.0 | 100.0 | 31 | 49 | 173 |
| S6 | choreography | 30 | 0 | 0 | 30 | 0.0 | 0.0 | — | 25 | 129 |
| S6 | orchestration | 30 | 0 | 0 | 30 | 0.0 | 0.0 | — | 50 | 188 |

### Skenario konkurensi (S7, 500 transaksi/iterasi → 15.000 transaksi total)

| Skenario | Approach | Txns | Committed | Unrecorded* | Consistency% | Inconsistency window (ms) | Latency (ms) |
|----------|----------|------|-----------|-------------|--------------|---------------------------|--------------|
| S7 | choreography | 15000 | 15000 | 0 | 100.0 | 6238 | 9722 |
| S7 | orchestration | 15000 | 14998 | 2 | 100.0 | 864 | 5241 |

\* Unrecorded = request yang gagal terkirim/mendapat respons di pintu masuk saat
puncak beban (koneksi diputus paksa); transaksi tidak pernah dimulai, bukan
inkonsistensi data.

## Analisis per Skenario

### S1 — Baseline Normal
Kedua pendekatan konsisten penuh (100%). Periode inkonsistensi sementara
identik (±31 ms) dan latency seimbang (±165 vs ±154 ms).

### S2 — Kegagalan Shipping (langkah akhir)
Kompensasi berantai berhasil penuh di kedua pendekatan (CTSR 100%).
**Recovery time setara** (±22 vs ±25 ms) — mekanisme pemulihan sama cepatnya.
Perbedaan latency end-to-end (±1639 vs ±160 ms) berasal dari jalur eksekusi
forward choreography yang melewati beberapa hop Kafka, bukan dari kecepatan
pemulihan.

### S3 — Kegagalan Inventory (langkah tengah)
Pola sama dengan S2: kompensasi penuh (100%), **recovery time setara** bahkan
sedikit lebih cepat di choreography (±18 vs ±31 ms). Periode inkonsistensi
sementara juga setara (±37 vs ±49 ms).

### S6 — Kegagalan Compensating Transaction
Kedua pendekatan **tidak** dapat memulihkan konsistensi (0%). Compensating
transaction yang gagal (marker `compensate_failed` di `saga_log`) tidak memiliki
mekanisme retry di kedua pendekatan, sehingga data tertinggal pada status
parsial permanen. Temuan ini menyoroti pentingnya idempotensi + retry pada
compensating transaction.

### S7 — Konkurensi 500 Transaksi
Hasil setelah perbaikan metodologi pengukuran (deteksi quiescence berbasis
`created_at` `saga_log`): **kedua pendekatan 100% konsisten** untuk seluruh
transaksi yang diproses. Perbedaan utama:
- **Periode inkonsistensi sementara**: choreography ±6238 ms vs orchestration
  ±864 ms — ±7× lebih lama. Rantai event Kafka dengan partisi tunggal
  mengantri 500 transaksi secara serial, sehingga transaksi terakhir berada
  dalam kondisi parsial selama beberapa detik.
- **Latency end-to-end**: choreography ±9722 ms vs orchestration ±5241 ms.
- **Availability pintu masuk**: choreography memproses seluruh 15.000 transaksi;
  orchestration kehilangan 2 request (0.01%) saat puncak beban karena entry
  point tunggalnya (orchestrator + port forward) menolak koneksi.

## Observasi S4 & S5 (otomatis, 10 run)

Dijalankan dengan `scripts/run-crash.ps1`; `DELAY_MS=3000` diset untuk
menciptakan window crash yang deterministik (saga berjalan ~3 s per langkah,
sehingga komponen dapat dihentikan saat transaksi masih berlangsung; tanpa
delay, saga selesai dalam ±100 ms lebih cepat dari latency `docker stop`).

- **S4 — Orchestrator Crash** (10 run): **10/10 inconsistent**. Pola konsisten:
  Order dan Payment `committed`, Inventory/Shipping tidak berpartisipasi —
  orchestrator dihentikan saat langkah inventory masih berjalan, sehingga
  kompensasi mundur tidak pernah dieksekusi. Commit parsial permanen.
- **S5 — Kafka Down** (10 run): **10/10 inconsistent**. Order `committed`;
  Payment kadang `committed` (jika event sempat dikonsumsi sebelum broker mati)
  atau tidak berpartisipasi; Inventory/Shipping tidak pernah berpartisipasi —
  rantai event terputus tanpa mekanisme replay, saga tidak pernah selesai.

Kedua skenario mengonfirmasi temuan utama: kegagalan komponen koordinasi
(orchestrator / broker) mengakibatkan **commit parsial tanpa kompensasi** di
pendekatan masing-masing.

## Perbandingan & Rekomendasi

1. **Konsistensi data (RM1)**: kedua pendekatan **setara** — keduanya 100%
   konsisten pada semua skenario yang dapat pulih (S1/S2/S3/S7) dan keduanya
   0% saat kompensasi gagal (S6). Tidak ada pendekatan yang lebih unggul dalam
   menjaga konsistensi akhir.
2. **Compensating transaction success rate (RM2)**: setara (100% pada S2/S3,
   0% pada S6). Kelemahan saga terletak pada compensating transaction itu
   sendiri, bukan pada mekanisme koordinasi.
3. **Recovery time (RM3)**: setara (±18–31 ms) pada kegagalan langkah.
   Klaim awal "orchestration 4–5× lebih cepat pulih" tidak terdukung ketika
   recovery time diukur sesuai definisi (deteksi kegagalan → final konsisten);
   perbedaan latency yang besar justru berasal dari jalur eksekusi, bukan
   pemulihan.
4. **Periode inkonsistensi sementara**: setara pada beban rendah (±37–49 ms),
   tetapi **±7× lebih lama di choreography pada beban tinggi** (±6,2 s vs
   ±0,9 s) karena serialisasi rantai event.
5. **Skalabilitas & titik tunggal**: choreography memproses seluruh transaksi
   tanpa kehilangan request namun lambat; orchestration lebih cepat namun entry
   point tunggalnya dapat menolak request pada puncak beban.

**Rekomendasi**: untuk sistem yang membutuhkan latency rendah dan periode
inkonsistensi sementara pendek, **orchestration** lebih tepat; untuk sistem
yang menuntut tidak ada request yang hilang dan menghindari titik tunggal,
**choreography** lebih tepat — dengan catatan perlu mekanisme outbox/event
replay dan idempotensi pada kompensasi.

## Catatan Metodologi

- Pengukuran awal S7 menghasilkan angka inconsistent yang menyesatkan karena
  checker mengklasifikasikan saga yang masih berjalan sebagai "inconsistent"
  (polling terlalu agresif, window quiescence 3 detik < jeda antar-langkah pada
  beban tinggi). Diperbaiki dengan: query checker per-saga (indeks `saga_id`,
  bukan full-table scan), interval polling 1,5 detik, deteksi quiescence 10
  detik berbasis `created_at` `saga_log`, dan klasifikasi langsung saat marker
  `compensate_failed` ditemukan (saga "sealed").
- Recovery time dihitung dari timestamp `saga_log` di database service, bukan
  dari polling workload generator, sehingga presisinya ±milidetik.
- Nilai latency S6 (±129–188 ms) mencerminkan waktu konfirmasi inkonsistensi,
  bukan waktu pemulihan (tidak ada pemulihan pada S6).

## Rekomendasi Langkah Selanjutnya

- Menambahkan retry pada compensating transaction dan mengukur dampaknya
  terhadap S6.
- Eksperimen dengan partisi Kafka > 1 untuk mengamati pengaruh paralelisme
  terhadap periode inkonsistensi choreography pada S7.