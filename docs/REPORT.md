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
| S8 | choreography | 10 | 0 | 0 | 0.0 (stuck) | 0 | 12811 |
| S9 | orchestration | 10 | 0 (10 compensated) | 0 | 100.0 (needless) | 10056 | 10162 |

\* Unrecorded = request yang gagal terkirim/mendapat respons di pintu masuk saat
puncak beban (koneksi diputus paksa); transaksi tidak pernah dimulai, bukan
inkonsistensi data. S8/S9 (event/response loss) dijalankan 10× untuk observasi;
latency-nya mencakup window konfirmasi (±10 detik quiescence/timeout) karena saga
tidak pernah selesai normal.

## Uji Signifikansi Statistik (Mann-Whitney U)

Perbandingan choreography vs orchestration per skenario menggunakan
**Mann-Whitney U test** (non-parametrik, dua sisi, α=0,05) pada mean per run
(n=30 per pendekatan; menghindari pseudo-replication pada S7). Data mentah di
`docs/runs/significance.json`.

| Skenario | Metrik | Choreo (ms) | Orchestr. (ms) | p-value | Signifikan |
|----------|--------|-------------|----------------|---------|------------|
| S1 | latency | 165,2 ± 276,2 | 154,1 ± 29,2 | <0,0001 | ya |
| S1 | inconsistency window | 30,8 ± 4,9 | 30,6 ± 8,4 | 0,2178 | tidak |
| S2 | latency | 1638,7 ± 16,3 | 160,2 ± 16,6 | <0,0001 | ya |
| S2 | inconsistency window | 48,0 ± 9,2 | 48,8 ± 6,6 | 0,3493 | tidak |
| S2 | **recovery time** | **22,5 ± 3,8** | **25,3 ± 3,5** | **0,0004** | **ya** |
| S3 | latency | 1670,2 ± 48,3 | 173,1 ± 21,1 | <0,0001 | ya |
| S3 | inconsistency window | 36,8 ± 8,1 | 48,8 ± 12,6 | <0,0001 | ya |
| S3 | **recovery time** | **18,0 ± 4,5** | **31,1 ± 7,9** | **<0,0001** | **ya** |
| S6 | latency | 128,5 ± 13,6 | 188,0 ± 39,5 | <0,0001 | ya |
| S6 | inconsistency window | 24,5 ± 5,3 | 50,0 ± 18,1 | <0,0001 | ya |
| S7 | latency | 9721,6 ± 823,8 | 5240,7 ± 794,6 | <0,0001 | ya |
| S7 | inconsistency window | 6238,0 ± 912,9 | 863,7 ± 313,7 | <0,0001 | ya |

**Interpretasi:**
- **Recovery time (S2/S3)**: perbedaan **signifikan** — choreography pulih lebih
  cepat (p<0,001). Magnitudo kecil (±3–13 ms); secara praktis keduanya pulih
  dalam puluhan milidetik. Klaim "setara" yang dilaporkan pada pengukuran awal
  **dikoreksi**: secara statistik tidak setara, melainkan choreography lebih
  cepat — sejalan dengan klaim Malyuga et al. (2020).
- **Window inkonsistensi beban rendah (S1/S2)**: tidak berbeda signifikan.
- **Latency & window inkonsistensi beban tinggi (S7)**: berbeda signifikan,
  orchestration jauh lebih cepat/singkat.

## Analisis per Skenario

### S1 — Baseline Normal
Kedua pendekatan konsisten penuh (100%). Periode inkonsistensi sementara
identik (±31 ms) dan latency seimbang (±165 vs ±154 ms).

### S2 — Kegagalan Shipping (langkah akhir)
Kompensasi berantai berhasil penuh di kedua pendekatan (CTSR 100%).
**Recovery time berbeda signifikan secara statistik** (Mann-Whitney U, p<0,001):
choreography 22,5 ms vs orchestration 25,3 ms — choreography lebih cepat, namun
magnitudo selisih kecil (±3 ms). Perbedaan latency end-to-end (±1639 vs ±160 ms)
jauh lebih besar dan berasal dari jalur eksekusi forward choreography yang melewati
beberapa hop Kafka, bukan dari kecepatan pemulihan.

### S3 — Kegagalan Inventory (langkah tengah)
Pola sama dengan S2: kompensasi penuh (100%), **recovery time berbeda signifikan**
(p<0,0001): choreography 18,0 ms vs orchestration 31,1 ms — choreography lebih cepat
(selisih ±13 ms). Periode inkonsistensi sementara juga berbeda signifikan (36,8 vs
48,8 ms, p<0,0001). Arah temuan ini mendukung klaim Malyuga et al. (2020) bahwa
choreography bekerja lebih cepat pada mekanisme koordinasinya.

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

### S8 — Event Loss Parsial (choreography only)
Mensimulasikan *dual-write problem*: order di-commit ke database, tetapi event
`saga.order.created` di-drop di titik publish (producer melaporkan sukses).
Hasil **10/10 inconsistent** — saga stuck permanen: Order `committed`, service
lain tidak berpartisipasi, tidak ada marker kegagalan sehingga kompensasi tidak
terpicu dan tidak ada mekanisme (outbox/replay) yang menyelamatkannya.

### S9 — Response Hilang / In-Doubt (orchestration only)
**Pasangan struktural dari S8** (akar masalah sama: sinyal koordinasi hilang).
Inventory berhasil di-commit tetapi response HTTP-nya di-drop (handler menahan
response melewati timeout orchestrator 10 s). Orchestrator menyimpulkan gagal dan
menjalankan kompensasi penuh. Hasil **10/10 `compensated` dengan flag
`needless_compensation`** — semua langkah sebenarnya sukses (tidak ada marker
kegagalan), namun transaksi dibatalkan sia-sia (false negative).

### Perbandingan S8 vs S9 — kedua pendekatan sama-sama rentan sinyal hilang

| Aspek | S8 (choreography) | S9 (orchestration) |
|-------|--------------------|--------------------|
| Sinyal hilang | Event dipublish di-drop | Response HTTP di-drop setelah commit |
| Konsistensi akhir | **0%** — saga stuck, Order menggantung | **100%** — semua dikompensasi |
| Kategori outcome | inconsistent | compensated + needless |
| Biaya kegagalan | Data meninggalkan state parsial permanen | Transaksi valid hilang (dibatalkan sia-sia) |

Kesimpulan untuk RM1/RM2: **tidak ada pendekatan yang bebas dari kelemahan
struktural terhadap hilangnya sinyal koordinasi** — choreography mengorbankan
konsistensi data (stuck), orchestration mengorbankan transaksi yang valid
(over-compensation). Perbedaannya: orchestration selalu berakhir konsisten
(dengan biaya pembatalan), sedangkan choreography dapat meninggalkan data
menggantung tanpa penyelesaian. Temuan ini selaras dengan analisis Malyuga et
al. (2020) bahwa endpoint idempotent dan pemulihan state diperlukan pada
sistem berbasis orchestrator.

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

1. **Konsistensi data (RM1)**: setara pada kondisi normal (S1/S2/S3/S7: 100%
   keduanya) dan saat kompensasi gagal (S6: 0% keduanya). **Perbedaan muncul
   pada mode kegagalan sinyal koordinasi hilang**: S8 (event di-drop,
   choreography) → saga stuck, 0% konsisten; S9 (response di-drop,
   orchestration) → fully compensated, 100% konsisten tetapi transaksi valid
   dibatalkan sia-sia (needless).
2. **Compensating transaction success rate (RM2)**: setara pada kompensasi
   normal (100% S2/S3) dan kegagalan kompensasi (0% S6). Pada S8 kompensasi
   tidak terpicu sama sekali (tidak ada yang mengetahui kegagalan); pada S9
   kompensasi berhasil penuh (100%) namun tidak diperlukan (false negative).
3. **Recovery time (RM3)**: berbeda signifikan secara statistik (Mann-Whitney U)
   pada skenario kegagalan langkah — choreography lebih cepat (S2: 22,5 vs
   25,3 ms, p<0,001; S3: 18,0 vs 31,1 ms, p<0,0001). Magnitudo selisih kecil
   (±3–13 ms) dan keduanya pulih dalam puluhan milidetik. Klaim awal
   "orchestration jauh lebih cepat pulih" tidak terdukung; perbedaan latency
   yang besar berasal dari jalur eksekusi, bukan pemulihan.
4. **Periode inkonsistensi sementara**: tidak berbeda signifikan pada S1/S2
   (31/31 dan 48/49 ms), tetapi **±7× lebih lama di choreography pada beban
   tinggi** (S7: 6238 vs 864 ms, p<0,0001) karena serialisasi rantai event.
5. **Skalabilitas & titik tunggal**: choreography memproses seluruh transaksi
   tanpa kehilangan request namun lambat; orchestration lebih cepat namun entry
   point tunggalnya dapat menolak request pada puncak beban.

**Rekomendasi**: untuk sistem yang membutuhkan latency rendah dan periode
inkonsistensi sementara pendek, **orchestration** lebih tepat; untuk sistem
yang menuntut tidak ada request yang hilang dan menghindari titik tunggal,
**choreography** lebih tepat — dengan catatan perlu mekanisme outbox/event
replay (mencegah saga stuck pada event loss) dan idempotensi + verifikasi
eksplisit pada kompensasi orchestration (mencegah pembatalan transaksi valid
pada response loss).

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

- Menambahkan mekanisme **outbox pattern / event replay** di choreography dan
  mengukur dampaknya terhadap S8 (event loss).
- Menambahkan retry pada compensating transaction dan mengukur dampaknya
  terhadap S6.
- Eksperimen dengan partisi Kafka > 1 untuk mengamati pengaruh paralelisme
  terhadap periode inkonsistensi choreography pada S7.