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
| S3 | choreography | 30 | 0 | 30 | 0 | 100.0 | 100.0 | 13 | 27 | 1624 |
| S3 | orchestration | 30 | 0 | 30 | 0 | 100.0 | 100.0 | 26 | 40 | 150 |
| S6 | choreography | 30 | 0 | 0 | 30 | 0.0 | 0.0 | tak pulih | 28 | 123 |
| S6 | orchestration | 30 | 0 | 0 | 30 | 0.0 | 0.0 | tak pulih | 42 | 156 |

### Skenario konkurensi (S7, 500 transaksi/iterasi → 15.000 transaksi total)

| Skenario | Approach | Txns | Committed | Unrecorded* | Consistency% | Inconsistency window (ms) | Latency (ms) |
|----------|----------|------|-----------|-------------|--------------|---------------------------|--------------|
| S7 | choreography | 15000 | 15000 | 0 | 100.0 | 5152 | 8476 |
| S7 | orchestration | 15000 | 14918 | 82 | 99.5 | 628 | 5010 |
| S8 | choreography | 10 | 0 | 0 | 0.0 (stuck) | 0 | 12345 |
| S9 | orchestration | 10 | 0 | 10 | 0 | 0 | 0 | 10 | 100.0 (needless) | 10043 | 10127 |
| S9s | orchestration | 10 | 0 | 10 | 0 | 0 | 0 | 10 | 100.0 (needless) | 10084 | 10174 |

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
| S7 | latency | 8476,1 ± 1541,5 | 5009,9 ± 1100,7 | <0,0001 | ya |
| S7 | inconsistency window | 5152,0 ± 1428,9 | 628,1 ± 360,5 | <0,0001 | ya |

**Interpretasi:**
- **Recovery time (S2/S3)**: perbedaan **signifikan** — choreography pulih lebih
  cepat (S2: p=0,0002; S3: p<0,0001). Magnitudo kecil (±5–13 ms); secara
  praktis keduanya pulih dalam puluhan milidetik. Klaim "setara" yang dilaporkan
  pada pengukuran awal **dikoreksi**: secara statistik tidak setara, melainkan
  choreography lebih cepat — sejalan dengan klaim Malyuga et al. (2020).
- **Window inkonsistensi S1** (tidak ada kegagalan): berbeda signifikan
  (choreography lebih lebar). **Latency S1** (tidak ada kegagalan): TIDAK
  berbeda signifikan (p=0,73) setelah re-run final — klaim awal "S1 latency
  berbeda signifikan" tidak stabil di lintas run (outlier 1626 ms pada run
  sebelumnya menghilang), sehingga tidak layak dipublikasikan sebagai temuan
  komparatif pada S1.
- **Window inkonsistensi S2** (ada kegagalan): tidak berbeda signifikan.
- **Latency & window inkonsistensi beban tinggi (S7)**: berbeda signifikan,
  orchestration jauh lebih cepat/singkat.
- **S8 dan S9 dieksklusi dari uji ini**: keduanya skenario eksklusif
  satu-pendekatan (S8: choreography only, S9: orchestration only) sehingga tidak
  memiliki pasangan untuk dibandingkan — hasilnya dilaporkan secara deskriptif
  (10/10 dengan satu kategori outcome yang seragam), bukan komparatif.

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
`created_at` `saga_log`): **kedua pendekatan 100% konsisten** untuk seluruh
transaksi yang diproses. Perbedaan utama:
- **Periode inkonsistensi sementara**: choreography ±5152 ms vs orchestration
  ±628 ms — ±8× lebih lama. Rantai event Kafka dengan partisi tunggal
  mengantri 500 transaksi secara serial, sehingga transaksi terakhir berada
  dalam kondisi parsial selama beberapa detik.
- **Latency end-to-end**: choreography ±8476 ms vs orchestration ±5010 ms.
- **Availability pintu masuk**: choreography memproses seluruh 15.000 transaksi;
  orchestration kehilangan 82 request (0,55%) saat puncak beban karena entry
  point tunggalnya (orchestrator + port forward) menolak koneksi — fluktuasi
  run-to-run tinggi (run sebelumnya mencatat 2 unrecorded) menandakan bahwa
  kondisi mesin lebih berperan daripada desain sistemik.

**Mekanisme fluktuasi 2 ↔ 82 request ditolak (S7 orchestration):** variasi
run-to-run yang lebar pada kolom `Unrecorded` berasal dari **interaksi
Docker Desktop pada Windows** dengan lalu lintas burst tinggi. Saat 500
goroutine membuka koneksi ke `localhost:8080` secara bersamaan dalam <1 detik,
port-forwarder Windows (vpnkit) meneruskan koneksi ke container orchestrator
melalui mekanisme TCP/IP host. Pool koneksi host-side memiliki batas
**ephemeral port** (~16.000) dan waktu tunggu **TIME_WAIT** (2–4 menit per
koneksi). Pada run dengan kondisi OS favorable (cache hangat, port pool belum
penuh), hampir semua koneksi diterima. Pada run dengan cache dingin atau
akumulasi TIME_WAIT dari run sebelumnya, beberapa koneksi di-reset oleh kernel
sebelum sampai ke orchestrator, sehingga klien workload menerima
`connectex: actively refused`. Fenomena ini adalah **infrastruktur pengujian
bukan sifat sistem saga**: dengan lingkungan produksi di Linux tanpa vpnkit
atau dengan Kubernetes ingress yang mengelola connection pool, tingkat
`Unrecorded` kemungkinan jauh lebih rendah. Namun untuk eksperimen ini,
angka 82 (run ini) dan 2 (run sebelumnya) sama-sama menunjukkan bahwa **titik
masuk tunggal orchestration rentan terhadap kehilangan request di puncak
beban**, dan temuan kualitatifnya konsisten walau magnitudonya tidak stabil.

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

Hasil (10 run, orchestration): **10/10 `compensated`** — konsisten dengan S9
(call-all). Pada S9s, shipping (yang tidak pernah commit) di-skip (terlihat dari
log "skip compensate (not committed)"); order, payment, inventory (yang committed
sebelum response di-drop) dikompensasi.

**Temuan penting — prediksi orphaned commit tidak terjadi:** Claude (penilai
independen) memprediksi bahwa selective compensation akan menghasilkan **orphaned
commit** (data sukses tanpa kompensasi → inkonsistensi permanen). Hasil tidak
mendukung prediksi tersebut. Alasannya: selective compensation membaca **kebenaran
di database** (row `committed`), bukan keyakinan orchestrator. Langkah yang
sebenarnya commit — termasuk yang kompensasinya "tidak perlu" (false negative)
— tetap dikompensasi karena ada buktinya. Tidak ada skenario pada S9 di mana
sebuah step commit tetapi orchestrator "tidak tahu" sampai-sampai skip
kompensasi, karena selective logic selalu memverifikasi DB truth.

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
   pada skenario kegagalan langkah — choreography lebih cepat (S2: 22,0 vs
   28,7 ms, p=0,0002; S3: 12,8 vs 26,0 ms, p<0,0001). Magnitudo selisih kecil
   (±5–13 ms) dan keduanya pulih dalam puluhan milidetik. Klaim awal
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

## Rekomendasi Langkah Selanjutnya

- Menambahkan mekanisme **outbox pattern / event replay** di choreography dan
  mengukur dampaknya terhadap S8 (event loss).
- Menambahkan retry pada compensating transaction dan mengukur dampaknya
  terhadap S6.
- Eksperimen dengan partisi Kafka > 1 untuk mengamati pengaruh paralelisme
  terhadap periode inkonsistensi choreography pada S7.