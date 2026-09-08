# Prompt Review Independen — Skripsi Saga Pattern (Choreography vs Orchestration)

> Cara pakai: salin seluruh isi file ini ke AI (Claude/ChatGPT/Gemini) sebagai satu pesan,
> lalu minta jawaban atas pertanyaan-pertanyaan di bagian akhir. Tidak perlu konteks tambahan —
> semua informasi sudah lengkap di prompt ini.

---

Anda adalah penilai akademik senior (pembimbing/penguji skripsi bidang rekayasa perangkat
lunak dan sistem terdistribusi). Evaluasilah proposal dan hasil penelitian berikut secara
kritis, jujur, dan objektif. Jangan menyenangkan penulis — tujuannya mencari kelemahan dan
saran perbaikan yang nyata.

## KONTEKS PENELITIAN

- **Judul**: Analisis Perbandingan Konsistensi Data Transaksi pada Saga Pattern (Choreography
  vs Orchestration) menggunakan Fault Injection Testing di Arsitektur Microservices Containerized
- **Jenjang**: Skripsi S1 (Sarjana), Fakultas Ilmu Komputer, Universitas Brawijaya
- **Bidang**: Rekayasa Perangkat Lunak / Sistem Terdistribusi
- **Objek**: Sistem e-commerce sederhana (4 service: Order, Payment, Inventory, Shipping),
  masing-masing dengan database PostgreSQL terpisah (database-per-service)
- **Teknologi**: Go, Apache Kafka (choreography), Redis (state orchestration), HTTP
  request-reply (orchestration), Docker Compose, PostgreSQL 15

## RUMUSAN MASALAH (persis dari proposal)

1. **RM1**: Bagaimana dampak skenario kegagalan terhadap konsistensi data transaksi antar
   service pada pendekatan choreography dan orchestration dalam saga pattern?
2. **RM2**: Bagaimana perbandingan compensating transaction success rate antara pendekatan
   choreography dan orchestration pada saga pattern dalam berbagai skenario kegagalan?
3. **RM3**: Bagaimana perbandingan recovery time antara kedua pendekatan tersebut pada
   masing-masing skenario kegagalan yang diujikan?

## TUJUAN PENELITIAN (persis dari proposal)

1. Mengevaluasi dampak skenario kegagalan terhadap konsistensi data transaksi antar service
   pada pendekatan choreography dan orchestration dalam saga pattern.
2. Menganalisis perbandingan compensating transaction success rate antara pendekatan
   choreography dan orchestration pada saga pattern dalam berbagai skenario kegagalan.
3. Menganalisis perbandingan recovery time antara pendekatan choreography dan orchestration
   pada masing-masing skenario kegagalan yang diujikan.

## DESAIN PENELITIAN

- **Arsitektur**: 4 service identik kode bisnisnya; hanya mekanisme koordinasi yang berbeda:
  choreography (event via Kafka, 8 topik, partisi tunggal) vs orchestration (orchestrator
  pusat memanggil service via HTTP, state saga di Redis). Kode bisnis 100% sama untuk kedua
  pendekatan (hanya env var APPROACH yang berbeda) — eksperimen adil.
- **Fault injection**: non-intrusive via environment variable container: FAIL_AT_STEP
  (service mana yang gagal), FAIL_AT_ATTEMPT, DELAY_MS, FAIL_ON_COMPENSATE. Tidak ada
  perubahan kode antar skenario.
- **Skenario** (masing-masing 30 iterasi, reset total antar iterasi):
  - S1 Baseline Normal (tanpa kegagalan) — keduanya
  - S2 Kegagalan langkah akhir (Shipping) — keduanya
  - S3 Kegagalan langkah tengah (Inventory) — keduanya
  - S4 Orchestrator Crash (container dihentikan di tengah saga) — orchestration saja
  - S5 Kafka Down (container dihentikan saat rantai event) — choreography saja
  - S6 Kegagalan Compensating Transaction (FAIL_ON_COMPENSATE=true) — keduanya
  - S7 Konkurensi 500 transaksi bersamaan (15.000 transaksi total) — keduanya
  - S8 Event Loss Parsial (event order.created sengaja di-drop di titik publish,
    broker tetap hidup; DROP_EVENT, one-shot) — choreography saja
- **Metrik** (sesuai proposal 3.5):
  - Consistency Rate: % transaksi berakhir konsisten (semua committed ATAU semua compensated)
  - Compensating Transaction Success Rate (CTSR): % kompensasi berhasil
  - Recovery Time: dari deteksi kegagalan (timestamp row "failed" di saga_log) sampai
    final state konsisten (timestamp row terakhir di saga_log) — diukur langsung dari
    timestamp database, bukan proksi
  - Metrik tambahan: periode inkonsistensi sementara (saga_log pertama → terakhir),
    latency end-to-end, statistik deskriptif mean/min/max/std
- **Penentuan outcome**: consistency checker membaca 4 database, klasifikasi: committed /
  compensated / inconsistent / not_found. Deteksi "saga selesai" berbasis quiescence
  (saga_log berhenti bertambah ≥10 detik) atau marker compensate_failed (sealed).

## HIPOTESIS AWAL (diturunkan dari literatur: Megargel 2021, Malyuga 2020, Lee 2023,
Daraghmi 2022, Talaver & Vakaliuk 2023, Munonye & Martinek 2020, Laigner 2021,
Fan 2020, Baão & Guerreiro 2026, Chen 2024, Yu 2025)

1. **H1**: Kedua pendekatan 100% konsisten saat kompensasi berjalan normal (S1/S2/S3).
2. **H2**: Orchestration recovery time LEBIH CEPAT dari choreography (kontrol terpusat,
   panggilan langsung vs rantai event Kafka) — ini klaim umum di literatur.
3. **H3**: CTSR orchestration ≥ choreography (kompensasi terpusat lebih andal daripada
   propagasi event berantai).
4. **H4**: S6 (kompensasi gagal): keduanya 0% — batas fundamental saga (literatur: kompensasi
   diasumsikan selalu sukses; Munonye 2020a, Talaver 2023, Baão 2026).
5. **H5**: S7 (beban tinggi): choreography berisiko kehilangan event → konsistensi lebih
   rendah dari orchestration (Megargel: replikasi data; Laigner: non-transactional queuing).
6. **H6**: S4/S5 (komponen koordinasi mati): commit parsial permanen di pendekatan
   masing-masing (orchestrator/broker = titik kritis; Malyuga, Lee, Baão).

## HASIL FAKTUAL (30 iterasi per skenario per pendekatan)

### Tabel A — Skenario 1 transaksi

| Skenario | Approach | Consistency% | CTSR% | Recovery (ms) | Window inkonsistensi (ms) | Latency (ms) |
|----------|----------|--------------|-------|---------------|---------------------------|--------------|
| S1 | choreography | 100 | — | — | 31 | 165 |
| S1 | orchestration | 100 | — | — | 31 | 154 |
| S2 | choreography | 100 | 100 | 22 | 48 | 1639 |
| S2 | orchestration | 100 | 100 | 25 | 49 | 160 |
| S3 | choreography | 100 | 100 | 18 | 37 | 1670 |
| S3 | orchestration | 100 | 100 | 31 | 49 | 173 |
| S6 | choreography | 0 | 0 | tak pulih | 25 | 129 |
| S6 | orchestration | 0 | 0 | tak pulih | 50 | 188 |

### Tabel B — S7 (konkurensi 500/iterasi, 15.000 transaksi total)

| Skenario | Approach | Txns | Committed | Request ditolak | Consistency% (dari yg diproses) | Window inkonsistensi (ms) | Latency (ms) |
|----------|----------|------|-----------|-----------------|----------------------------------|---------------------------|--------------|
| S7 | choreography | 15000 | 15000 | 0 | 100 | 6238 | 9722 |
| S7 | orchestration | 15000 | 14998 | 2 | 100 | 864 | 5241 |

### Tabel C — S4/S5 (otomatis, 10 run)

| Skenario | Approach | Hasil |
|----------|----------|-------|
| S4 Orchestrator Crash | orchestration | 10/10 inconsistent — Order+Payment committed, Inventory/Shipping tidak, kompensasi tidak pernah jalan |
| S5 Kafka Down | choreography | 10/10 inconsistent — Order committed (Payment kadang ikut), Inventory/Shipping tidak, saga macet |

### Ringkasan hasil vs hipotesis

| Hipotesis | Hasil | Verdict |
|-----------|-------|---------|
| H1: keduanya konsisten di S1/S2/S3 | 100% keduanya | Terkonfirmasi |
| H2: orchestration pulih lebih cepat | Setara (22 vs 25 ms; 18 vs 31 ms) | **Terbantah** |
| H3: CTSR orchestration ≥ | Setara (100/100 di S2/S3, 0/0 di S6) | Tidak terdukung (setara) |
| H4: S6 keduanya 0% | 0% keduanya | Terkonfirmasi |
| H5: S7 choreography < orchestration | Keduanya 100% dari yang diproses; orchestration justru menolak 2 request | **Terbalik arah** |
| H6: S4/S5 commit parsial | 10/10 inconsistent keduanya | Terkonfirmasi |

### Interpretasi penulis saat ini

1. Konsistensi akhir (RM1) dan CTSR (RM2): **setara dalam kondisi normal** —
   perbedaan tidak muncul di "apakah konsisten", melainkan di karakteristik
   operasional; **berbeda pada mode kegagalan event loss (S8)** yang hanya
   mungkin terjadi di choreography (saga stuck permanen, 10/10).
2. Recovery time (RM3): **berbeda signifikan secara statistik** (Mann-Whitney U,
   n=30): choreography lebih cepat (S2: 22,5 vs 25,3 ms, p<0,001; S3: 18,0 vs
   31,1 ms, p<0,0001), magnitudo kecil. Klaim umum "orchestration jauh lebih
   cepat pulih" tidak terdukung saat recovery diukur dari deteksi kegagalan,
   bukan dari latency request. Perbedaan latency end-to-end (1639 vs 160 ms)
   berasal dari jalur eksekusi maju (hop Kafka), bukan dari mekanisme pemulihan.
3. Metrik pembeda yang ditemukan: **periode inkonsistensi sementara 7× lebih
   lama di choreography saat beban tinggi** (6238 vs 864 ms, p<0,0001) dan
   **entry point orchestration dapat menolak request saat puncak beban**
   (kejenuhan titik tunggal).
4. Kegagalan kompensasi (S6) = batas fundamental saga di KEDUA pendekatan (0%) —
   kelemahan ada di compensating transaction, bukan di pola koordinasi.
5. S8 (event loss) membuktikan titik lemah dual-write choreography yang
   disebut literatur (Laigner 2021): order committed + event hilang = saga
   stuck tanpa kompensasi; orchestration kebal secara struktural.

## LITERATUR PENDUKUNG (kutipan kunci)

- Megargel et al. (2021): choreography = coupling rendah tapi response time tak menentu &
  multi-salinan data; orchestration = predictable tapi controller = potensi bottleneck.
- Malyuga et al. (2020): orchestrator sentral = single point of failure yang "mampu merusak
  konsistensi saat shutdown di tengah saga"; klaim "choreography bekerja lebih cepat" (dalam
  model dengan overhead replikasi orchestrator).
- Lee et al. (2023): orchestration = standar yang diterima luas; butuh backup mechanism
  untuk recovery cepat saat orchestrator gagal.
- Daraghmi et al. (2022): saga = ACD bukan ACID; read isolation hilang (dirty read);
  solusi quota cache + commit sync; TIDAK mengevaluasi dampak kegagalan terkontrol.
- Talaver & Vakaliuk (2023): "bukan berarti yang satu superior dari yang lain — keduanya
  dipakai di situasi berbeda"; data bisa terbaca dirty sampai saga selesai.
- Munonye & Martinek (2020a): choreography kompleks saat kompensasi gagal; orchestration =
  single point of failure; saga berasumsi kompensasi selalu sukses.
- Laigner et al. (2021): 2PC nyaris tak dipakai (6,79%); database-per-service 43%;
  eventual consistency = model de facto; non-transactional queuing = celah konsistensi.
- Fan et al. (2020): 2PC bottleneck — throughput turun hingga 3,3×, commit rate jatuh
  0,31→0,03 di kontensi tinggi (sumber motivasi memilih saga).
- Baão & Guerreiro (2026): orchestration "harus highly available, tidak boleh jadi SPOF";
  choreography aliran implisit sulit dilacak; **belum ada kerangka evaluasi komprehensif
  konsistensi** — gap yang diisi penelitian ini.
- Chen et al. (2024) & Yu et al. (2025): fault injection non-intrusive = tren;
  kerangka FIT 5 komponen; **belum ada studi yang menerapkan FIT untuk perbandingan
  konsistensi choreography vs orchestration**.

## KETERBATASAN YANG SUDAH DIAKUI PENULIS

1. S8 (event loss) dijalankan 10 run (observasi awal, bukan 30) — jumlah run
   lebih kecil dari skenario lain.
2. Partisi Kafka = 1 (rantai serial) — mempengaruhi angka latency/window choreography.
3. Lingkungan: satu mesin, Docker Compose, 4 service toy — tidak digeneralisasi.
4. Statistik deskriptif + Mann-Whitney U (belum ada uji parametrik/equivalence test).
5. S4/S5 menggunakan DELAY_MS=3000 untuk menciptakan window crash yang deterministik.
6. Tidak ada retry pada kompensasi (S6 = 0% di keduanya); tidak ada outbox/replay
   (S8 = stuck permanen di choreography).

## PERTANYAAN YANG HARUS DIJAWAB

1. **Kekuatan rumusan masalah**: Apakah 3 rumusan masalah ini cukup kuat, fokus, dan
   terjawab oleh desain penelitian? Adakah rumusan yang seharusnya direformulasi?
2. **Kualitas hasil**: Apakah hasil "setara" di RM1/RM2/RM3 merupakan temuan yang valid dan
   cukup kuat untuk skripsi S1, atau terkesan sia-sia? Berikan penilaian jujur.
3. **Kekuatan narasi**: Bagaimana seharusnya hasil ini dibingkai agar bernilai ilmiah
   (bukan sekadar "semuanya sama")? Apa inti kontribusi yang paling defensif?
4. **Kelemahan fatal?**: Adakah kelemahan metodologis yang bisa menggugurkan kesimpulan?
   (misal: pengukuran, desain skenario, analisis statistik, generalisasi)
5. **Hipotesis yang salah arah**: Apa implikasi dari terbantahnya H2 dan H5?
6. **Rekomendasi konkret**: Apa saja yang WAJIB dilakukan penulis sebelum sidang, dan apa
   yang nice-to-have? Prioritaskan.
7. **Prediksi pertanyaan penguji**: Pertanyaan tersulit apa yang mungkin diajukan penguji
   terhadap hasil ini, dan bagaimana menjawabnya?
8. **Skor**: Berikan skor 1–10 untuk: (a) kekuatan rumusan masalah, (b) kualitas desain
   eksperimen, (c) kualitas hasil/temuan, (d) kesiapan untuk sidang skripsi S1.
9. **Satu kalimat kesimpulan**: Apakah penelitian ini layak diterima sebagai skripsi S1
   dengan hasil saat ini?

Format jawaban: jawab setiap pertanyaan dengan heading jelas, berikan alasan spesifik
(rujuk angka/tabel), jujur dan kritis. Gunakan Bahasa Indonesia.