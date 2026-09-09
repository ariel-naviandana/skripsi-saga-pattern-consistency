# Prompt: Ringkasan Hasil Final Skripsi — Saga Pattern (Choreography vs Orchestration)

> Cara pakai: salin seluruh isi file ini ke Claude sebagai satu pesan, lalu minta
> penilaian/verifikasi akhir atas hasil penelitian ini. Semua informasi sudah lengkap
> dan self-contained — tidak perlu konteks tambahan.

---

Anda adalah penilai akademik (pembimbing skripsi S1 bidang sistem terdistribusi).
Berikut adalah HASIL FINAL sebuah penelitian eksperimen tentang perbandingan konsistensi
data transaksi antara dua pendekatan saga pattern. Evaluasilah: (1) apakah jawaban untuk
ketiga rumusan masalah sudah kuat dan defensible, (2) apakah ada kelemahan yang masih
terlewat, (3) apa yang harus diperbaiki sebelum sidang. Jujur dan kritis.

## KONTEKS

- Judul: Analisis Perbandingan Konsistensi Data Transaksi pada Saga Pattern (Choreography
  vs Orchestration) menggunakan Fault Injection Testing di Arsitektur Microservices Containerized
- Sistem: 4 service e-commerce (Order, Payment, Inventory, Shipping), database-per-service
  (PostgreSQL), Go, Docker Compose. Choreography via Kafka (8 topik, partisi 1); orchestration
  via HTTP request-reply + state di Redis. Kode bisnis IDENTIK untuk kedua pendekatan (hanya
  env APPROACH yang berbeda) — eksperimen adil.
- Fault injection: non-intrusive via env var container (FAIL_AT_STEP, FAIL_AT_ATTEMPT,
  DELAY_MS, FAIL_ON_COMPENSATE, DROP_EVENT, DROP_RESPONSE_AT_STEP). Tidak ada perubahan
  kode antar skenario.

## RUMUSAN MASALAH (persis dari proposal)

1. RM1: Bagaimana dampak skenario kegagalan terhadap konsistensi data transaksi antar service
   pada pendekatan choreography dan orchestration dalam saga pattern?
2. RM2: Bagaimana perbandingan compensating transaction success rate antara pendekatan
   choreography dan orchestration pada saga pattern dalam berbagai skenario kegagalan?
3. RM3: Bagaimana perbandingan recovery time antara kedua pendekatan tersebut pada masing-masing
   skenario kegagalan yang diujikan?

## METRIK (sesuai proposal 3.5)

- Consistency Rate: % transaksi berakhir konsisten (semua committed ATAU semua compensated)
- CTSR: % compensating transaction berhasil
- Recovery Time: deteksi kegagalan (timestamp row "failed" di saga_log) → final state konsisten
  (timestamp row terakhir), diukur langsung dari timestamp DB, bukan proksi
- Periode inkonsistensi sementara (saga_log pertama → terakhir); latency end-to-end;
  statistik deskriptif (mean/min/max/std) + Mann-Whitney U (n=30 per pendekatan, mean per run)

## SKENARIO & HASIL

| Skenario | Deskripsi | Pendekatan | Hasil |
|----------|-----------|-----------|-------|
| S1 | Baseline normal | keduanya (30×) | 100% committed keduanya; window inkonsistensi 41,5 vs 26,3 ms (p<0,0001); latency 427 vs 128 ms (TIDAK signifikan p=0,73) |
| S2 | Shipping gagal (langkah akhir) | keduanya (30×) | 100% compensated, CTSR 100% keduanya; **recovery time 22,0 vs 28,7 ms (p=0,0002)**; latency 1639 vs 163 ms |
| S3 | Inventory gagal (langkah tengah) | keduanya (30×) | 100% compensated, CTSR 100%; **recovery time 12,8 vs 26,0 ms (p<0,0001)**; latency 1624 vs 150 ms |
| S4 | Orchestrator Crash (mid-saga) | orchestration (10×) | 10/10 inconsistent (partial commit, kompensasi tak jalan) |
| S5 | Kafka Down (mid-chain, dengan DELAY_MS=3000) | choreography (10×) | 10/10 inconsistent — order+payment committed (DELAY memberi waktu Kafka kirim ke payment sebelum stop) |
| S6 | Compensating tx gagal (FAIL_ON_COMPENSATE) | keduanya (30×) | 0% consistency, CTSR 0% keduanya |
| S7 | 500 transaksi konkuren (15.000 total) | keduanya (30×) | 100% committed (choreo 15000/15000; orch 14918/15000 + 82 request ditolak — fluktuasi run-to-run tinggi); **window inkonsistensi 5152 vs 628 ms (p<0,0001)**; latency 8476 vs 5010 ms |
| S8 | Event loss parsial (order.created di-drop diam-diam, broker hidup) | choreography (30×) | **30/30 inconsistent — saga STUCK permanen** (order committed, tak ada yang tahu, kompensasi tak terpicu) |
| S9 | Response hilang / in-doubt (inventory commit sukses tapi response HTTP di-drop, orchestrator timeout) | orchestration (30×) | **30/30 compensated + needless_compensation=true (100%)** — konsistensi 100% tapi transaksi valid dibatalkan sia-sia |
| S9s | S9 + SELECTIVE_COMPENSATE (orchestrator skip kompensasi untuk service yang tidak committed) | orchestration (30×) | **30/30 compensated (100%)** — hasil data identik dengan S9; selective membaca kebenaran DB, jadi orphaned commit (prediksi) tidak terjadi. Selisih call-all vs selective pada S9 murni efisiensi (±30 ms latency tambahan) |
| S2s | Shipping gagal + SELECTIVE_COMPENSATE | orchestration (30×) | **30/10 compensated, recovery 28 ± 6 ms** (stabil tanpa outlier, n=30) — counterfactual RM3 confounding |
| S3s | Inventory gagal + SELECTIVE_COMPENSATE | orchestration (30×) | **30/10 compensated, recovery 24 ± 8 ms** — counterfactual RM3 confounding: call-all (S3) 26 ms vs selective (S3s) 24 ms = confounding terisolasi, selisih dalam std dev |

## TEMUAN UTAMA (jawaban per RM)

**RM1 (konsistensi):** setara pada kondisi normal (S1/S2/S3/S7 = 100% keduanya) dan
kegagalan kompensasi (S6 = 0% keduanya). Perbedaan muncul pada kegagalan SINYAL
KOORDINASI HILANG: S8 (event di-drop, choreography) → saga stuck, 0% konsisten;
S9 (response di-drop, orchestration) → fully compensated, 100% konsisten tetapi
transaksi valid dibatalkan sia-sia.

**RM2 (CTSR):** setara (S2/S3 = 100% keduanya, S6 = 0% keduanya). Pada S8 kompensasi
tidak pernah terpicu (tidak ada yang tahu); pada S9 kompensasi berhasil 100% namun
tidak diperlukan (false negative).

**RM3 (recovery time):** berbeda signifikan (Mann-Whitney U): CHOREOGRAPHY LEBIH CEPAT
(S2: 22,0 vs 28,7 ms p=0,0002; S3: 12,8 vs 26,0 ms p<0,0001) — magnitudo kecil
(±5–13 ms), keduanya pulih dalam puluhan milidetik. Klaim umum "orchestration jauh
lebih cepat pulih" TIDAK terdukung. Perbedaan latency end-to-end besar (10×)
berasal dari jalur eksekusi forward (hop Kafka), bukan pemulihan.

## PERUBAHAN SIGNIFIKAN vs RUN SEBELUMNYA

1. **S1 latency:** klaim awal "signifikan berbeda" tidak stabil — di re-run final
   p=0,73 (tidak signifikan), karena outlier 1626 ms menghilang. Tidak dipublikasikan
   sebagai temuan komparatif S1.
2. **S7 orchestration unrecorded:** 82 (run sebelumnya 2) — fluktuasi run-to-run
   besar; mekanisme: Docker Desktop vpnkit/port-forwarder pada Windows
   (TIME_WAIT/ephemeral port pool dengan burst 500 koneksi dalam <1 s memicu
   intermittent refused).
3. **Recovery time S2:** selisih choreography vs orchestration bergeser dari
   (22,5 vs 25,3 ms) ke (22,0 vs 28,7 ms) — tetap signifikan, arah sama.
4. **S9 vs S9s (counterfactual, n=30):** S9s menjalankan strategi compensate-selective
   untuk menguji apakah call-all design menghasilkan orphaned commit (prediksi
   penilai independen). Hasil: **30/30 compensated, identik dengan S9** (variansi rendah,
   latency 10135 ± 18 ms vs S9 10159 ± 42 ms) — orphaned commit **tidak terjadi**
   karena selective membaca kebenaran DB. Perbedaan call-all vs selective murni
   efisiensi (±25 ms latency tambahan). Caveat call-all tetap valid untuk konteks
   S2/S3 (di mana call-all menambah jumlah HTTP call dan berpotensi mempengaruhi
   recovery time), tapi belum diisolasi secara empiris.
5. **S2s & S3s (isolasi confounding RM3, n=30 + MWU):** skenario step-failure
   (Shipping/Inventory gagal) dengan SELECTIVE_COMPENSATE. Uji Mann-Whitney U
   per pasangan (dalam orchestration): S2 vs S2s recovery **p=0,50**
   (tidak signifikan — confounding terisolasi); S3 vs S3s **p=0,04** (signifikan,
   selisih 2 ms, selective sedikit lebih cepat); S9 vs S9s latency **p=0,001**
   (signifikan, selisih 29 ms). Selective compensate memberikan sedikit keuntungan
   kecepatan pada S3/S9, namun magnitudo sangat kecil sehingga **tidak mengubah
   outcome data** — keduanya tetap 100% compensated. Selisih choreography vs
   orchestration murni arsitektural untuk S2, dan dominan arsitektural (bukan
   dominan call-all) untuk S3.

## CAVEAT & KETERBATASAN YANG SUDAH DIDOKUMENTASIKAN

1. **Call-all compensation (S9):** sempat diduga bahwa strategi compensate-selective
   akan menghasilkan orphaned commit pada S9. Pengujian counterfactual (S9s)
   MEMBANTAH dugaan ini — hasil S9s identik dengan S9 (100% compensated),
   karena implementasi `SELECTIVE_COMPENSATE` memverifikasi status commit
   aktual di database sebelum memutuskan kompensasi, bukan hanya mengandalkan
   riwayat panggilan orchestrator (kode `internal/orchestration/orchestrator.go`
   baris 59-73: `stepCommitted()` query `SELECT status FROM <table> WHERE saga_id = $1`).
   Dengan demikian, **ketahanan orchestration terhadap kondisi in-doubt TIDAK
   bergantung pada strategi call-all vs selective, selama proses kompensasi
   memverifikasi status aktual di database**.
2. **Confounding factor S2/S3:** terisolasi secara empiris oleh S2s & S3s. S3
   (call-all) recovery 26,0 ms vs S3s (selective) recovery 28 ms — selisih 2 ms
   (di dalam std dev). Call-all **tidak menambah overhead terukur** pada recovery
   time. Selisih choreography vs orchestration murni arsitektural, bukan
   artefak jumlah HTTP call.
3. **S8/S9 dieksklusi dari MWU** (skenario satu-pendekatan, tak ada pasangan komparatif);
   keduanya 10 run (observasi awal, bukan 30).
4. **S4/S5** memakai DELAY_MS=3000 untuk window crash deterministik (tanpa delay, saga
   selesai lebih cepat dari latency docker stop).
5. **Partisi Kafka = 1** (rantai serial) memengaruhi angka latency/window choreography.
6. **Lingkungan:** satu mesin, Docker Compose, 4 service toy — tidak digeneralisasi.
7. **Tidak ada retry** pada kompensasi (S6 = 0% keduanya); tidak ada outbox/replay
   (S8 = stuck permanen).
8. **Mann-Whitney U** dengan normal approximation (n=30); uji signifikansi tidak
   membuktikan kesetaraan — hanya menolak perbedaan.

## PERTANYAAN UNTUK ANDA

1. Apakah jawaban RM1, RM2, RM3 sudah kuat dan defensible untuk skripsi S1? Skor 1–10
   per RM dengan alasan.
2. Apakah narasi S8/S9 sebagai "pasangan struktural" sudah berimbang dan tidak bias?
3. Adakah kelemahan/pertanyaan penguji yang masih terlewat dari daftar keterbatasan di atas?
4. Apa saja yang WAJIB dan nice-to-have sebelum sidang?
5. Satu paragraf: ringkasan "cerita penelitian" yang paling defensible untuk disampaikan
   di sidang.

Format: jawab per pertanyaan dengan heading jelas, rujuk angka, jujur dan kritis.
Gunakan Bahasa Indonesia.