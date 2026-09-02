# Laporan Hasil Eksperimen

Laporan awal hasil eksperimen skenario S1–S7 untuk pendekatan **choreography**
dan **orchestration**. Data mentah per iterasi tersimpan di `docs/runs/` dan
agregat di `docs/runs/summary.json` (dihasilkan oleh `go run ./cmd/analyze`).

## Metodologi Ringkas

- 4 service (Order, Payment, Inventory, Shipping), database-per-service (PostgreSQL).
- Choreography: koordinasi via Apache Kafka (event-based).
- Orchestration: koordinasi via Saga Orchestrator (HTTP request-reply) + state di Redis.
- Setiap skenario diulang **30 iterasi**, di antara iterasi dilakukan reset penuh
  (truncate DB, flush Redis, reset counter fault).
- Metrik: **Consistency Rate**, **Compensating Transaction Success Rate**,
  dan latensi hingga final state (proksi *recovery time*).

## Hasil Agregat (30 iterasi)

| Skenario | Approach | Txns | Committed | Compensated | Inconsistent | Consistency% | CompSuccess% | AvgLatency (ms) |
|----------|----------|------|-----------|-------------|--------------|--------------|--------------|-----------------|
| S1 | choreography | 30 | 30 | 0 | 0 | 100.0 | — | 228 |
| S1 | orchestration | 30 | 30 | 0 | 0 | 100.0 | — | 107 |
| S2 | choreography | 30 | 0 | 30 | 0 | 100.0 | 100.0 | 600 |
| S2 | orchestration | 30 | 0 | 30 | 0 | 100.0 | 100.0 | 132 |
| S3 | choreography | 30 | 0 | 30 | 0 | 100.0 | 100.0 | 588 |
| S3 | orchestration | 30 | 0 | 30 | 0 | 100.0 | 100.0 | 110 |
| S6 | choreography | 30 | 0 | 0 | 30 | 0.0 | 0.0 | 1100 |
| S6 | orchestration | 30 | 0 | 0 | 30 | 0.0 | 0.0 | 1140 |
| S7 | choreography | 15000 | 14889 | 0 | 111 | 99.3 | — | 10300 |
| S7 | orchestration | 15000 | 14948 | 0 | 52* | 99.7 | — | 6942 |

\* Pada S7 orchestration, 52 transaksi gagal terkirim (request error di bawah
beban) sehingga tidak tercatat pada status akhir; dihitung sebagai tidak
konsisten terhadap total transaksi.

## Analisis per Skenario

### S1 — Baseline Normal
Kedua pendekatan mencapai konsistensi penuh (100%). Orchestration lebih cepat
(latensi ±107 ms vs ±228 ms) karena alur sinkron tanpa antrian broker.

### S2 — Kegagalan Shipping (langkah akhir)
Kompensasi berantai berhasil penuh di kedua pendekatan (CompSuccess 100%).
Orchestration menyelesaikan kompensasi lebih cepat (±132 ms vs ±600 ms) karena
mekanisme request-reply langsung, sedangkan choreography mengikuti propagasi
event antar service (beberapa hop Kafka).

### S3 — Kegagalan Inventory (langkah tengah)
Pola sama dengan S2: kompensasi penuh (100%), orchestration lebih cepat
(±110 ms vs ±588 ms). Kegagalan di tengah rantai menghasilkan kompensasi pada
Order dan Payment; Inventory dan Shipping tidak berpartisipasi.

### S6 — Kegagalan Compensating Transaction
Kedua pendekatan **tidak** dapat mengembalikan konsistensi (0%). Ini wajar:
jika operasi kompensasi itu sendiri gagal, data tertinggal pada status parsial
(Order/Payment tetap `committed`). Temuan ini menyoroti pentingnya mekanisme
kompensasi yang idempoten + retry. Waktu hingga kondisi tidak konsisten
terkonfirmasi serupa (±1,1 s) di kedua pendekatan.

### S7 — Konkurensi 500 Transaksi
Di bawah beban, keduanya tetap menjaga konsistensi tinggi (≥99%). Choreography
sedikit lebih rendah (99.3%) karena sifat asinkron: beberapa saga belum settle
dalam jendela polling. Orchestration menunjukkan latensi rata-rata lebih rendah
(±6.9 s vs ±10.3 s) tetapi terdapat 52 request error — menandakan orchestrator
sebagai titik tunggal yang dapat jenuh saat beban puncak.

## Observasi S4 & S5 (dijalankan manual)

- **S4 — Orchestrator Crash**: transaksi terpotong di tengah → commit parsial
  (misal Order + Payment `committed`) tanpa kompensasi → **inkonsisten**. Crash
  di awal (sebelum commit pertama) tidak meninggalkan efek pada data bisnis,
  hanya sisa state di Redis.
- **S5 — Kafka Down**: event `order.created` tidak sampai ke service berikutnya
  → Order `committed`, Payment/Inventory/Shipping tidak berpartisipasi →
  **inkonsisten** (saga tidak pernah selesai).

## Perbandingan & Rekomendasi

1. **Konsistensi**: dalam skenario kegagalan langkah (S2/S3) kedua pendekatan
   sama-sama mampu mengembalikan konsistensi. Perbedaan tidak muncul pada
   *apakah* konsisten, melainkan pada *seberapa cepat* pemulihan.
2. **Recovery speed**: orchestration konsisten lebih cepat pada kegagalan
   langkah (S2/S3) dan beban normal (S1) karena koordinasi sinkron.
3. **Skalabilitas & titik tunggal**: orchestration lebih cepat namun
   rentan terhadap kegagalan/kebanjiran orchestrator (S4, S7 request error).
   Choreography lebih tahan terhadap kegagalan titik pusat namun lebih lambat
   dan membutuhkan settle time pada beban tinggi.
4. **Kegagalan kompensasi (S6)**: tidak ada pendekatan yang unggul; keduanya
   membutuhkan idempotensi + retry pada compensating transaction agar robust.

**Rekomendasi awal**: untuk sistem dengan prioritas *consistency* dan waktu
pemulihan cepat pada kegagalan langkah, **orchestration** lebih tepat; untuk
sistem yang mengutamakan *availability* dan menghindari titik tunggal
orchestrator, **choreography** lebih tepat — dengan catatan perlu mekanisme
*event replay / outbox* agar saga yang tertunda tetap selesai.

## Rekomendasi Langkah Selanjutnya

- Menambahkan metrik *recovery time* eksplisit (timestamp deteksi kegagalan →
  final state konsisten) via snapshot, bukan hanya latensi request.
- Menganalisis S7 request-error orchestration lebih dalam (resource limit,
  HTTP client timeout) sebagai temuan skalabilitas.
- Menambahkan retry pada compensating transaction dan mengukur dampaknya
  terhadap S6.