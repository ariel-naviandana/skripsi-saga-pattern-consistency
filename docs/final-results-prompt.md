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
| S1 | Baseline normal | keduanya (30×) | 100% committed keduanya; window inkonsistensi 31 vs 31 ms (tidak signifikan); latency 165 vs 154 ms |
| S2 | Shipping gagal (langkah akhir) | keduanya (30×) | 100% compensated, CTSR 100% keduanya; **recovery time 22,5 vs 25,3 ms (p<0,001)**; latency 1639 vs 160 ms |
| S3 | Inventory gagal (langkah tengah) | keduanya (30×) | 100% compensated, CTSR 100%; **recovery time 18,0 vs 31,1 ms (p<0,0001)**; latency 1670 vs 173 ms |
| S4 | Orchestrator Crash (mid-saga) | orchestration (10×) | 10/10 inconsistent (partial commit, kompensasi tak jalan) |
| S5 | Kafka Down (mid-chain) | choreography (10×) | 10/10 inconsistent (saga macet) |
| S6 | Compensating tx gagal (FAIL_ON_COMPENSATE) | keduanya (30×) | 0% consistency, CTSR 0% keduanya |
| S7 | 500 transaksi konkuren (15.000 total) | keduanya (30×) | 100% committed keduanya (choreo 15000/15000; orch 14998/15000 + 2 request ditolak); **window inkonsistensi 6238 vs 864 ms (p<0,0001)**; latency 9722 vs 5241 ms |
| S8 | Event loss parsial (order.created di-drop diam-diam, broker hidup) | choreography (10×) | **10/10 inconsistent — saga STUCK permanen** (order committed, tak ada yang tahu, kompensasi tak terpicu) |
| S9 | Response hilang / in-doubt (inventory commit sukses tapi response HTTP di-drop, orchestrator timeout) | orchestration (10×) | **10/10 compensated + needless_compensation=true** — konsistensi 100% tapi transaksi valid dibatalkan sia-sia |

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
(S2: 22,5 vs 25,3 ms p<0,001; S3: 18,0 vs 31,1 ms p<0,0001) — magnitudo kecil
(±3–13 ms), keduanya pulih dalam puluhan milidetik. Klaim umum "orchestration jauh
lebih cepat pulih" TIDAK terdukung. Perbedaan latency end-to-end besar (10×) berasal
dari jalur eksekusi forward (hop Kafka), bukan pemulihan.

## CAVEAT & KETERBATASAN YANG SUDAH DIDOKUMENTASIKAN

1. **Call-all compensation (S9):** hasil S9 adalah konsekuensi desain `fail()` yang
   mengompensasi keempat endpoint tanpa syarat (idempotent). Dengan strategi
   compensate-selective, S9 kemungkinan besar menghasilkan orphaned commit
   (inkonsistensi permanen) — "orchestration selalu konsisten" BUKAN properti
   universal, melainkan hasil keputusan desain implementasi ini.
2. **Confounding factor S2/S3:** selisih recovery time sebagian bisa berasal dari
   jumlah panggilan ekstra call-all (4 call vs rantai choreography 2–3 hop),
   di samping penjelasan arsitektural.
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