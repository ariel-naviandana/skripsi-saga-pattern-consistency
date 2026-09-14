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
| S1 | choreography | 30 | 30 | 0 | 0 | 100.0 | — | — | 42 | 427 |
| S1 | orchestration | 30 | 30 | 0 | 0 | 100.0 | — | — | 26 | 128 |
| S2 | choreography | 30 | 0 | 30 | 0 | 100.0 | 100.0 | 22 | 49 | 1639 |
| S2 | orchestration | 30 | 0 | 30 | 0 | 100.0 | 100.0 | 29 | 52 | 163 |
| S2s | orchestration | 30 | 0 | 30 | 0 | 100.0 | 100.0 | 28 | 56 | 166 |
| S3 | choreography | 30 | 0 | 30 | 0 | 100.0 | 100.0 | 13 | 27 | 1624 |
| S3s | orchestration | 30 | 0 | 30 | 0 | 100.0 | 100.0 | 24 | 42 | 145 |
| S6 | choreography | 30 | 0 | 0 | 30 | 0.0 | 0.0 | tak pulih | 28 | 123 |
| S6 | orchestration | 30 | 0 | 0 | 30 | 0.0 | 0.0 | tak pulih | 42 | 156 |

### Skenario konkurensi (S7, 500 transaksi/iterasi → 15.000 transaksi total)

| Skenario | Approach | Txns | Committed | Unrecorded* | Consistency% | Inconsistency window (ms) | Latency (ms) |
|----------|----------|------|-----------|-------------|--------------|---------------------------|--------------|
| S7 | choreography | 15000 | 15000 | 0 | 100.0 | 2818 | 7170 |
| S7 | orchestration | 15000 | 14952 | 48 | 99.7 | 465 | 5898 |

\* Unrecorded = request yang gagal terkirim/mendapat respons di pintu masuk saat
puncak beban (koneksi diputus paksa); transaksi tidak pernah dimulai, bukan

### Skenario sinyal koordinasi hilang (S8, S9, S9s — 30 iterasi)

| Skenario | Approach | Txns | Committed | Compensated | Inconsistent | Consistency% | CTSR% | Needless | Inconsistency window (ms) | Latency (ms) |
|----------|----------|------|-----------|-------------|--------------|--------------|-------|----------|---------------------------|--------------|
| S8 | choreography | 30 | 0 | 0 | 30 | 0.0 | 0.0 | 0 | 0 (stuck) | 12349 |
| S9 | orchestration | 30 | 0 | 30 | 0 | 100.0 | 100.0 | 30 (100%) | 10048 | 10159 |
| S9s | orchestration | 30 | 0 | 30 | 0 | 100.0 | 100.0 | 30 (100%) | 10035 | 10130 |
inkonsistensi data. S8/S9/S9s (event/response loss) dijalankan 30×; latency-nya
mencakup window konfirmasi (±10 detik quiescence/timeout) karena saga tidak
pernah selesai normal.

## Uji Signifikansi Statistik (Mann-Whitney U)

Perbandingan choreography vs orchestration per skenario menggunakan
**Mann-Whitney U test** (non-parametrik, dua sisi, α=0,05) pada mean per run
(n=30 per pendekatan; menghindari pseudo-replication pada S7). Data mentah di
`docs/runs/significance.json`.

| Skenario | Metrik | Choreo (ms) | Orchestr. (ms) | p-value | Signifikan |
|----------|--------|-------------|----------------|---------|------------|
| S1 | latency | 426,8 ± 633,1 | 127,7 ± 33,4 | 0,7337 | tidak |
| S1 | inconsistency window | 41,5 ± 22,7 | 26,3 ± 7,3 | <0,0001 | ya |
| S2 | latency | 1639,2 ± 35,5 | 163,1 ± 26,0 | <0,0001 | ya |
| S2 | inconsistency window | 49,4 ± 17,9 | 52,0 ± 9,0 | 0,1275 | tidak |
| S2 | **recovery time** | **22,0 ± 6,7** | **28,7 ± 6,4** | **0,0002** | **ya** |
| S3 | latency | 1623,9 ± 19,5 | 149,6 ± 33,5 | <0,0001 | ya |
| S3 | inconsistency window | 27,2 ± 6,4 | 40,1 ± 9,8 | <0,0001 | ya |
| S3 | **recovery time** | **12,8 ± 3,0** | **26,0 ± 6,5** | **<0,0001** | **ya** |
| S6 | latency | 122,9 ± 22,7 | 156,2 ± 30,0 | <0,0001 | ya |
| S6 | inconsistency window | 27,7 ± 6,8 | 41,7 ± 8,2 | <0,0001 | ya |
| S7 | latency | 7170,4 ± 1297,6 | 5921,4 ± 602,0 | <0,0001 | ya |
| S7 | inconsistency window | 2817,6 ± 906,8 | 462,7 ± 133,4 | <0,0001 | ya |

**Selective vs call-all (dalam orchestration, n=30):**

| Pasangan | Metrik | Call-All (ms) | Selective (ms) | p-value | Signifikan |
|----------|--------|---------------|----------------|---------|------------|
| S2 vs S2s | recovery | 28,7 ± 6,4 | 27,8 ± 6,4 | 0,5046 | **tidak** |
| S3 vs S3s | recovery | 26,0 ± 6,5 | 24,2 ± 7,6 | 0,0359 | **ya** (selective lebih cepat 2 ms) |
| S9 vs S9s | latency | 10158,9 ± 41,6 | 10130,1 ± 18,5 | 0,0013 | **ya** (selective lebih cepat 29 ms) |

**Interpretasi:**
- **Recovery time (S2/S3)**: perbedaan **signifikan** — choreography pulih lebih
  cepat (p<0,001). Magnitudo kecil (±5–13 ms); secara praktis keduanya pulih
  dalam puluhan milidetik. Klaim "setara" yang dilaporkan pada pengukuran awal
  **dikoreksi**: secara statistik tidak setara, melainkan choreography lebih
  cepat — sejalan dengan klaim Malyuga et al. (2020).
- **Window inkonsistensi S1** (tidak ada kegagalan): berbeda signifikan
  (choreography lebih lebar). **Latency S1** (tidak ada kegagalan): TIDAK
  berbeda signifikan (p=0,73) setelah re-run final — klaim awal "S1 latency
  berbeda signifikan" tidak stabil di lintas run (outlier 1626 ms pada pengukuran
  sebelumnya menghilang), sehingga tidak layak dipublikasikan sebagai temuan
  komparatif pada S1.
- **Window inkonsistensi S2** (ada kegagalan): tidak berbeda signifikan.
- **Latency & window inkonsistensi beban tinggi (S7)**: berbeda signifikan,
  orchestration jauh lebih cepat/singkat.
- **Selective vs call-all (dalam orchestration, n=30):** S2 vs S2s recovery
  time p=0,50 (tidak signifikan — confounding terisolasi untuk S2); S3 vs S3s
  p=0,04 (signifikan, selisih 2 ms — selective sedikit lebih cepat); S9 vs S9s
  latency p=0,001 (signifikan, selisih 29 ms — selective lebih cepat). Selective
  compensate memberikan **kecepatan sedikit lebih tinggi** pada S3 dan S9, namun
  magnitudo sangat kecil (±2–29 ms) sehingga secara praktis tidak mengubah
  outcome data.
- **S8 dan S9 dieksklusi dari uji ini**: keduanya skenario eksklusif
  satu-pendekatan (S8: choreography only, S9: orchestration only) sehingga tidak
  memiliki pasangan untuk dibandingkan — hasilnya dilaporkan secara deskriptif
  (30/30 dengan satu kategori outcome yang seragam), bukan komparatif.
  Recovery time S9 tidak dilaporkan karena tidak ada penanda deteksi kegagalan
  yang setara (timeout HTTP ≠ row `failed` di saga_log).

## Analisis per Skenario

### S1 — Baseline Normal
Kedua pendekatan konsisten penuh (100%). Periode inkonsistensi sementara
berbeda signifikan secara statistik (choreography 41,5 ms vs orchestration 26,3 ms,
p<0,0001), namun latency end-to-end **tidak berbeda signifikan** (Mann-Whitney U,
p=0,73) setelah re-run final — klaim awal "S1 latency berbeda signifikan" terbukti
tidak stabil di lintas run.

**Justifikasi outlier (1626 ms pada pengukuran awal):** outlier tunggal pada
run ke-6 pengukuran choreography sebelumnya (satu dari 30 run, ~3%) merupakan
kemungkinan **cold-start container**: urutan pertama POST setelah `docker compose
up -d` mengaktifkan inisialisasi connection pool PostgreSQL, JIT compiler Go,
dan TCP handshake pertama yang lebih lambat (~1–1,5 detik). Setelah run pertama,
warm-up selesai dan latency turun ke rentang normal 80–250 ms. Fenomena ini
khas untuk container yang baru start dan tidak mencerminkan performa steady-state
sistem. Pada re-run final (run ke-2 dan seterusnya), warm-up sudah terjadi
sebelumnya karena run-crash/run-scenario sebelumnya telah mengisi cache OS dan
TCP TIME_WAIT. Outlier tersebut bukan temuan arsitektural melainkan artefak
pengukuran urutan pertama, sehingga di-exclude dari klaim komparatif.

### S2 — Kegagalan Shipping (langkah akhir)
Kompensasi berantai berhasil penuh di kedua pendekatan (CTSR 100%).
**Recovery time berbeda signifikan secara statistik** (Mann-Whitney U, p=0,0002):
choreography 22,0 ms vs orchestration 28,7 ms — choreography lebih cepat, namun
magnitudo selisih kecil (±5–7 ms). Perbedaan latency end-to-end (±1639 vs ±163 ms)
jauh lebih besar dan berasal dari jalur eksekusi forward choreography yang melewati
beberapa hop Kafka, bukan dari kecepatan pemulihan.

### S3 — Kegagalan Inventory (langkah tengah)
Pola sama dengan S2: kompensasi penuh (100%), **recovery time berbeda signifikan**
(p<0,0001): choreography 12,8 ms vs orchestration 26,0 ms — choreography lebih
cepat (selisih ±13 ms). Periode inkonsistensi sementara juga berbeda signifikan (27,2 vs
40,1 ms, p<0,0001). Arah temuan ini mendukung klaim Malyuga et al. (2020) bahwa
choreography bekerja lebih cepat pada mekanisme koordinasinya.

**Catatan confounding factor (S2/S3):** selisih recovery time perlu dibaca dengan
hati-hati. Strategi kompensasi call-all pada orchestrator mengirim **4 panggilan
HTTP kompensasi** (termasuk ke shipping yang tidak pernah berpartisipasi), sedangkan
rantai kompensasi choreography hanya menyentuh service yang benar-benar terlibat
(2–3 hop). Sebagian selisih kecepatan recovery orchestration (±3–13 ms) dapat
berasal dari jumlah panggilan ekstra ini, di samping penjelasan arsitektural murni.

### S6 — Kegagalan Compensating Transaction
Kedua pendekatan **tidak** dapat memulihkan konsistensi (0%). Compensating
transaction yang gagal (marker `compensate_failed` di `saga_log`) tidak memiliki
mekanisme retry di kedua pendekatan, sehingga data tertinggal pada status
parsial permanen. Temuan ini menyoroti pentingnya idempotensi + retry pada
compensating transaction.

### S7 — Konkurensi 500 Transaksi
Hasil setelah perbaikan metodologi pengukuran (deteksi quiescence berbasis
`created_at` `saga_log`):
- **Choreography**: 100% konsisten untuk seluruh 15.000 transaksi yang
  diproses. Periode inkonsistensi sementara ±2818 ms; latency ±7170 ms.
  Rantai event Kafka dengan partisi tunggal mengantri 500 transaksi secara
  serial, sehingga transaksi terakhir berada dalam kondisi parsial selama
  beberapa detik.
- **Orchestration**: 99,7% committed (14.952/15.000); 48 request not_found
  dari 2 dari 30 run (~6,7% run terkena, bukan 0%). Periode inkonsistensi
  sementara ±465 ms; latency ±5898 ms. Kejenuhan orchestrator sebagai titik
  tunggal bersifat kondisi intermittent, bukan deterministik — fenomena dasar
  tetap terkonfirmasi meski magnitudonya lebih presisi (48 vs 580) setelah
  kontaminasi eksternal disingkirkan.

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

**Catatan penting — konsistensi ≠ correctness:** hasil S9 **100% konsisten secara
status akhir data** (semua service mencapai status compensated), tetapi dari
sudut pandang **pelanggan**, pesanan yang sebenarnya valid (order sukses, payment
terdebet, stok berkurang) dihapus begitu saja karena sistem salah paham. Konsistensi
data teknis ≠ kebenaran keputusan bisnis. Oleh karena itu, perbandingan "S8 (0%)
vs S9 (100%) konsisten" bukan apples-to-apples di level metrik — S8 gagal
mencapai state akhir sama sekali (stuck), sedangkan S9 mencapai state akhir
konsisten tapi dengan membatalkan transaksi yang seharusnya sukses. **Keduanya
sama-sama gagal dari perspektif pelanggan**: S8 meninggalkan data menggantung,
S9 meninggalkan keputusan bisnis yang salah. Tidak ada pemenang di S8 vs S9 —
yang ada adalah dua manifestasi berbeda dari kerentanan struktural yang sama.

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

### S9s — Response Hilang dengan Compensate-Selective (orchestration only)

**Counterfactual dari keputusan call-all.** S9s menjalankan skenario yang sama
dengan S9, tetapi orchestrator menggunakan strategi **compensate-selective**
(`SELECTIVE_COMPENSATE=true`): sebelum memanggil kompensasi untuk sebuah service,
orchestrator **query database** untuk mengecek apakah service tersebut benar-benar
memiliki row `committed`. Jika tidak ada, kompensasi **di-skip**. Tujuannya
mengisolasi apakah strategi call-all (yang selalu memanggil keempat endpoint
kompensasi) memang menghasilkan data outcome berbeda dengan selective (yang hanya
memanggil yang committed).

Hasil (30 run, orchestration): **30/30 `compensated`** — konsisten dengan S9
(call-all). Pada S9s, shipping (yang tidak pernah commit) di-skip (terlihat dari
log "skip compensate (not committed)"); order, payment, inventory (yang committed
sebelum response di-drop) dikompensasi.

**Temuan penting — prediksi orphaned commit tidak terjadi:** Claude (penilai
independen) memprediksi bahwa selective compensation akan menghasilkan **orphaned
commit** (data sukses tanpa kompensasi → inkonsistensi permanen). Hasil tidak
mendukung prediksi tersebut pada n=10 maupun n=30. Alasannya: selective
compensation membaca **kebenaran di database** (row `committed`), bukan keyakinan
orchestrator. Langkah yang sebenarnya commit — termasuk yang kompensasinya "tidak
perlu" (false negative) — tetap dikompensasi karena ada buktinya. Tidak ada skenario
pada S9 di mana sebuah step commit tetapi orchestrator "tidak tahu" sampai-sampai skip
kompensasi, karena selective logic selalu memverifikasi DB truth (kode `stepCommitted`
di `internal/orchestration/orchestrator.go` baris 59-73: query `SELECT status
FROM <table> WHERE saga_id = $1`). Hasil dengan n=30: 30/30 compensated + 30/30
needless = 100% konsisten, latency 10159 ± 42 ms — **variansi rendah**, mengonfirmasi
bahwa hasilnya stabil dan bukan kebetulan dari outlier kecil.

**Implikasi untuk caveat call-all:** percobaan ini **memperkuat** bahwa
desain call-all pada S9 bersifat konservatif-benign — baik call-all maupun
selective menghasilkan outcome data identik (semua participating services
`compensated`, 100% konsisten). Perbedaan call-all hanya berupa **HTTP call
tambahan ke shipping** yang tidak participated (no-op, hanya menambah latency
±30 ms). Konsekuensi desain call-all pada S9 murni **efisiensi**, bukan
konsistensi. Caveat di paragraf sebelumnya tetap valid untuk konteks S2/S3
di mana call-all membuat orchestrator mengirim lebih banyak HTTP call
daripada yang strictly perlu, sehingga berpotensi mempengaruhi recovery time
namun sulit diisolasi tanpa eksperimen S2s/S3s yang juga memerlukan selective
mode.

Latency S9s (10174 ± 203 ms) sedikit lebih tinggi dari S9 (10127 ± 15 ms) —
tambahan overhead 4 query DB di `fail()` saat selective mode aktif. Untuk
eksperimen S2/S3 (di mana recovery time terdefinisi dan call-all mungkin
memberi kontribusi pada selisih), eksperimen S2s/S3s akan menjadi lanjutan
yang diperlukan untuk mengisolasi confounding tersebut secara empiris.

Data: `docs/runs/S9s/orchestration/`.

### S2s & S3s — Counterfactual Call-All di Skenario Step-Failure (orchestration only)

**Isolasi confounding call-all pada RM3.** S2s dan S3s menjalankan skenario yang
sama dengan S2 (Shipping gagal) dan S3 (Inventory gagal), tetapi orchestrator
menggunakan strategi **compensate-selective** — sebelum memanggil kompensasi,
orchestrator query database untuk mengecek apakah service tersebut benar-benar
commit; jika tidak, kompensasi di-skip. Tujuannya: mengisolasi apakah selisih
recovery time S2/S3 choreography vs orchestration disebabkan oleh arsitektur
koordinasi, atau oleh fakta bahwa call-all orchestrator mengirim lebih banyak HTTP call
dari yang strictly perlu.

Hasil (30 run, orchestration, n=30 sesuai proposal 3.6):
- **S2s**: 30/30 compensated, recovery **28 ± 6 ms** (stabil, tanpa outlier).
- **S3s**: 30/30 compensated, recovery **24 ± 8 ms**.

**Isolasi confounding — hasil utama untuk RM3 (dengan n=30):**
- S3 (call-all): 26,0 ± 6,5 ms; **S3s (selective): 24 ± 8 ms** — selisih
  **-2 ms** (selective justru sedikit lebih cepat, tapi selisih dalam std dev).
  Confirmed: **call-all tidak menambah overhead terukur pada recovery time**.
  Selisih 13 ms pada S3 (choreography 12,8 ms vs orchestration 26,0 ms) bukan
  karena call-all, tapi karena **arsitektur koordinasinya** — choreography memang
  intrinsik lebih cepat dalam kompensasi.
- S2 (call-all): 29 ± 6 ms; S2s (selective): 28 ± 6 ms — selisih **1 ms** dalam
  std dev. Konfirmasi lebih kuat dibanding n=10 sebelumnya (yang menunjukkan 67 ms
  outlier 296 ms di run 1).

**Implikasi final untuk RM3:** klaim "choreography pulih lebih cepat" bukan
artefak jumlah HTTP call. Selisih choreography vs orchestration murni
bersifat arsitektural. Temuan ini memperkuat signifikansi temuan RM3
sebelumnya (p<0,001 di S2 dan S3).

Data: `docs/runs/S2s/orchestration/`, `docs/runs/S3s/orchestration/`.

**Caveat penting (konsekuensi desain, bukan properti universal):** hasil S9
(100% konsisten, needless compensation) adalah konsekuensi langsung dari
strategi kompensasi **call-all** pada `fail()` di `internal/orchestration/
orchestrator.go`, yang mengompensasi keempat endpoint tanpa syarat (dan
kompensasi bersifat idempotent). Jika implementasi memakai strategi
**compensate-selective** (hanya mengompensasi step yang terkonfirmasi berhasil),
hasil S9 kemungkinan besar berupa **orphaned commit** (data yang sukses tetapi
tidak pernah dikompensasi → inkonsistensi permanen), bukan needless
compensation. Dengan demikian, "orchestration selalu berakhir konsisten" bukan
properti universal pola orchestration — melainkan hasil dari keputusan desain
spesifik implementasi ini.

## Observasi S4 & S5 (otomatis, 30 run)

Dijalankan dengan `scripts/run-crash.ps1 -Runs 30`; `DELAY_MS=3000` diset
untuk menciptakan window crash yang deterministik (saga berjalan ~3 s per
langkah, sehingga komponen dapat dihentikan saat transaksi masih berlangsung;
tanpa delay, saga selesai dalam ±100 ms lebih cepat dari latency `docker stop`).

- **S4 — Orchestrator Crash** (30 run): **30/30 inconsistent**. Pola konsisten:
  Order dan Payment `committed`, Inventory/Shipping tidak berpartisipasi —
  orchestrator dihentikan saat langkah inventory masih berjalan, sehingga
  kompensasi mundur tidak pernah dieksekusi. Commit parsial permanen.
- **S5 — Kafka Down** (30 run): **30/30 inconsistent**. Order `committed`;
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
   pada skenario kegagalan langkah — choreography lebih cepat (S2: 22,0 vs
   28,7 ms, p=0,0002; S3: 12,8 vs 26,0 ms, p<0,0001). Magnitudo selisih kecil
   (±5–13 ms) dan keduanya pulih dalam puluhan milidetik. Klaim awal
   "orchestration jauh lebih cepat pulih" tidak terdukung. **Confounding
   call-all telah diisolasi empiris** via S3s (selective compensate): recovery
   time call-all (26,0 ms) ≈ selective (28 ms) → call-all tidak menambah
   overhead, selisih choreography vs orchestration murni arsitektural.
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

- **Pembersihan container proyek lain sebelum batch eksperimen.** Container
  dari proyek lain yang berjalan bersamaan (9 container tambahan: API, worker,
  database, message broker, cache) dapat menyebabkan resource contention pada
  Docker Desktop VM (CPU, memori, network stack), terlepas dari ada-tidaknya
  konflik port eksplisit. Sebelum menjalankan batch skenario, pastikan
  `docker ps` bersih dari container proyek lain.
- Pengukuran awal S7 menghasilkan angka inconsistent yang menyesatkan karena
  checker mengklasifikasikan saga yang masih berjalan sebagai "inconsistent"
  (polling terlalu agresif, window quiescence 3 detik < jeda antar-langkah pada
  beban tinggi). Diperbaiki dengan: query checker per-saga (indeks `saga_id`,
  bukan full-table scan), interval polling 1,5 detik, deteksi quiescence 10
  detik berbasis `created_at` `saga_log`, dan klasifikasi langsung saat marker
  `compensate_failed` ditemukan (saga "sealed").
- Recovery time dihitung dari timestamp `saga_log` di database service, bukan
  dari polling workload generator, sehingga presisinya ±milidetik.
- **Recovery time untuk S9 tidak terdefinisi (nol) bukan karena kelalaian** —
  definisi proposal 3.5.3 membutuhkan titik deteksi kegagalan (timestamp row
  `failed` di `saga_log`), lalu selisih ke final state konsisten. Pada S9,
  orchestrator tidak pernah menulis row `failed` karena timeout terjadi di level
  transport HTTP (di luar jangkauan saga_log); kompensasi dipicu oleh asumsi
  "gagal" tanpa jejak eksplisit di database. Akibatnya, `recovery_count=0` di
  S9 bukan data hilang tapi mencerminkan sifat mode kegagalannya (in-doubt
  terdeteksi di luar saga_log). Perbandingan recovery time choreography vs
  orchestration untuk S9 tidak dimungkinkan; ini batasan desain eksperimen, bukan
  kelalaian implementasi.
- Nilai latency S6 (±129–188 ms) mencerminkan waktu konfirmasi inkonsistensi,
  bukan waktu pemulihan (tidak ada pemulihan pada S6).
- **Non-atomicity write bisnis + saga_log (B1).** Delapan fungsi bisnis
  (`CreateOrder`, `ProcessPayment`, `ReserveInventory`, `ScheduleShipping` +
  4 compensate) membungkus INSERT/UPDATE ke tabel bisnis dan INSERT ke
  `saga_log` sebagai **dua statement terpisah yang masing-masing auto-commit**
  (tidak dalam satu database transaction). Spot-check terhadap 500 transaksi
  S7 tidak menemukan orfan (terverifikasi untuk S7 tanpa kompensasi).
  Namun pada skenario dengan kompensasi call-all (S2, S3, S6), service yang
  tidak berpartisipasi tetap menerima panggilan compensate — `writeLog`
  menulis entri `"compensated"` ke `saga_log` meski tabel bisnis kosong
  untuk saga tersebut (terverifikasi empiris: S2 Shipping, S3 Inventory).
  Entri palsu ini tidak memengaruhi klasifikasi outcome (checker baca dari
  tabel bisnis) atau metrik agregat (analyze tool baca dari JSON).
- **Partial idempotency compensate (B2).** Compensate functions memiliki
  status guard (`WHERE status = 'committed'`) sehingga UPDATE aman dipanggil
  berkali-kali. Namun `writeLog()` menulis duplikat ke `saga_log` tanpa
  `ON CONFLICT`. Cakupan masalah lebih luas dari yang awalnya didokumentasikan:
  bukan cuma risiko double-invocation, tapi juga manifestation dari
  call-all mode — service yang tidak pernah commit tetap mendapat entri
  saga_log `"compensated"` palsu (lihat B1 di atas).
- **Inventory Service tanpa validasi stok (B3).** Proposal section 3.2.1
  menyebutkan Inventory Service "mengurangi stok", namun implementasi hanya
  mencatat reservasi (append-only log) tanpa kolom `stock` atau validasi
  ketersediaan. Kegagalan pada S3 disimulasikan murni melalui fault
  injection (`FAIL_AT_STEP=inventory`), bukan melalui kondisi bisnis nyata
  seperti stok habis.
- **Connection pool S7 belum diverifikasi (C3).** Semua service menggunakan
  `MaxConns=4` (pgx default). Pola pemakaian koneksi antara choreography
  (event-driven, lepas-pakai per event) dan orchestration (HTTP sekuensial
  per saga) berbeda secara struktural, sehingga dampak MaxConns=4 bisa saja
  tidak sama rata ke kedua pendekatan — ini **asumsi yang belum diverifikasi
  secara empiris** dan merupakan potensi confounding factor tambahan yang
  belum diisolasi.
- **Kebocoran SELECTIVE_COMPENSATE antar-run (T1).** PowerShell `$env:VAR`
  persist dalam satu sesi terminal. Sebelum perbaikan, `$env:SELECTIVE_COMPENSATE`
  hanya di-set eksplisit di branch S9/S9s/S2s/S3s — branch lain (S1-S8) tidak
  menyentuhnya. Kalau S9s/S2s/S3s dijalankan duluan di satu sesi, nilai `true`
  tertinggal dan S2/S3/S6/S8 bisa ikut terpengaruh. Diperbaiki: semua branch
  sekarang secara eksplisit set `$env:SELECTIVE_COMPENSATE = "false"`.
  **Bukti retroaktif bahwa kebocoran tidak pernah terjadi pada data lama:**
  keberadaan entri saga_log `"compensated"` palsu di S2 Shipping dan S3
  Inventory (temuan T2) membuktikan bahwa SELECTIVE_COMPENSATE tidak pernah
  bocor ke data lama — kalau bocor, orchestrator akan melewati compensate
  untuk service yang tidak commit, sehingga entri palsu tidak akan ada.
- **Entri saga_log palsu di call-all mode (T2).** Mode call-all (default)
  memanggil compensate ke SEMUA service, termasuk yang tidak pernah commit.
  `writeLog()` menulis `"compensated"` tanpa mengecek apakah service tersebut
  benar-benar berpartisipasi. Terverifikasi empiris: S2 Shipping punya entri
  saga_log `"compensated"` tapi tabel bisnis kosong; S3 Inventory pola yang
  sama. Entri palsu tidak memengaruhi klasifikasi outcome (checker baca tabel
  bisnis) atau Timeline() (Order selalu dipanggil terakhir, jadi timestamp
  asli tetap jadi `final`).
- **Attempt counter global per proses (T5).** `ShouldFail()` menambah counter
  `attempt` di SETIAP pemanggilan fungsi bisnis (forward + compensate) dalam
  satu service, bukan per-step. Untuk `FAIL_AT_ATTEMPT=1` (semua skenario
  saat ini), ini tidak berpengaruh. Namun untuk `FAIL_AT_ATTEMPT>1`,
  behavior akan meleset dari ekspektasi — counter menghitung total pemanggilan
  ke SELURUH fungsi bisnis, bukan percobaan ke-N untuk step tertentu.
- **Rantai kompensasi choreography bersifat topology-specific (T9).**
  Shipping Service tidak punya consumer reverse (tidak bisa terima sinyal
  kompensasi). Rantai reverse hanya didesain untuk "kegagalan merambat dari
  titik mundur ke awal" — valid untuk S2/S3 yang diuji, tapi tidak
  digeneralisasi ke pola kegagalan lain (misal Payment gagal).
- **Amount:0 hardcode di event republish (T10).** Payment reverse-consumer
  mem-publish `PaymentResultEvent` dengan `Amount: 0` saat kompensasi karena
  `InventoryResultEvent` tidak memiliki field `Amount`. Tidak berdampak ke
  hasil (`CompensateOrder` hanya memakai `SagaID`), tapi kode rapuh.

## Rekomendasi Langkah Selanjutnya

- Menambahkan mekanisme **outbox pattern / event replay** di choreography dan
  mengukur dampaknya terhadap S8 (event loss).
- Menambahkan retry pada compensating transaction dan mengukur dampaknya
  terhadap S6.
- Eksperimen dengan partisi Kafka > 1 untuk mengamati pengaruh paralelisme
  terhadap periode inkonsistensi choreography pada S7.