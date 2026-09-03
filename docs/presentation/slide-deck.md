# Slide Deck Sempro — Teks & Visual per Slide

Panduan merakit presentasi di Canva. Palet: bg `#ffffff`, teks hitam, aksen biru `#0070c0`, biru tua `#17406d`, error merah `#C00000`.

Aturan: 1 slide = 1 ide. Teks di slide = kata kunci. Penjelasan penuh lewat lisan (naskah di bawah tiap slide).

---

## BAB 1 — PENDAHULUAN

### Slide 1 — Cover
**Teks di slide:**
- Judul: *Analisis Perbandingan Konsistensi Data Transaksi pada Saga Pattern (Choreography vs Orchestration) Menggunakan Fault Injection Testing*
- PROPOSAL SKRIPSI
- Ariel Naviandana Putra — NIM 235150701111010
- Sistem Informasi · Fakultas Ilmu Komputer · Universitas Brawijaya

**Visual:** template FILKOM + logo (sudah ada).

**Naskah:** Salam, perkenalan singkat, sebut judul.

---

### Slide 2 — Peta Presentasi
**Teks di slide:** (tanpa teks tambahan, diagram cukup)

**Visual:** `agenda.drawio` — Latar Belakang → Landasan Teori → Metodologi → Rencana & Target

**Naskah:** "Presentasi ini akan saya sampaikan dalam empat bagian: latar belakang masalah, landasan teori, metodologi penelitian, dan rencana serta target capaian."

---

### Slide 3 — Microservices: Arsitektur Arus Utama
**Teks di slide:**
- Judul: *Microservices: Arsitektur Arus Utama*
- (tanpa teks lain — angka bicara)

**Visual:** `s2-adoption-stats.drawio` (74% Gartner 2023 · 77% & 92% O'Reilly 2020)

**Naskah:** "Microservices sudah menjadi arsitektur arus utama. Survei Gartner 2023 mencatat 74% organisasi telah mengadopsinya, dan laporan O'Reilly menunjukkan 77% adopsi dengan 92% tingkat keberhasilan."

---

### Slide 4 — Masalah: Database per Service
**Teks di slide:**
- Judul: *Masalah: Database per Service*
- (caption satu baris, tulis di Canva): "Transaksi lintas layanan ≠ satu transaksi ACID"

**Visual:** `s2-failure-flow.drawio` (alur gagal + kompensasi merah)

**Naskah:** "Namun microservices membawa konsekuensi: tiap service punya database sendiri. Transaksi yang melintasi Order, Payment, hingga Inventory tidak lagi bisa dijamin satu transaksi ACID. Jika satu service gagal, data menjadi parsial dan inkonsisten."

---

### Slide 5 — ACID vs BASE & CAP
**Teks di slide:**
- ACID → hanya berlaku dalam satu database
- BASE → *Eventually Consistent*
- CAP → saat partisi: pilih *Consistency* atau *Availability*

**Visual:** teks saja (3 baris besar).

**Naskah:** "Dalam teori transaksi terdistribusi, properti ACID tidak dapat diterapkan langsung lintas layanan. Sistem berskala besar mengadopsi model BASE — konsistensi akhir atau eventual consistency. Teorema CAP juga menyatakan saat terjadi partisi jaringan, kita harus memilih antara konsistensi atau ketersediaan."

---

### Slide 6 — Two-Phase Commit Tidak Cocok
**Teks di slide:**
- Throughput turun hingga **3,3×** *(Fan et al., 2020)*
- Semua service harus online
- Penguncian sumber daya sepanjang transaksi

**Visual:** teks saja.

**Naskah:** "Alternatif klasiknya adalah two-phase commit. Namun penelitian Fan et al. 2020 membuktikan throughput-nya turun hingga 3,3 kali lipat, ia mengharuskan semua service online, dan mengunci sumber daya sepanjang transaksi. Jadi 2PC tidak praktis untuk microservices."

---

### Slide 7 — Solusi: Saga Pattern
**Teks di slide:**
- Judul: *Solusi: Saga Pattern*

**Visual:** `saga-concept.drawio` (transaksi global → local tx → compensating merah)

**Naskah:** "Saga pattern hadir sebagai solusi: transaksi global dipecah menjadi rangkaian transaksi lokal, dan setiap langkah memiliki compensating transaction yang dijalankan mundur jika terjadi kegagalan — sehingga sistem mencapai eventual consistency tanpa mengorbankan ketersediaan."

---

### Slide 8 — Rumusan Masalah
**Teks di slide (3 angka besar):**
1. **Konsistensi data** — dampak kegagalan: choreography vs orchestration?
2. **Compensating success rate** — perbandingan antar pendekatan?
3. **Recovery time** — perbandingan antar pendekatan?

**Visual:** teks, angka besar biru tua.

**Naskah:** "Saya merumuskan tiga masalah: pertama, bagaimana dampak skenario kegagalan terhadap konsistensi data transaksi pada kedua pendekatan. Kedua, bagaimana perbandingan tingkat keberhasilan compensating transaction. Ketiga, bagaimana perbandingan recovery time pada masing-masing skenario."

---

### Slide 9 — Tujuan
**Teks di slide:**
- Mengevaluasi dampak kegagalan terhadap konsistensi data
- Menganalisis compensating transaction success rate
- Menganalisis recovery time

**Visual:** teks (sejajar slide 8).

**Naskah:** "Tujuan penelitian sejajar dengan rumusan masalah: mengevaluasi dampak kegagalan terhadap konsistensi data, serta menganalisis perbandingan compensating transaction success rate dan recovery time antara kedua pendekatan."

---

### Slide 10 — Manfaat
**Teks di slide:**
- **Akademis** — kontribusi empiris: data kuantitatif konsistensi saga
- **Praktis** — panduan pemilihan pendekatan bagi arsitek/developer

**Visual:** teks (2 kartu).

**Naskah:** "Secara akademis, penelitian ini memberi kontribusi empiris berupa data kuantitatif. Secara praktis, hasilnya menjadi panduan berbasis data untuk memilih pendekatan saga yang sesuai."

---

### Slide 11 — Batasan Masalah
**Teks di slide:**
- Hanya saga (bukan 2PC / TCC)
- 4 service · domain e-commerce
- 7 skenario kegagalan
- Docker Compose (bukan Kubernetes)
- Tanpa aspek keamanan

**Visual:** teks (daftar pendek).

**Naskah:** "Penelitian dibatasi hanya pada saga pattern — tidak mencakup 2PC atau TCC. Objeknya transaksi e-commerce dengan empat service. Skenario kegagalan dibatasi tujuh skenario. Hasil berlaku di lingkungan Docker Compose, dan aspek keamanan tidak dibahas."

---

## BAB 2 — LANDASAN TEORI

### Slide 12 — Penelitian Terkait
**Teks di slide (tabel mini):**

| Penelitian | Fokus | Keterbatasan |
|---|---|---|
| Daraghmi 2022 | saga e-commerce | tanpa fault injection |
| Lee 2023 | orchestration only | tanpa pembanding |
| Malyuga 2020 | orchestrator fault tolerant | orchestration only |
| Yu 2025 / Chen 2024 | kerangka FIT | tidak di saga |

**Visual:** tabel.

**Naskah:** "Beberapa penelitian terkait: Daraghmi mengimplementasikan saga di e-commerce namun tanpa fault injection. Lee dan Malyuga fokus orchestration saja. Yu dan Chen menyusun kerangka fault injection namun tidak diterapkan pada saga pattern."

---

### Slide 13 — Gap Penelitian
**Teks di slide (tebal, besar):**
> Belum ada penelitian yang menggabungkan:
> perbandingan 2 pendekatan + fault injection terkontrol + evaluasi konsistensi + containerized

**Visual:** teks saja.

**Naskah:** "Belum ditemukan penelitian yang secara bersamaan membandingkan kedua pendekatan saga, menggunakan fault injection terkontrol, mengevaluasi konsistensi data, dan berjalan di lingkungan containerized. Di sinilah posisi penelitian ini."

---

### Slide 14 — Saga Pattern
**Teks di slide:**
- Judul: *Saga Pattern*
- (caption): "compensating transaction ≠ rollback — transaksi baru yang membatalkan secara semantik"

**Visual:** `saga-pattern.drawio` (maju biru, mundur merah)

**Naskah:** "Secara konsep, saga mengeksekusi transaksi lokal secara maju. Compensating transaction bukan rollback database — ia transaksi baru yang secara semantik membatalkan efek transaksi sebelumnya, tetap ACID di level lokal."

---

### Slide 15 — Choreography
**Teks di slide:**
- Judul: *Choreography*
- (caption): "event · tanpa koordinator"

**Visual:** `choreography.drawio`

**Naskah:** "Pendekatan pertama, choreography: tidak ada koordinator pusat. Setiap service mempublikasikan event dan merespons event dari service lain. Loose coupling tinggi, namun alur sulit dilacak dan data direplikasi antar service."

---

### Slide 16 — Orchestration
**Teks di slide:**
- Judul: *Orchestration*
- (caption): "koordinator pusat · request-reply · state di Redis"

**Visual:** `orchestration.drawio`

**Naskah:** "Pendekatan kedua, orchestration: satu Saga Orchestrator memanggil service secara berurutan via request-reply, menyimpan state di Redis, dan memicu kompensasi saat gagal. Visibilitas alur tinggi, namun orchestrator menjadi titik kritis."

---

### Slide 17 — Choreography vs Orchestration
**Teks di slide (tabel):**

| Aspek | Choreography | Orchestration |
|---|---|---|
| Koordinasi | event, terdesentralisasi | orchestrator pusat |
| Data | replikasi antar service | eksklusif per service |
| Risiko | inkonsistensi salinan | kegagalan orchestrator |
| Coupling | loose | tight ke orchestrator |

**Visual:** tabel.

**Naskah:** "Perbandingan keduanya dari sisi data: choreography terdesentralisasi dengan risiko inkonsistensi antar salinan; orchestration terpusat dengan risiko kegagalan orchestrator. Perbedaan inilah yang menjadi dasar hipotesis pola konsistensi yang berbeda saat kegagalan."

---

### Slide 18 — Konsistensi Data dalam Saga
**Teks di slide:**
- Fokus: **data consistency**
- Konsisten = berhasil penuh **atau** terkompensasi penuh
- Selama saga berjalan → data dapat terbaca *dirty*

**Visual:** teks.

**Naskah:** "Penelitian berfokus pada data consistency: nilai data di seluruh service mencerminkan hasil akhir yang benar. Selama saga berjalan, data bisa terbaca dirty — inilah karakteristik bawaan saga yang akan diukur dampaknya."

---

## BAB 3 — METODOLOGI

### Slide 19 — Tipe & Strategi Penelitian
**Teks di slide:**
- Tipe: **Konstruktif** (construction) — membangun 2 sistem
- Strategi: **Eksperimen kuantitatif komparatif**
- Kondisi uji setara → perbedaan hasil = efek pendekatan

**Visual:** teks.

**Naskah:** "Penelitian ini implementatif dengan tipe konstruktif: dua sistem dibangun sebagai artefak. Strateginya eksperimen kuantitatif komparatif — kondisi uji dibuat setara, sehingga perbedaan hasil dapat diatribusikan pada perbedaan pendekatan implementasi."

---

### Slide 20 — Lingkungan Eksperimen
**Teks di slide:**
- Go 1.21+ · Kafka 3.x · Redis 7.x · PostgreSQL 15.x
- Docker Compose (containerized)
- Prometheus + Grafana

**Visual:** teks/ikon.

**Naskah:** "Seluruh komponen berjalan dalam container Docker Compose pada satu mesin — Go sebagai bahasa service, Kafka untuk choreography, Redis untuk state orchestration, PostgreSQL per service, dan Prometheus-Grafana untuk monitoring."

---

### Slide 21 — Rancangan Sistem
**Teks di slide:**
- Judul: *Rancangan Sistem*

**Visual:** `architecture.drawio`

**Naskah:** "Alur transaksinya: Order mencatat pesanan, Payment memproses pembayaran, Inventory mengurangi stok, Shipping menjadwalkan pengiriman. Tiap service punya PostgreSQL sendiri. Koordinasi via Kafka untuk choreography, via orchestrator dengan Redis untuk orchestration."

---

### Slide 22 — Dua Implementasi
**Teks di slide:**
- Judul: *Dua Implementasi Setara*

**Visual:** `two-panels.drawio`

**Naskah:** "Kedua implementasi memakai basis kode service yang identik — perbedaannya hanya mekanisme koordinasi. Ini penting: perbedaan hasil pengujian murni berasal dari pendekatan, bukan dari perbedaan sistem."

---

### Slide 23 — Snapshot Data
**Teks di slide:**
- Judul: *Snapshot Data*

**Visual:** `snapshot.drawio` (initial → intermediate → failure → final)

**Naskah:** "Untuk mengukur konsistensi, setiap implementasi mencatat snapshot database pada empat titik: sebelum transaksi, setelah tiap transaksi lokal, saat kegagalan, dan setelah kompensasi selesai. Perbandingan initial versus final state menjadi dasar perhitungan consistency rate."

---

### Slide 24 — Fault Injection
**Teks di slide:**
- Non-intrusive via **environment variable** container
- `FAIL_AT_STEP` · `FAIL_AT_ATTEMPT` · `DELAY_MS` · `FAIL_ON_COMPENSATE`
- Kerangka FIT 5 komponen *(Yu et al., 2025)*

**Visual:** teks.

**Naskah:** "Fault injection dirancang non-intrusive — dikonfigurasi lewat environment variable di level container, tanpa mengubah kode service. Empat parameternya: langkah yang gagal, percobaan ke berapa, delay, dan kegagalan kompensasi. Desain ini mengacu kerangka FIT lima komponen dari Yu et al."

---

### Slide 25 — Skenario Pengujian
**Teks di slide (tabel):**

| ID | Skenario |
|---|---|
| S1 | Baseline normal |
| S2 | Shipping gagal (langkah akhir) |
| S3 | Inventory gagal (langkah tengah) |
| S4 | Orchestrator crash *(orchestration)* |
| S5 | Kafka down *(choreography)* |
| S6 | Compensating transaction gagal |
| S7 | 500 transaksi konkuren |

**Visual:** tabel.

**Naskah:** "Tujuh skenario mencakup kegagalan service, kegagalan infrastruktur, kegagalan kompensasi, dan beban tinggi. S4 khusus orchestration, S5 khusus choreography."

---

### Slide 26 — Metrik Pengukuran
**Teks di slide (3 kartu + 1 catatan):**
- **Consistency Rate** — utama
- **Compensating Tx Success Rate**
- **Recovery Time**
- (+ Throughput & Latency sebagai pembanding)

**Visual:** teks (3 kartu, reuse style kartu stats).

**Naskah:** "Metrik utamanya consistency rate: proporsi transaksi yang berakhir konsisten. Kedua, compensating transaction success rate. Ketiga, recovery time. Throughput dan latency dikumpulkan sebagai metrik pembanding performa."

---

### Slide 27 — Prosedur & Analisis Data
**Teks di slide:**
- Tiap skenario: **30 iterasi**
- Reset total antar iterasi (DB + Redis + fault counter)
- Analisis: statistik deskriptif (mean, min, max, SD) per pendekatan

**Visual:** teks.

**Naskah:** "Setiap skenario diulang 30 kali dengan reset total di antara iterasi. Data dianalisis dengan statistik deskriptif — rata-rata, minimum, maksimum, standar deviasi — lalu dibandingkan antar pendekatan per skenario."

---

### Slide 28 — Progres Implementasi
**Teks di slide (checklist):**
- ✅ Infrastruktur: Kafka, Redis, 4× PostgreSQL, Docker Compose
- ✅ 4 service + saga logic (choreography & orchestration)
- ✅ Fault injection non-intrusive + pipeline 7 skenario
- ⏳ Pengumpulan data penuh & analisis

**Visual:** teks checklist.

**Naskah:** "Sebagai bukti kelayakan, sistem sudah berjalan end-to-end: infrastruktur, kedua implementasi saga, fault injection, dan pipeline pengujian. Verifikasi awal menunjukkan hasil konsisten sesuai harapan. Selanjutnya pengumpulan data penuh dan analisis."

---

### Slide 29 — Target & Penutup
**Teks di slide:**
- Target: P0 → **P1 (50%)** → **P2 (80%)** → sidang
- Kontribusi: rekomendasi pemilihan pendekatan saga
- **Terima kasih** — siap menerima masukan

**Visual:** teks.

**Naskah:** "Target capaian mengikuti milestone: setelah P0 ini, minimal 50% pada P1 dan 80% pada P2. Kontribusi akhirnya adalah rekomendasi berbasis data untuk pemilihan pendekatan saga pattern. Terima kasih, saya siap menerima saran dan masukan."
