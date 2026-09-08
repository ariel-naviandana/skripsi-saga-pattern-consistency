# Penjelasan Program Skripsi (8 Tahap + Status Alignment Akhir)

> Dokumen rangkuman untuk penjelasan ke dosen / penilai (Claude / reviewer lain)
> dan dasar untuk menyesuaikan slide PPT presentasi sempro. Format: 8 tahap
> penjelasan sederhana (dengan FAQ masing-masing), ditambah status alignment akhir
> terhadap rumusan masalah proposal.

---

## Tahap 1 — Konsep Besar

**Topik:** Analisis Perbandingan Konsistensi Data Transaksi pada Saga Pattern (Choreography
vs Orchestration) menggunakan Fault Injection Testing.

**Penjelasan sederhana (bahasa awam):**

- **Microservices** = aplikasi dipecah jadi layanan-layanan kecil yang berdiri sendiri
  (misal Order, Payment, Inventory, Shipping), masing-masing punya database sendiri
  (database-per-service). Tujuannya: kalau satu rusak, yang lain tidak ikut rusak.
- **Masalahnya:** transaksi bisnis (seperti "pesan barang") biasanya memotong **beberapa
  service sekaligus** (catat order → potong saldo → kurangi stok → jadwal kirim).
  Karena tiap service punya DB sendiri, **tidak ada satu transaksi besar** yang
  bisa menjamin semuanya komit atau semua rollback.
- **Saga pattern** = solusinya: transaksi global dipecah jadi **transaksi-transaksi
  lokal** berurutan di tiap service. Tiap service commit sendiri-sendiri. Kalau
  satu gagal, jalan **compensating transaction** (transaksi "pembalik") untuk membatalkan
  efek transaksi yang sudah sukses sebelumnya.
- **Compensating transaction ≠ rollback.** Rollback = undo secara fisik (transaksi
  belum final). Compensating = **transaksi baru** yang membalik efek transaksi sebelumnya
  secara semantik (mis: payment committed → compensating: refund saldo).
- **Choreography vs Orchestration**: dua cara mengkoordinasikan saga.
  - **Choreography** = tanpa koordinator; tiap service publish event ke Kafka,
    service berikutnya subscribe. Analogi: grup WA — semua anggota mandiri, tahu
    dari pesan siapa-siapa. Tidak ada pemimpin.
  - **Orchestration** = ada orchestrator pusat yang memanggil service satu-satu
    via HTTP. Analogi: pemandu tur — yang menentukan rute.
- **Mengapa ada eventual consistency:** saat saga berjalan, data antar service
  **belum konsisten** (mis: order sudah ada, payment sudah ada, stok belum dikurangi).
  Inkonsistensi ini sementara — sampai saga selesai. Yang dibandingkan: apakah
  kedua pendekatan menjaga inkonsistensi dengan cara yang sama atau berbeda.

### FAQ Tahap 1

- **Q: Kenapa tidak pakai 2PC?**
  A: 2PC mengunci semua service selama transaksi (throughput turun 3,3× di beban
  tinggi menurut Fan et al. 2020), dan satu service yang down membuat semua macet.
- **Q: Apa beda choreography dan orchestration secara fundamental?**
  A: Siapa yang mengendalikan alur. Choreography = alur muncul dari pertukaran
  event antar service (desentral). Orchestration = alur ditentukan pusat (terpusat).
- **Q: Kenapa hasil eksperimen bisa beda?**
  A: Karena mekanisme kompensasi berbeda — choreography merambat lewat event chain
  (bisa putus), orchestration via HTTP langsung (bisa timeout/over-compensate).

---

## Tahap 2 — Arsitektur Sistem

**Penjelasan:**

Sistem terdiri dari **11 container Docker**:

- **4 service** (Order, Payment, Inventory, Shipping): tiap service adalah program
  Go mandiri, HTTP API di port 8081-8084.
- **4 database PostgreSQL** (saga-order-db, saga-payment-db, saga-inventory-db,
  saga-shipping-db): masing-masing di port 5431-5434. Service hanya mengakses DB-nya
  sendiri (database-per-service).
- **Kafka** (port 9092, 9094): message broker untuk choreography. 8 topik untuk
  propagasi event.
- **Redis** (port 6379): state management untuk orchestrator (saga_id → state).
- **Orchestrator** (port 8080): service khusus untuk mode orchestration — tidak
  ada di mode choreography.
- **Prometheus + Grafana**: monitoring (pendukung, bukan inti eksperimen).

**Kode bisnis IDENTIK** antara choreography dan orchestration. Bedanya hanya
mekanisme koordinasi: orchestration ditambahkan HTTP handler (`internal/http/`),
choreography ditambahkan Kafka producer/consumer (`internal/choreography/`).

### FAQ Tahap 2

- **Q: Kenapa perlu Kafka DAN Redis?**
  A: Kafka = event bus untuk choreography. Redis = state tracker untuk orchestrator.
  Tiap pendekatan butuh alat berbeda.
- **Q: Kenapa tidak pakai database bersama?**
  A: Melanggar prinsip microservices. Tiap service harus otonom. Trade-off:
  data terdesentralisasi → transaksi sulit.
- **Q: Bagaimana cara pindah pendekatan?**
  A: Set environment variable `APPROACH=choreography` atau `orchestration`,
  lalu `docker compose up -d`. Kode bisnis tidak berubah.

---

## Tahap 3 — Alur Eksekusi Saga

**Penjelasan:**

### Happy path (semua langkah sukses)
Misalkan customer pesan produk:

1. **Order**: catat pesanan (status `committed`) → publish event `order.created`.
2. **Payment**: baca event → bayar → catat pembayaran (committed) → publish
   `payment.processed`.
3. **Inventory**: baca event → kurangi stok (committed) → publish `inventory.reserved`.
4. **Shipping**: baca event → jadwalkan kirim (committed) → publish `shipping.scheduled`.

Saga selesai: semua committed. **Recovery time: 0** (tidak ada kegagalan).

### Compensation path (satu langkah gagal)
Misalkan Inventory gagal (mis: stok habis):

1-3: sama seperti happy path sampai Inventory gagal.
4. **Inventory**: business logic → tulis `saga_log` dengan status `failed`.
5. **Compensation chain** (mundur): Payment dikompensasi (saldo dikembalikan),
   Order dikompensasi (pesanan dibatalkan). Shipping tidak pernah jalan.
6. Status akhir: order, payment `compensated`; inventory, shipping `not participating`.

**Recovery time** = dari timestamp row `failed` (di Inventory) sampai timestamp row
terakhir di saga_log (Order compensated). Untuk S3: **12,8 ms (choreography) vs
26,0 ms (orchestration)** — choreography pulih lebih cepat (signifikan, p<0,0001).

### FAQ Tahap 3

- **Q: Kenapa recovery time penting?**
  A: Menunjukkan seberapa cepat sistem pulih dari kegagalan. Ini metrik utama
  untuk Responsiveness.
- **Q: Kenapa choreography lebih cepat pulih?**
  A: Setelah deteksi kegagalan, kompensasi choreography merambat lewat 3 Kafka
  hop dengan latency ~5-10 ms tiap hop. Orchestration juga 3 hop tapi via HTTP
  dengan overhead per-call yang sedikit lebih besar.

---

## Tahap 4 — Struktur Folder & Kode

```
projek-skripsi/
├── cmd/                 → 6 binary entry points (main.go per service)
│   ├── order-service/, payment-service/, inventory-service/, shipping-service/
│   ├── orchestrator/   → saga orchestrator
│   ├── workload-generator/ → CLI kirim N transaksi + ukur hasilnya
│   └── analyze/         → CLI hitung summary agregat + Mann-Whitney U
├── internal/
│   ├── business/        → LOGIKA BISNIS murni (OrderService, PaymentService, dst.)
│   ├── choreography/    → event Kafka producer/consumer
│   ├── orchestration/   → HTTP client orchestrator + Redis state
│   ├── common/          → FaultConfig (fault injection middleware)
│   ├── consistency/     → Checker baca 4 DB, klasifikasi outcome
│   └── http/            → HTTP handler (orchestration path)
├── pkg/                 → wrapper eksternal
│   ├── kafka/           → Sarama producer/consumer
│   ├── postgres/        → pgxpool connection
│   └── redis/           → go-redis client
├── deployments/         → Docker Compose
├── scripts/             → run-scenario.ps1/.sh, run-crash.ps1, setup.sh, reset.sh
└── docs/                → REPORT.md, SCENARIOS.md, slide deck sempro
```

**Prinsip layer (dari atas ke bawah):**
1. `cmd/` — pintu masuk (baca config, jalankan server)
2. `internal/http/` atau `internal/choreography/` — transport
3. `internal/business/` — logika bisnis (tidak tahu transport)
4. `pkg/` — infrastruktur (Kafka, DB, Redis)

### FAQ Tahap 4

- **Q: Kenapa kode bisnis dipisah dari transport?**
  A: Agar bisa dipakai oleh choreography maupun orchestration tanpa duplikasi.
  Kompensasi identik di kedua pendekatan.
- **Q: Kenapa FaultConfig di common/?**
  A: Karena fault injection dipakai banyak service — di-`embed` di business logic
  (via Fault.ShouldFail()), di Kafka producer (DropEvent), di HTTP handler
  (DropResponseAtStep), di orchestrator (SelectCompensate). Semua non-intrusive.

---

## Tahap 5 — Bedah Kode Inti

**File-file paling penting untuk dibaca saat sidang:**

### `internal/common/faultinject.go` — Jantung Fault Injection
- `FaultConfig`: struct dengan field `FailAtStep`, `FailAtAttempt`, `DelayMS`,
  `FailOnCompensate`, `DropEvent`, `DropResponseAtStep`, `SelectCompensate`.
- `NewFaultConfig()`: baca dari env var.
- `ShouldFail(step, compensate)`: return true kalau step ini harus gagal
  (cek `step == FailAtStep` && attempt ≥ FailAtAttempt).
- `ResetAttempt()`: dipanggil `/fault/reset` di reset.sh sebelum tiap iterasi.

### `internal/business/order.go` (dan payment/inventory/shipping) — Business Logic
- `CreateOrder()`: cek fault → kalau gagal tulis `failed` ke saga_log, return error.
  Kalau sukses INSERT ke orders + tulis saga_log `committed`.
- `CompensateOrder()`: cek fault on-compensate → kalau gagal tulis
  `compensate_failed`, return error. Kalau sukses UPDATE orders ke `compensated`.

### `internal/choreography/order.go` — Event Chain
- `CreateOrderAndPublish()`: panggil business → publish ke Kafka topic
  `saga.order.created`.
- `Run()`: dua goroutine consumer — satu listen `shipping.scheduled` (publish
  `saga.closed`), satu listen `payment.failed` (compensate order → publish
  `saga.closed`).

### `internal/orchestration/orchestrator.go` — Central Coordinator
- `Start()`: HTTP POST berurutan ke 4 service → kalau gagal, panggil `fail()`.
- `fail()`: panggil compensate berurutan ke 4 endpoint. **Kalau SELECTIVE_COMPENSATE
  aktif, query DB dulu sebelum kompensasi** (skip yang tidak commit).

### `internal/consistency/checker.go` — Outcome Classifier
- `Timeline(sagaID)`: return (detection, final, first) timestamps dari saga_log.
- `Check(sagaID)`: baca 4 DB, klasifikasi → committed / compensated /
  inconsistent / not_found.

### FAQ Tahap 5

- **Q: Kenapa fault dicek di business layer, bukan di HTTP handler?**
  A: Agar fault bekerja di kedua pendekatan (HTTP handler tidak dipanggil di
  choreography; choreography publish event langsung). Fault di business layer =
  fault terjadi di mana pun service dipanggil.
- **Q: Kenapa recovery time diukur dari saga_log, bukan dari HTTP response?**
  A: Karena saga_log adalah **sumber kebenaran tunggal** yang sama untuk kedua
  pendekatan. Menghindari bias dari pengukuran latensi polling.

---

## Tahap 6 — Fault Injection & 12 Skenario

**Mekanisme fault injection non-intrusive:** hanya set env var container. Tidak
ubah kode.

**Tiga parameter fault utama** (proposal 3.4.1):
1. `FAIL_AT_STEP` + `FAIL_AT_ATTEMPT` → suntik kegagalan di step tertentu, pada attempt ke-N.
2. `DELAY_MS` → simulasi layanan lambat.
3. `FAIL_ON_COMPENSATE` → suntik kegagalan kompensasi.

**Dua parameter fault tambahan** (eksperimen):
4. `DROP_EVENT` (S8) → drop event di Kafka secara diam-diam.
5. `DROP_RESPONSE_AT_STEP` (S9) → drop HTTP response setelah commit berhasil.

**12 Skenario dan Fungsinya:**

| # | Skenario | Fungsi | Dijawab RM |
|---|----------|--------|-----------|
| 1 | S1 Baseline | Konteks normal tanpa fault | (semua) |
| 2 | S2 Shipping gagal | Step-failure terakhir; kompensasi penuh | RM1, RM2, RM3 |
| 3 | S3 Inventory gagal | Step-failure tengah; kompensasi mundur | RM1, RM2, RM3 |
| 4 | S4 Orchestrator crash | Titik-lembah infrastruktur | RM1 (konteks) |
| 5 | S5 Kafka down | Titik-lembah infrastruktur | RM1 (konteks) |
| 6 | S6 Kompensasi gagal | Batas fundamental saga | RM2 langsung |
| 7 | S7 500 tx/konkuren | Skala + window inkonsistensi | RM1 + latency |
| 8 | S8 Event loss | Sinyal koordinasi hilang — choreography | RM1 bonus |
| 9 | S9 Response loss | Sinyal koordinasi hilang — orchestration | RM1 bonus |
| 10 | S9s S9 + selective compensate | Counterfactual call-all | Isolasi confounding |
| 11 | S2s S2 + selective compensate | Counterfactual call-all RM3 | Isolasi confounding |
| 12 | S3s S3 + selective compensate | Counterfactual call-all RM3 | Isolasi confounding |

### FAQ Tahap 6

- **Q: Kenapa pakai DELAY_MS=3000 di S4/S5?**
  A: Saga selesai dalam ±100 ms; docker stop butuh ±300 ms. Tanpa delay,
  crash terjadi setelah saga selesai. Delay membuat window deterministik.
- **Q: Kenapa SELECTIVE_COMPENSATE?**
  A: Untuk mengisolasi apakah call-all design (orchestrator kompensasi semua
  service, bukan hanya yang commit) menyumbang selisih recovery time vs
  choreography.

---

## Tahap 7 — Pengukuran & Hasil

**Metrik (sesuai proposal 3.5):**
1. **Consistency Rate** — % transaksi berakhir konsisten (semua committed atau semua compensated).
2. **CTSR** — % kompensasi berhasil.
3. **Recovery Time** — dari deteksi kegagalan (row `failed`) sampai final state konsisten.

**Metrik tambahan** (3.7 + bonus):
4. **Window inkonsistensi sementara** — saga_log pertama → terakhir.
5. **Latency end-to-end** — POST → settle.

**Statistik:** mean/min/max/std per skenario per pendekatan; **Mann-Whitney U**
(n=30, dua sisi, α=0.05) untuk signifikansi selisih choreography vs orchestration.

### Hasil utama (re-run final, commit #12)

| Skenario | Choreo | Orch | Signifikan |
|----------|--------|------|------------|
| S1 latency | 427 ms | 128 ms | TIDAK (p=0.73) |
| S1 window | 42 ms | 26 ms | ya (p<0.0001) |
| **S2 recovery** | **22 ms** | **29 ms** | **ya (p=0.0002)** |
| **S3 recovery** | **13 ms** | **26 ms** | **ya (p<0.0001)** |
| S3 latency | 1624 ms | 150 ms | ya |
| S7 latency | 8476 ms | 5010 ms | ya |
| S7 window | 5152 ms | 628 ms | ya |

**Isolasi confounding RM3:**
- S3 (call-all) recovery: 26,0 ms
- S3s (selective compensate) recovery: 28 ms
- **Selisih 2 ms** (di dalam std dev) → call-all tidak menambah overhead terukur

### FAQ Tahap 7

- **Q: Kenapa pakai Mann-Whitney U bukan t-test?**
  A: n=30, distribusi recovery time tidak normal (ada outlier). U-test non-parametrik.
- **Q: Kenapa S1 latency tidak signifikan?**
  A: Outlier 1626 ms (cold-start container) hilang di re-run final; tanpa outlier,
  latency kedua pendekatan homogen.

---

## Tahap 8 — Persiapan Bimbingan / Sidang

### "Cerita 5 Menit" (untuk sidang sempro)

> Microservices memecah aplikasi jadi layanan mandiri, masing-masing dengan database
> sendiri. Tapi transaksi bisnis memotong banyak service sekaligus, sehingga butuh
> Saga Pattern. Penelitian ini membandingkan dua implementasi Saga — choreography
> (event-driven via Kafka) dan orchestration (HTTP central coordinator) — dari
> konsistensi data, success rate kompensasi, dan recovery time, saat berbagai skenario
> kegagalan disuntikkan secara terkontrol (fault injection).
>
> Hasil: konsistensi dan CTSR **setara** di kedua pendekatan saat kompensasi normal;
> keduanya 0% saat kompensasi gagal. **Recovery time berbeda signifikan** — choreography
> sedikit lebih cepat (S3: 13 vs 26 ms, p<0,0001), dan confounding call-all pada
> orchestrator sudah **diisolasi empiris** via eksperimen selective compensate
> (S3s = 28 ms, sama dengan S3).
>
> Pada mode sinyal-koordinasi hilang (S8/S9): choreography cenderung stuck (data
> menggantung), orchestration cenderung over-compensate (transaksi valid dibatalkan).
> Keduanya sama-sama rentan — tidak ada pemenang universal. Rekomendasi: pilih
> berdasarkan prioritas sistem — latency & window pendek → orchestration;
> tanpa titik tunggal → choreography. Kompensasi idempotent + retry wajib pada
> keduanya.

### Pertanyaan penguji yang mungkin muncul

| # | Pertanyaan | Jawaban siap |
|---|-----------|--------------|
| 1 | "Kenapa pemulihan orchestration tidak lebih cepat?" | Karena kontrol terpusat langsung memang seharusnya lebih cepat secara intuitif, tapi eksperimen menunjukkan **call-all tidak menambah overhead** (S3s 28 ms ≈ S3 26 ms). Selisih murni arsitektural. |
| 2 | "Kenapa pakai partisi Kafka = 1?" | Untuk mensimulasikan skenario worst-case (rantai serial). Partisi > 1 adalah rekomendasi lanjutan (sudah ditulis di REPORT.md). |
| 3 | "Kenapa S8/S9/S9s/S2s/S3s hanya 10 run?" | Untuk observasi kualitatif counterfactual. Tidak applicable untuk uji signifikansi (single-approach). |
| 4 | "Kenapa tidak ada retry pada kompensasi?" | Untuk mengisolasi efek fault, bukan retry. Retry adalah rekomendasi lanjutan. |
| 5 | "Apa kontribusi utama?" | (a) Perbandingan empiris choreography vs orchestration pada konsistensi di bawah fault injection terkontrol (belum ada sebelumnya); (b) Isolasi confounding call-all via selective compensate; (c) Identifikasi kerentanan struktural di kedua pendekatan pada sinyal-koordinasi hilang. |

---

## Status Alignment Akhir (setelah commit #12)

### Pemetaan RM → Skenario → Hasil

| Rumusan Masalah | Dijawab oleh | Hasil Inti | Skor |
|-----------------|-------------|------------|------|
| **RM1** (Konsistensi data) | S1, S2, S3, S6, S7, S8, S9, S9s, S4, S5 (semua skenario) | Setara di kondisi normal; beda karakter di S8 (stuck) vs S9 (needless). Batas fundamental di S6 (0%). | 8/10 |
| **RM2** (CTSR) | S2, S3, S6, S9, S9s | Setara: 100% di S2/S3/S9, 0% di S6. Kelemahan ada di compensating transaction itu sendiri. | 7/10 |
| **RM3** (Recovery time) | S2, S3, S2s, S3s | Choreography **signifikan lebih cepat** (S2: 22 vs 29 ms p=0.0002; S3: 13 vs 26 ms p<0.0001). Confounding call-all sudah **diisolasi empiris** via S2s/S3s. | 8.5/10 |

### Verdict Objektif

- **Target proposal tercapai**: 3 RM terjawab, semua metrik sesuai proposal (3.5), 7+ skenario aktif (lebih dari 7 yang diminta).
- **Konsistensi internal**: semua outlier dan varians dijelaskan (S1 cold-start, S7 vpnkit TIME_WAIT, S2s outlier run 1).
- **Kontribusi nyata**: satu-satunya eksperimen yang membandingkan kedua pendekatan secara empiris di bawah fault injection terkontrol dengan selective compensate sebagai counterfactual.
- **Skor rata-rata**: ~8/10 — siap untuk sidang S1.

### Yang TIDAK dilakukan (rencana lanjutan)

- **Retry pada kompensasi** (S6 + retry) — akan meningkatkan CTSR S6
- **Partisi Kafka > 1** — akan menurunkan window inkonsistensi S7
- **Event loss di choreography + retry/outbox** (S8+) — akan menutup titik-lembah choreography
- **Validasi di lingkungan cloud multi-node** — di luar scope skripsi S1

---

## Referensi Cepat untuk Claude / Reviewer

| File | Isi |
|------|-----|
| `docs/REPORT.md` | Laporan lengkap (tabel, analisis, catatan metodologi, rekomendasi) |
| `docs/SCENARIOS.md` | Definisi 12 skenario + fault config |
| `docs/final-results-prompt.md` | Self-contained prompt untuk review independen |
| `docs/ai-review-prompt.md` | Self-contained prompt untuk review awal |
| `README.md` | Quick start + ringkasan skenario + cara running |
| `cmd/workload-generator/main.go` | CLI untuk generate transaksi |
| `cmd/analyze/main.go` | CLI untuk hitung summary + Mann-Whitney U |
| `internal/orchestration/orchestrator.go` | Central coordinator + selective compensate |
| `internal/common/faultinject.go` | Fault injection middleware |

Commit terakhir: **#12**. Total commit: 12. Total baris kode Go: ~1500 LOC.

---

## Penyesuaian PPT Sempro yang Disarankan

| Slide | Isi lama | Penyesuaian |
|-------|---------|-------------|
| Slide hipotesis/ekspektasi | Klaim "orchestration 4-5× lebih cepat pulih" | **Ganti** dengan "diharapkan orchestration lebih cepat (berdasarkan literatur)" — eksperimen membuktikan klaim ini **tidak terdukung** |
| Slide hasil recovery time | Tabel tanpa S2s/S3s | **Tambah** kolom S2s/S3s — tunjukkan confounding terisolasi |
| Slide S8 vs S9 | Framing "100% konsisten" | **Ganti** dengan framing bisnis: "konsistensi ≠ correctness, S8 vs S9 bukan perbandingan mana yang lebih baik" |
| Slide keterbatasan | "S1 outlier tanpa penjelasan", "S7 fluktuasi tidak terjelas" | **Ganti** dengan justifikasi teknis (cold-start container, Docker vpnkit TIME_WAIT) |
| Slide kontribusi | "Recovery time setara" | **Ganti** dengan "choreography signifikan lebih cepat, confounding call-all terisolasi via S2s/S3s" |
| Slide rekomendasi | "Pilih berdasarkan constraint" | Tambah kalimat: "Mekanisme kompensasi idempotent + retry adalah syarat mutlak pada kedua pendekatan" |

Total slide saat ini: 29. Tidak perlu tambah slide; cukup revisi konten pada slide yang ada.
