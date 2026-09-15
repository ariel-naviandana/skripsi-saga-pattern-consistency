# Penjelasan Lengkap — Skripsi Saga Pattern Consistency

> Dokumen ini berisi penjelasan lengkap dari konsep paling dasar (microservices) sampai bedah kode inti, ditulis dengan bahasa sederhana dan analogi.

---

## Daftar Isi

1. [Microservices — Apa Itu dan Kenapa?](#1-microservices)
2. [Saga Pattern — Konsep Dasar](#2-saga-pattern)
3. [Kenapa Ada Dua Cara: Choreography vs Orchestration](#3-choreography-vs-orchestration)
4. [Arsitektur Sistem](#4-arsitektur-sistem)
5. [Pemilihan Stack dan Komponen](#5-pemilihan-stack)
6. [Alur Eksekusi Saga](#6-alur-eksekusi-saga)
7. [Fault Injection & 12 Skenario](#7-fault-injection)
8. [Pengukuran & Hasil](#8-pengukuran-dan-hasil)
9. [Bedah Kode Inti](#9-bedah-kode-inti)

---

## 1. Microservices

### 1.1 Dulu: Monolith (Satu Aplikasi Besar)

Bayangkan toko online yang **semuanya di satu gedung**:

```
┌─────────────────────────────────────────┐
│              TOKO ONLINE                │
│  ┌─────────┐ ┌─────────┐ ┌─────────┐   │
│  │  Order  │ │ Payment │ │Inventory│   │
│  │  Fitur  │ │  Fitur  │ │  Fitur  │   │
│  └────┬────┘ └────┬────┘ └────┬────┘   │
│       └───────────┴───────────┘         │
│              ↓                          │
│        ┌──────────┐                     │
│        │ SATU DB  │ ← semua fitur       │
│        │ (MySQL)  │   pakai DB yang sama │
│        └──────────┘                     │
└─────────────────────────────────────────┘
```

Semua fitur (order, payment, inventory, shipping) ada di **satu program**, pakai **satu database**.

**Kelebihannya:** Gampang. Satu tim, satu codebase, satu deploy.

**Kekurangannya:**

1. **Kalau satu fitur error, semuanya mati.** Fitur inventory ada bug yang bikin memory leak? Seluruh aplikasi crash — order, payment, shipping ikut mati.

2. **Sulit scale.** Misalnya fitur order yang paling ramai (1000 request/detik), tapi fitur shipping cuma dapat 10 request/detik. Kamu mau scale server yang mana? Kalau scale server, semua fitur ikut di-scale — padahal shipping nggak butuh banyak server.

3. **Deploy lama.** Kamu mau update fitur payment? Harus deploy **seluruh aplikasi**. Kalau ada error di deployment, semua fitur terdampak.

4. **Tim besar = konflik.** 20 developer kerja di satu codebase. Sering tabrakan file, merge conflict, deploy yang satu mengganggu yang lain.

### 1.2 Sekarang: Microservices (Pisah Jadi Bagian-Bagian Kecil)

**Microservices** = memecah aplikasi besar jadi **layanan-layanan kecil yang berdiri sendiri**, masing-masing punya **database sendiri**.

```
┌──────────┐   ┌──────────┐   ┌──────────┐   ┌──────────┐
│  Order   │   │ Payment  │   │Inventory │   │ Shipping │
│ Service  │   │ Service  │   │ Service  │   │ Service  │
│ (Go,Port │   │ (Go,Port │   │ (Go,Port │   │ (Go,Port │
│  8081)   │   │  8082)   │   │  8083)   │   │  8084)   │
└────┬─────┘   └────┬─────┘   └────┬─────┘   └────┬─────┘
     ↓              ↓              ↓              ↓
┌──────────┐   ┌──────────┐   ┌──────────┐   ┌──────────┐
│ order_db │   │payment_db│   │invent_db │   │ship_db   │
│(Postgres)│   │(Postgres)│   │(Postgres)│   │(Postgres)│
└──────────┘   └──────────┘   └──────────┘   └──────────┘
```

Setiap service = **program kecil mandiri** yang bisa dijalankan, di-deploy, dan di-scale **terpisah**.

**Analogi:** Dari "satu gedung toko" → "4 toko kecil di 4 lokasi berbeda". Toko Order di Jl. Merdeka, Toko Payment di Jl. Sudirman, dll.

### 1.3 Kenapa Pake Microservices?

| Masalah di Monolith | Solusi Microservices |
|---------------------|---------------------|
| Satu fitur error → semua mati | Satu service error → service lain tetap jalan |
| Sulit scale selectif | Scale service yang ramai aja (misal: scale Order 10x, Shipping tetap 1x) |
| Deploy lama + berisiko | Deploy service yang diubah aja (misal: update Payment, yang lain nggak terdampak) |
| Tim besar konflik | Tim kecil per service (1-2 orang per service, punya codebase sendiri) |

### 1.4 Trade-off: Dapat Apa, Kehilangan Apa

| Dapat | Kehilangan |
|-------|-----------|
| Isolasi error (satu service mati, yang lain jalan) | Transaksi lintas service jadi sulit |
| Scale per service | Data terpecah, sulit query gabungan |
| Deploy per service | Kompleksitas jaringan naik |
| Tim per service | Butuh koordinasi antar service |

**Microservices bukan solusi sempurna** — itu trade-off. Dan **saga pattern** adalah salah satu cara menangani masalah "transaksi lintas service" yang muncul dari trade-off ini.

### 1.5 Masalah Baru: Transaksi Terdistribusi

Di monolith, beli laptop:
```sql
BEGIN;
  INSERT INTO orders ...;       -- catat pesanan
  UPDATE accounts SET ...;      -- potong saldo
  UPDATE products SET ...;      -- kurangi stok
  INSERT INTO shipments ...;    -- jadwal kirim
COMMIT;  -- semua atau tidak sama sekali
```

Satu database, satu transaksi, gampang. Kalau gagal, tinggal ROLLBACK.

Di microservices, **setiap service punya database sendiri**. Tidak ada satu transaksi SQL yang bisa menjangkau keempat database sekaligus. **Ini masalah utama yang coba dipecahkan oleh saga pattern.**

---

## 2. Saga Pattern

### 2.1 Masalahnya: Transaksi yang Memotong Banyak Service

Bayangkan kamu beli laptop di toko online. Di belakang layar, ada **4 departemen** yang harus kerja:

1. **Departemen Order** — catat pesanan kamu
2. **Departemen Kasir** — potong saldo/kartu kredit
3. **Departemen Gudang** — kurangi stok laptop
4. **Departemen Pengiriman** — jadwal kirim

Kalau ini toko kecil (satu gedung, satu database), gampang — tinggal bilang "semua kerja, kalau gagal semua batalkan." Tapi di **microservices**, setiap departemen punya **database sendiri-sendiri** yang tidak bisa diakses satu sama lain.

**Masalahnya:** Kalau Gudang bilang "stok habis!" setelah Kasir sudah potong saldo, kamu mau gimana? Saldo sudah terpotong, tapi barang tidak dikirim. Data jadi **tidak konsisten** — sebagian sudah masuk, sebagian belum.

### 2.2 Solusi: Saga Pattern

**Saga pattern** = aturan main yang bilang: "Kalau ada yang gagal, jalanin operasi pembalik (compensating transaction) untuk membatalkan efek step yang sudah sukses."

**Analogi:** Kamu pesan katering untuk acara:
- Pesan nasi → catering terima → **committed**
- Pesan lauk → catering terima → **committed**
- Pesan kue → catering bilang "habis!" → **FAILED**

Maka:
- Lauk dikembalikan (compensating transaction) → **compensated**
- Nasi dikembalikan (compensating transaction) → **compensated**
- Semua batal, data konsisten lagi

**Istilah teknis:**
- **Compensating transaction** = operasi pembalik yang membatalkan efek step yang sudah committed
- **Compensating ≠ rollback** — Rollback membatalkan SEBELUM commit (seperti menghapus tulisan sebelum kering). Compensating = menulis "DIBATALKAN" di samping tulisan yang sudah kering

### 2.3 Compensating Transaction Itu Apa?

**Rollback** (di monolith):
```sql
BEGIN;
  INSERT INTO orders VALUES (...);
  -- oops, error!
ROLLBACK;  -- ← data tidak pernah masuk database
```
Data seperti tidak pernah terjadi. Seperti **menghapus tulisan dari papan tulis sebelum kering**.

**Compensating transaction** (di microservices):
```
Step 1: INSERT order (status: committed) → COMMIT sudah terjadi
Step 3: GAGAL

Step 3b: UPDATE order SET status = 'compensated' → transaksi BARU
```
Data **tetap ada** di database (bukan dihapus), tapi statusnya diubah ke "compensated." Seperti **menulis "DIBATALKAN" di samping tulisan yang sudah kering**.

**Kenapa Beda?**

Karena di microservices, kalau Order Service sudah COMMIT, **data sudah final**. Tidak ada "rollback" yang bisa menghapusnya — karena tidak ada satu transaksi besar yang membungkus semua service.

Jadi satu-satunya cara "membatalkan" adalah dengan **menulis data baru** yang bilang "ini sudah dibatalkan."

### 2.4 Kompensasi Gagal, Apa yang Terjadi?

Bayangkan: Payment sudah dikompensasi (refund), tapi waktu mau kompensasi Order, **error lagi**.

```
Step 1: Order → committed ✅
Step 2: Payment → committed ✅
Step 3: Inventory → FAILED ❌

Step 3b: Payment → COMPENSATED ✅
Step 3c: Order → COMPENSATION FAILED ❌ ← error lagi!
```

**Kondisi sekarang:**
- Order: status `committed` (tidak berubah — kompensasi gagal)
- Payment: status `compensated` (sudah dikompensasi)
- Inventory: tidak ada data
- Shipping: tidak ada data

**Ini kondisi permanen yang tidak bisa diperbaiki** — karena tidak ada mekanisme retry atau retry kedua. Ini disebut **batas fundamental** saga pattern (S6 di skripsi ini).

### 2.5 Ringkasan Konsep Saga

| Konsep | Penjelasan |
|--------|-----------|
| **Saga** | Serangkaian transaksi lokal berurutan + compensating transaction jika ada kegagalan |
| **Compensating transaction** | Operasi pembalik yang membatalkan efek step yang sudah committed |
| **Compensating ≠ rollback** | Rollback = undo sebelum commit. Compensating = transaksi baru yang mengubah status |
| **Batas fundamental** | Kalau kompensasi sendiri gagal, data stuck permanen |

---

## 3. Choreography vs Orchestration

### 3.1 Analogi: Dua Cara Mengatur Acara Pernikahan

Bayangkan kamu ngurusin pernikahan. Ada 4 vendor yang harus koordinasi:
- **Dekorasi** (Order)
- **Catering** (Payment)
- **Fotografer** (Inventory)
- **Musik** (Shipping)

#### Cara 1: Grup WhatsApp (Choreography)

Kamu buat grup WA berisi 4 vendor. Tidak ada yang jadi "bos." Tiap vendor mandiri:

- Dekorasi: "Selesai dekorasi!" → kirim foto ke grup
- Catering baca chat → "Oke, giliran aku siapin makanan"
- Fotografer baca chat → "Oke, giliran aku foto"
- Musik baca chat → "Oke, giliran aku siapin sound system"

**Kalau ada masalah** (misal: fotografer bilang "kameraku rusak!"):
- Catering baca chat → "Fotografer gagal? Aku batalin pesanan makananku"
- Dekorasi baca chat → "Catering batal? Aku batalin dekorasiku"

Reaksi merambat sendiri lewat chat. Tidak ada yang koordinasi.

#### Cara 2: Wedding Organizer (Orchestration)

Kamu hire **wedding organizer (WO)** yang jadi bos:

- WO: "Dekorasi, mulai!" → Dekorasi kerja → lapor ke WO "selesai"
- WO: "Catering, mulai!" → Catering kerja → lapor ke WO "selesai"
- WO: "Fotografer, mulai!" → Fotografer kerja → lapor ke WO "selesai"
- WO: "Musik, mulai!" → Musik kerja → lapor ke WO "selesai"

**Kalau ada masalah** (fotografer gagal):
- WO langsung tahu (karena semua lapor ke WO)
- WO suruh catering: "Batalin pesanan makanan"
- WO suruh dekorasi: "Batalin dekorasi"

### 3.2 Perbedaan Kuncinya

| Aspek | Choreography | Orchestration |
|-------|-------------|---------------|
| **Analogi** | Grup WhatsApp | Wedding Organizer |
| **Koordinasi** | Tiap service sendiri | Orchestrator pusat |
| **Komunikasi** | Event via Kafka | HTTP langsung ke service |
| **Kalau crash** | Event chain bisa putus | Orchestrator = single point of failure |
| **Visibility** | Sulit lihat status keseluruhan | Orchestrator tahu semua status |
| **Kompensasi** | Merambat lewat event | Langsung dipanggil orchestrator |
| **Complexity** | Distribusi (tiap service punya logika koordinasi) | Terpusat (semua logika di orchestrator) |

### 3.3 Kenapa Tidak Ada yang "Lebih Baik"?

Karena **keduanya punya trade-off berbeda**:

- **Choreography** lebih resilient terhadap failure (tidak ada single point of failure), tapi lebih sulit dimonitor dan dikontrol
- **Orchestration** lebih gampang dimonitor dan dikontrol, tapi punya single point of failure (orchestrator)

**Tidak ada pemenang universal** — tergantung kebutuhan sistem.

### 3.4 Skripsi Ini: Membandingkan secara Empiris

Sebelumnya, perbandingan choreography vs orchestration lebih banyak di level **teori/konsep**. Penelitian ini membandingkan secara **empiris** (dengan eksperimen beneran):

1. Suntikkan kegagalan secara terkontrol (fault injection)
2. Ukur: konsistensi data, success rate kompensasi, recovery time
3. Bandingkan hasilnya secara statistik (Mann-Whitney U test)

---

## 4. Arsitektur Sistem

### 4.1 Bird's Eye View

```
┌─────────────────────────────────────────────────────────────┐
│                    HOST (Laptop Kamu)                        │
│                                                             │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐ │
│  │   Workload  │  │   Checker   │  │   Analyze Tool      │ │
│  │  Generator  │  │             │  │   (Mann-Whitney U)  │ │
│  │  (port 9999)│  │  (24 detik  │  │                     │ │
│  │             │  │   timeout)  │  │                     │ │
│  └──────┬──────┘  └──────┬──────┘  └─────────────────────┘ │
│         │                │                                  │
│         │ HTTP           │ Query DB                         │
│         ▼                ▼                                  │
│  ┌─────────────────────────────────────────────────────────┐ │
│  │              DOCKER CONTAINERS                          │ │
│  │                                                         │ │
│  │  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐   │ │
│  │  │  Order  │  │ Payment │  │Inventory│  │Shipping │   │ │
│  │  │  :8081  │  │  :8082  │  │  :8083  │  │  :8084  │   │ │
│  │  └────┬────┘  └────┬────┘  └────┬────┘  └────┬────┘   │ │
│  │       └─────────────┴────────────┴─────────────┘        │ │
│  │                      │                                  │ │
│  │            ┌─────────┴─────────┐                        │ │
│  │            │    Orchestrator   │                        │ │
│  │            │      :8080        │                        │ │
│  │            └───────────────────┘                        │ │
│  │                                                         │ │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────────────────┐  │ │
│  │  │  Kafka   │  │  Redis   │  │  4x PostgreSQL       │  │ │
│  │  │  :9092   │  │  :6379   │  │  :5431-5434          │  │ │
│  │  └──────────┘  └──────────┘  └──────────────────────┘  │ │
│  │                                                         │ │
│  │  ┌──────────┐  ┌──────────┐                            │ │
│  │  │Prometheus│  │ Grafana  │                            │ │
│  │  │  :9090   │  │  :3000   │                            │ │
│  │  └──────────┘  └──────────┘                            │ │
│  └─────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
```

### 4.2 Database-per-Service

Setiap service punya **database PostgreSQL sendiri**:

```
Order Service     → order_db (port 5431)
Payment Service   → payment_db (port 5432)
Inventory Service → inventory_db (port 5433)
Shipping Service  → shipping_db (port 5434)
```

**Order Service tidak boleh akses payment_db.** Ini prinsip microservices — setiap service harus otonom.

**Konsekuensinya:** Kalau mau tahu "status pesanan lengkap" (order + payment + inventory + shipping), harus query ke 4 database. Checker (`internal/consistency/checker.go`) yang melakukannya.

### 4.3 Database Schema

Tiap database punya 2 tabel:

```sql
-- Tabel Bisnis (contoh: order_db)
CREATE TABLE orders (
    id            SERIAL PRIMARY KEY,
    saga_id       VARCHAR(64) NOT NULL UNIQUE,
    product       VARCHAR(64) NOT NULL,
    quantity      INT NOT NULL DEFAULT 1,
    total_price   DECIMAL(12,2) NOT NULL,
    status        VARCHAR(20) NOT NULL DEFAULT 'pending',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Tabel Saga Log
CREATE TABLE saga_log (
    id            SERIAL PRIMARY KEY,
    saga_id       VARCHAR(64) NOT NULL,
    step          VARCHAR(32) NOT NULL,
    status        VARCHAR(20) NOT NULL,
    snapshot      JSONB,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

**Analogi:**
- Tabel bisnis = struk belanja (apa yang dibeli, berapa harga)
- Tabel saga_log = catatan petugas (step 1 selesai, step 2 selesai, dst.)

### 4.4 Kafka — "Papan Pengumuman" Digital

**Analogi:** Papan pengumuman di kantor. departemen A tulis memo → taruh di papan → departemen B baca → kerja.

**Yang perlu kamu tahu:**
- **Topic** = jenis pengumumannya (misal: "saga.order.created" = "order sudah dibuat")
- **Producer** = yang nulis memo
- **Consumer** = yang baca memo
- **Broker** = papan pengumumannya (Kafka container)

**Kenapa Kafka, bukan WhatsApp/RabbitMQ?**
- Kafka **menyimpan pesan** (bisa dibaca ulang/replay). Kalau service crash, pesan tetap ada.
- RabbitMQ = queue (pesanan dihapus setelah dibaca). Kalau consumer crash, pesan hilang.
- Untuk eksperimen S8 (event loss), kita butuh pesan yang bisa di-drop secara terkontrol — Kafka memungkinkan ini.

**8 topik Kafka:**
- 4 topik forward (sukses): order.created, payment.processed, inventory.reserved, shipping.scheduled
- 3 topik failed (gagal): payment.failed, inventory.failed, shipping.failed
- 1 topik closing: saga.closed

### 4.5 Redis — "Sticky Note" Cepat

**Analogi:** Sticky note di meja orchestrator. Setiap kali selesai satu step, orchestrator tulis: "Sekarang di step payment." Kalau ada yang nanya "saga abc123 di mana?", orchestrator tinggal liat sticky note.

**Yang perlu kamu tahu:**
- Redis = database in-memory (data di RAM, bukan disk) → sangat cepat (±0.1 ms)
- Cuma menyimpan: `saga:abc123 → "committed"` (key-value sederhana)
- TTL: data otomatis hilang setelah 24 jam

**Penting:** Redis HANYA jejak audit, BUKAN mekanisme recovery. Kalau orchestrator crash, state di Redis tetap ada, tapi nggak ada yang baca untuk "resume" saga

### 4.6 Prometheus + Grafana — "CCTV" Monitoring

**Prometheus** = CCTV yang merekam metrik (CPU, memory, request rate).
**Grafana** = layar monitor yang menampilkan CCTV-nya.

**Yang perlu kamu tahu:**
- Prometheus **BUKAN** sumber throughput/latency utama
- Throughput/latency diukur langsung oleh **workload-generator** (bukan Prometheus)
- Prometheus cuma untuk **observasi umum** — melihat container up/down, latency spike, dll

### 4.7 Kode Bisnis Identik — Hanya Transport yang Beda

**Analogi:** Departemen Kasir punya SOP (Standard Operating Procedure):
1. Cek nominal
2. Potong saldo
3. Catat transaksi

SOP-nya **sama** mau dikasih tahu lewat grup WhatsApp (choreography) atau lewat telepon WO (orchestration). Yang beda cuma **cara menerima perintahnya**.

**Di kode:**
- `internal/business/` = SOP Kasir (ProsesPayment, KompensasiPayment)
- `internal/choreography/` = "baca dari grup WhatsApp" → jalankan SOP
- `internal/orchestration/orchestrator.go` = "dapat telepon dari WO" → jalankan SOP

**Kenapa Ini Penting?**

Karena kalau kode bisnisnya beda, perbandingan jadi **unfair**. "Choreography lebih cepat" bisa jadi karena kodenya lebih optimal, bukan karena arsitektur-nya.

Dengan kode bisnis identik, **perbedaan hasil murni berasal dari mekanisme koordinasi** — choreography vs orchestration.

### 4.8 Switch Pendekatan: Cuma Ganti 1 Environment Variable

```powershell
$env:APPROACH = "choreography"   # jalankan choreography
# atau
$env:APPROACH = "orchestration"  # jalankan orchestration
```

Kode bisnisnya sama. Transport layer yang berubah.

### 4.9 Layer Architecture

```
┌─────────────────────────────────────────┐
│         LAYER 3: TRANSPORT              │
│  (Cara komunikasi antar service)        │
│                                         │
│  Choreography: Kafka publish/subscribe  │
│  Orchestration: HTTP request/response   │
│  Monitoring: Prometheus metrics         │
└─────────────────────┬───────────────────┘
                      │ panggil
                      ▼
┌─────────────────────────────────────────┐
│         LAYER 2: BISNIS LOGIC           │
│  (Apa yang dilakukan service)           │
│                                         │
│  CreateOrder, ProcessPayment,           │
│  ReserveStock, ScheduleShipping         │
│                                         │
│  CompensateOrder, CompensatePayment,    │
│  CompensateInventory, CompensateShipping│
└─────────────────────┬───────────────────┘
                      │ akses
                      ▼
┌─────────────────────────────────────────┐
│         LAYER 1: DATA                   │
│  (Penyimpanan data)                     │
│                                         │
│  PostgreSQL (transaksi bisnis)          │
│  Redis (state tracking)                 │
│  Kafka (event storage)                  │
└─────────────────────────────────────────┘
```

**Analogi:**
- **Transport** = cara kamu kasih tahu pesanan (WhatsApp, telepon, surat)
- **Bisnis** = isi pesanan (pesan nasi goreng, bayar Rp 50.000)
- **Data** = arsip (gudang, kotak pos)

### 4.10 Folder Structure

```
skripsi-saga-pattern-consistency/
├── cmd/                          # 6 executable
│   ├── workload-generator/       # kirim request ke sistem
│   ├── checker/                  # query 4 DB, klasifikasi outcome
│   ├── analyze/                  # aggregation + Mann-Whitney U
│   ├── order-svc/                # jalankan Order Service
│   ├── payment-svc/              # jalankan Payment Service
│   ├── inventory-svc/            # jalankan Inventory Service
│   └── shipping-svc/             # jalankan Shipping Service
│
├── internal/                     # kode private
│   ├── business/                 # kode bisnis murni
│   ├── common/                   # fitur lintas service (faultinject, types)
│   ├── choreography/             # transport layer: Kafka
│   ├── orchestration/            # transport layer: HTTP + Redis
│   ├── http/                     # HTTP handlers
│   ├── consistency/              # checker
│   └── run/                      # runner
│
├── pkg/                          # reusable wrappers (kafka, postgres, redis)
├── scripts/                      # bash/powershell helpers
├── deployments/                  # Docker Compose files
├── docs/                         # dokumentasi
└── db/                           # SQL schema
```

---

## 5. Pemilihan Stack dan Komponen

### 5.1 Go — Bahasa Pemrograman

**Apa Itu:** Go (Golang) = bahasa pemrograman yang dibuat oleh Google tahun 2009. Dipakai untuk membuat program komputer yang berjalan di server.

**Analogi:** Go itu seperti **bahasa Indonesia** untuk menulis instruksi ke komputer. Bedanya, bahasa ini dirancang supaya **cepat, simpel, dan gampang dibaca**.

**Dipakai untuk Apa:** Semua service (Order, Payment, Inventory, Shipping, Orchestrator, Checker, Workload Generator) ditulis pakai Go.

**Kenapa Pilih Go:**

| Alternatif | Kekurangan | Kenapa Bukan |
|------------|-----------|-------------|
| **Python** | Lambat (interpreted) | Butuh performa untuk 500 konkuren (S7) |
| **Java** | Kompleks, boilerplate banyak | Terlalu berat untuk prototipe |
| **Node.js** | Single-threaded (event loop) | Butuh handling konkurensi yang lebih terkontrol |
| **Go** | Kurang mature ecosystem | Pilihan terbaik untuk performa + kesederhanaan |

**Alasan utama:** Go punya **goroutine** (mirip thread tapi lebih ringan) yang memudahkan menulis kode yang bisa handle banyak request sekaligus. Cocok untuk microservices yang harus handle 500 request konkuren (S7).

### 5.2 Apache Kafka — Message Broker

**Apa Itu:** Kafka = sistem yang menyimpan dan mengirim **pesan** antar program.

**Analogi:** Kantor pos digital. Kamu tulis surat → taruh di kotak pos (Kafka) → penerima baca → kerja. Bedanya kantor pos biasa: surat **dihapus** setelah dibaca. Di Kafka, surat **tetap ada** — bisa dibaca ulang kalau perlu.

**Kenapa Pilih Kafka:**

| Alternatif | Kekurangan | Kenapa Bukan |
|------------|-----------|-------------|
| **RabbitMQ** | Pesan dihapus setelah dibaca | Butuh pesan yang bisa di-replay (untuk S8) |
| **Redis Pub/Sub** | Tidak menyimpan pesan | Tidak reliable — kalau consumer mati, pesan hilang |
| **Kafka** | Kompleks setup | Pilihan terbaik untuk eksperimen yang butuh replay |

**Alasan utama:** Di skenario S8 (event loss), kita butuh **meng-drop event secara terkontrol**. Kafka memungkinkan ini karena pesannya tersimpan.

### 5.3 PostgreSQL — Database

**Apa Itu:** PostgreSQL = sistem manajemen database relasional. Menyimpan data dalam **tabel**.

**Analogi:** Gudang arsip dengan sistem indeks yang rapi. Kamu bisa cari data berdasarkan kolom tertentu, gabungkan tabel, dan jaminan data tidak corrupt.

**Kenapa Pilih PostgreSQL:**

| Alternatif | Kekurangan | Kenapa Bukan |
|------------|-----------|-------------|
| **MySQL** | Kurang fitur advanced | PostgreSQL lebih cocok untuk transaksi kompleks |
| **MongoDB** | Kurang cocok untuk transaksi relasional | Butuh join antar tabel (saga_log + bisnis) |
| **SQLite** | Tidak bisa diakses dari banyak container | Butuh 4 database terpisah |
| **PostgreSQL** | Setup lebih kompleks | Pilihan terbaik untuk transaksi terdistribusi |

**Alasan utama:** PostgreSQL mendukung **ACID** — jaminan bahwa transaksi database dilakukan dengan benar.

### 5.4 Redis — In-Memory Database

**Apa Itu:** Redis = database yang menyimpan data di **RAM** (memori), bukan di disk. Sangat cepat (±0.1 ms per operasi).

**Analogi:** Sticky note di meja. Kamu tulis sesuatu, langsung terlihat. Bedanya dengan arsip di gudang (PostgreSQL): sticky note lebih cepat diakses, tapi kalau listrik mati, data hilang.

**Kenapa Pilih Redis:**

| Alternatif | Kekurangan | Kenapa Bukan |
|------------|-----------|-------------|
| **PostgreSQL** | Lambat untuk read/write sederhana | Overkill untuk key-value sederhana |
| **Memcached** | Tidak ada persistence | Butuh data yang bisa dibaca ulang |
| **Redis** | Data di RAM (bisa hilang) | Pilihan terbaik untuk caching + state tracking |

**Alasan utama:** Redis mendukung **TTL** — data otomatis hilang setelah waktu tertentu (24 jam). Cocok untuk state saga.

### 5.5 Docker Compose — Container

**Apa Itu:** Docker = teknologi untuk "membungkus" aplikasi beserta semua dependensinya ke dalam **container**. Docker Compose = alat untuk menjalankan **banyak container sekaligus**.

**Analogi:** Docker itu seperti **kotak kemasan** yang berisi: program + library + database + semua yang dibutuhkan. Docker Compose itu seperti **daftar kemasan** yang bilang: "jalankan kotak A, B, C, D sekaligus."

**Kenapa Pilih Docker:**

| Alternatif | Kekurangan | Kenapa Bukan |
|------------|-----------|-------------|
| **Jalankan langsung di OS** | Sulit setup di mesin berbeda | Butuh environment yang reproducible |
| **VM** | Terlalu berat untuk 11 service | — |
| **Docker** | Learning curve | Pilihan terbaik untuk microservices |

**Alasan utama:** Docker memungkinkan **reproducible experiment** — siapa pun bisa menjalankan eksperimen yang sama di mesin yang berbeda.

### 5.6 Client Libraries

| Library | Fungsi | Kenapa Pilih |
|---------|--------|-------------|
| **Sarama** | Client Kafka untuk Go | Pure Go, simpel, mature |
| **pgx/v5** | Client PostgreSQL untuk Go | Connection pooling, cepat, type-safe |
| **go-redis/v9** | Client Redis untuk Go | Cepat, support TTL |

### 5.7 Mann-Whitney U Test — Statistik

**Apa Itu:** Mann-Whitney U = uji statistik untuk membandingkan **dua kelompok data**. Tidak butuh asumsi data berdistribusi normal.

**Analogi:** Kamu punya 30 nilai ujian kelas A dan 30 nilai kelas B. Kamu mau tahu: "Apakah kelas A secara signifikan lebih tinggi?" Mann-Whitney U test menjawab pertanyaan itu.

**Kenapa Pilih Mann-Whitney:**

| Alternatif | Kekurangan | Kenapa Bukan |
|------------|-----------|-------------|
| **T-test** | Butuh asumsi normalitas | Data kita mungkin tidak normal |
| **Wilcoxon signed-rank** | Butuh data berpasangan | Data kita independent |
| **Mann-Whitney U** | Kurang powerful dari t-test | Pilihan terbaik untuk data non-normal |

### 5.8 Ringkasan Stack

| Komponen | Alasan Utama |
|----------|-------------|
| **Go** | Cepat, simpel, goroutine untuk konkurensi |
| **Kafka** | Menyimpan pesan, bisa di-replay, untuk S8 |
| **PostgreSQL** | ACID, untuk transaksi terdistribusi |
| **Redis** | Cepat (RAM), untuk state tracking |
| **Docker** | Reproducible experiment |
| **Prometheus + Grafana** | Monitoring pendukung |
| **Sarama/pgx/go-redis** | Client libraries yang mature untuk Go |
| **Mann-Whitney U** | Statistik non-parametrik untuk data non-normal |

---

## 6. Alur Eksekusi Saga

### 6.1 Happy Path (Semua Sukses)

Bayangkan customer beli laptop. Semua service berhasil.

```
Customer: "Saya mau beli laptop"
         ↓
    ┌─────────┐
    │ ORDER   │ → Catat pesanan, status: committed
    │ SERVICE │ → Tulis ke database: INSERT orders
    └────┬────┘ → Tulis ke saga_log: "order committed"
         ↓
    ┌─────────┐
    │ PAYMENT │ → Potong saldo, status: committed
    │ SERVICE │ → Tulis ke database: INSERT payments
    └────┬────┘ → Tulis ke saga_log: "payment committed"
         ↓
    ┌──────────┐
    │INVENTORY │ → Reserve stok, status: committed
    │ SERVICE  │ → Tulis ke database: INSERT inventory
    └────┬─────┘ → Tulis ke saga_log: "inventory committed"
         ↓
    ┌──────────┐
    │ SHIPPING │ → Jadwal kirim, status: committed
    │ SERVICE  │ → Tulis ke database: INSERT shipments
    └────┬─────┘ → Tulis ke saga_log: "shipping committed"
         ↓
      SELESAI ✅
```

**Kondisi akhir:** Semua 4 database punya status `committed`. Recovery time = 0. Consistency rate = 100%.

### 6.2 Compensation Path (Inventory Gagal)

```
ORDER SERVICE:
  → INSERT INTO orders (saga_id, status) VALUES ('abc123', 'committed') ✅

PAYMENT SERVICE:
  → INSERT INTO payments (saga_id, status) VALUES ('abc123', 'committed') ✅

INVENTORY SERVICE:
  → Cek stok... stok habis!
  → Tulis ke saga_log: "inventory failed"
  → Return error ❌

KOMPENSASI PAYMENT:
  → UPDATE payments SET status = 'compensated' WHERE saga_id = 'abc123' ✅

KOMPENSASI ORDER:
  → UPDATE orders SET status = 'compensated' WHERE saga_id = 'abc123' ✅
```

**Kondisi akhir:**
- order_db: orders → compensated
- payment_db: payments → compensated
- inventory_db: inventory → KOSONG
- shipping_db: shipments → KOSONG

**Consistency rate = 100%** (semua yang participate sudah compensated)

### 6.3 Compensation Gagal (S6)

```
KOMPENSASI PAYMENT: ✅
KOMPENSASI ORDER: GAGAL ❌ (FAIL_ON_COMPENSATE)

Kondisi: Order committed, Payment compensated → MIXED permanen
```

**Consistency rate = 0%** — tidak bisa diperbaiki.

### 6.4 Crash Infrastruktur (S4/S5)

**S4: Orchestrator Crash (Orchestration)**
```
Orchestrator:
  → panggil Order → committed ✅
  → panggil Payment → committed ✅
  → panggil Inventory → sedang diproses...
  ═══ ORCHESTRATOR DI-STOP ═══
  → Shipping tidak pernah dipanggil
  → Kompensasi tidak pernah dijalankan
```

**S5: Kafka Down (Choreography)**
```
Order → committed ✅ → publish event ✅
Payment → committed ✅ → publish event ✅
Inventory → committed ✅ → publish event...
  ═══ KAFKA DI-STOP ═══
  → Event tidak sampai ke Shipping
  → Shipping tidak pernah jalan
```

**Keduanya menghasilkan:** 3 committed, 0 compensated, 1 kosong → INCONSISTENT

### 6.5 Sinyal Koordinasi Hilang (S8/S9)

**S8: Event Loss (Choreography)**
```
Order → committed ✅ → publish "order.created" → EVENT DI-DROP diam-diam
Payment: tidak pernah menerima event → tidak ada yang terjadi
```
**Kondisi:** 1 committed, 3 kosong → INCONSISTENT (stuck permanen)

**S9: Response Loss (Orchestration)**
```
Order → committed ✅
Payment → committed ✅
Inventory → committed ✅ → response HTTP DI-DROP
Orchestrator timeout → mengira inventory GAGAL → kompensasi penuh
```
**Kondisi:** 0 committed, 3 compensated, 1 kosong → COMPENSATED (padahal sebenarnya semua sukses!)

### 6.6 Selective vs Call-All

| Mode | Perilaku | HTTP Calls |
|------|----------|------------|
| **Call-All** | Orchestrator panggil SEMUA service, tanpa cek | 4 calls |
| **Selective** | Orchestrator query DB dulu, hanya panggil yang committed | 3-4 calls |

**Selective mode bukan solusi untuk false negative (S9)** — karena semua service memang committed, selective tetap kompensasi semua.

### 6.7 500 Transaksi Konkuren (S7)

**Tidak ada fault injection** — semua transaksi berjalan normal, tapi 500 sekaligus.

- **Choreography:** Window inkonsistensi = ±2818 ms (lama karena serial di Kafka)
- **Orchestration:** Window inkonsistensi = ±465 ms (lebih cepat karena HTTP langsung), tapi 48 transaksi ditolak

---

## 7. Fault Injection & 12 Skenario

### 7.1 Apa Itu Fault Injection?

**Fault injection** = sengaja menyuntikkan kegagalan secara terkontrol untuk menguji apakah sistem bisa menanganinya.

**Analogi:** Kamu lagi uji mobil. Kamu **sengaja** pecahkan ban, matikan mesin di tengah jalan, atau rem gagal — untuk lihat apakah mobil punya fitur darurat yang bisa menyelamatkan penumpang.

### 7.2 5 Parameter Fault

| # | Parameter | Fungsi | Analogi |
|---|-----------|--------|---------|
| 1 | `FAIL_AT_STEP` | Service mana yang gagal | Pilih ban mana yang dikempesin |
| 2 | `FAIL_AT_ATTEMPT` | Pada request keberapa gagal | "Baru setelah 3 pesanan, ban mulai bocor" |
| 3 | `DELAY_MS` | Delay sebelum merespons | "Supir tidur 3 detik sebelum ngerem" |
| 4 | `FAIL_ON_COMPENSATE` | Kompensasi juga gagal | "Tombol rem darurat juga dipasang rusak" |
| 5 | `DROP_EVENT` | Event Kafka di-drop diam-diam | "Surat dari pos sampai ke tangan, tapi diteruskan ke orang lain" |
| 6 | `DROP_RESPONSE_AT_STEP` | Response HTTP di-drop | "Gudang sudah kirim, tapi surat konfirmasi tidak sampai" |

### 7.3 Bagaimana Kerjanya di Kode

Kita tidak mengubah kode bisnis. Kita pakai **environment variable** di Docker Compose:

```yaml
- FAIL_AT_STEP=inventory
- FAIL_AT_ATTEMPT=1
- DELAY_MS=3000
```

Di kode bisnis, ada pengecekan di awal setiap operasi:

```go
if s.Fault.ShouldFail("order", false) {
    return error  // ← gagal!
}
```

Kalau env var nggak diset → `ShouldFail()` return false → tidak ada fault → kode jalan normal.

### 7.4 12 Skenario

| # | Skenario | Fault | Hasil | Konsistensi |
|---|----------|-------|-------|-------------|
| S1 | Baseline | Tidak ada | 100% committed | 100% |
| S2 | Shipping gagal | `FAIL_AT_STEP=shipping` | 100% compensated | 100% |
| S3 | Inventory gagal | `FAIL_AT_STEP=inventory` | 100% compensated | 100% |
| S4 | Orchestrator crash | `DELAY_MS=3000` | 30/30 inconsistent | 0% |
| S5 | Kafka crash | `DELAY_MS=3000` | 30/30 inconsistent | 0% |
| S6 | Compensate gagal | `FAIL_ON_COMPENSATE` | mix committed/compensated | 0% |
| S7 | 500 konkuren | Tidak ada | 100%/99.7% committed | 100% |
| S8 | Event loss | `DROP_EVENT` | 1 committed, 3 kosong | 0% |
| S9 | Response loss | `DROP_RESPONSE` | 3 compensated, 1 kosong | 0% |
| S9s | S9 + selective | `SELECT_COMPENSATE` | 3 compensated, 1 kosong | 0% |
| S2s | S2 + selective | `SELECT_COMPENSATE` | 100% compensated | 100% |
| S3s | S3 + selective | `SELECT_COMPENSATE` | 100% compensated | 100% |

---

## 8. Pengukuran & Hasil

### 8.1 3 Metrik Utama

| # | Metrik | Apa yang Diukur | Unit | Analogi |
|---|--------|----------------|------|---------|
| 1 | **Consistency Rate** | Persentase transaksi yang akhirnya konsisten | % | "Berapa % pesanan yang selesai dengan benar?" |
| 2 | **Recovery Time** | Waktu dari kegagalan pertama sampai state konsisten | ms | "Berapa lama WO butuh untuk selesai batalin semua vendor?" |
| 3 | **Latency** | Waktu total dari request masuk sampai selesai | ms | "Berapa lama customer menunggu dari pesan sampai selesai?" |

### 8.2 Hasil Lengkap

| Skenario | Metric | Choreography | Orchestration | p-value | Sig? |
|----------|--------|-------------|---------------|---------|------|
| S1 | latency | 427 ms | 128 ms | 0.73 | No |
| S1 | window | 42 ms | 26 ms | <0.0001 | **Yes** |
| S2 | recovery | 22 ms | 29 ms | 0.0002 | **Yes** |
| S3 | recovery | 13 ms | 26 ms | <0.0001 | **Yes** |
| S3 | latency | 1624 ms | 150 ms | <0.0001 | **Yes** |
| S6 | latency | 123 ms | 156 ms | <0.0001 | **Yes** |
| S7 | latency | 7170 ms | 5898 ms | <0.0001 | **Yes** |
| S7 | window | 2818 ms | 465 ms | <0.0001 | **Yes** |
| S9s | latency | — | 10130 ms | 0.001 | **Yes** |
| S2s | recovery | — | 28 ms | 0.505 | No |
| S3s | recovery | — | 24 ms | 0.036 | **Yes** |

### 8.3 Statistik: Mann-Whitney U Test

**Apa Itu:** Uji statistik non-parametrik untuk membandingkan dua kelompok data. Tidak butuh asumsi data berdistribusi normal.

**Interpretasi p-value:**

| p-value | Interpretasi |
|---------|-------------|
| < 0.05 | **Signifikan** — perbedaan tidak kebetulan |
| ≥ 0.05 | **Tidak signifikan** — perbedaan bisa kebetulan |

**Contoh:** S2 Recovery Time — Choreography: 22 ms, Orchestration: 29 ms, p=0.0002 → "Choreography lebih cepat 7 ms, perbedaan signifikan."

### 8.4 Rekomendasi

| Kebutuhan | Rekomendasi | Alasan |
|-----------|------------|--------|
| **Konsistensi data** | Choreography | Window lebih pendek, 100% committed di S7 |
| **Kontrol & visibility** | Orchestration | Orchestrator tahu semua status |
| **Kecil (< 100 transaksi)** | Keduanya sama | S1-S3: konsistensi 100% di kedua |
| **Besar (500+ transaksi)** | Orchestration | Window 6x lebih pendek |
| **Fault tolerance** | Choreography | Tidak ada single point of failure |
| **S8 (event loss)** | Choreography + outbox pattern | Tanpa outbox, choreography stuck permanen |
| **S9 (response loss)** | Orchestration + timeout lebih pendek | Selective tidak menyelesaikan masalah |

### 8.5 Batasan Penelitian

| # | Batasan | Dampak |
|---|---------|--------|
| 1 | Hanya 4 service | Skema lebih kompleks mungkin berbeda |
| 2 | Database identik (PostgreSQL) | Database berbeda mungkin berbeda |
| 3 | Koneksi LAN (Docker) | Koneksi WAN (cloud) mungkin berbeda |
| 4 | 30 iterasi per skenario | Lebih banyak iterasi → lebih robust |

---

## 9. Bedah Kode Inti

### 9.1 Fault Injection (`internal/common/faultinject.go`)

#### Panel Kontrol Fault

Bayangkan kamu punya **panel kontrol** di meja kerja:

```
┌──────────────────────────────────────────────────┐
│              PANEL KONTROL FAULT                  │
│                                                  │
│  ┌──────────┐  ┌──────────┐  ┌──────────────┐   │
│  │ Pilihan  │  │ Nomor    │  │ Delay        │   │
│  │ Service  │  │ Percobaan│  │ (ms)         │   │
│  │ [order]  │  │ [1]      │  │ [3000]       │   │
│  └──────────┘  └──────────┘  └──────────────┘   │
│                                                  │
│  ┌──────────────┐  ┌──────────────┐              │
│  │ Event Drop   │  │ Response     │              │
│  │ [inventory   │  │ Drop         │              │
│  │  .reserved]  │  │ [inventory]  │              │
│  └──────────────┘  └──────────────┘              │
│                                                  │
│  ┌──────────────┐  ┌──────────────┐              │
│  │ Kompensasi   │  │ Selective    │              │
│  │ Juga Gagal   │  │ Mode         │              │
│  │ [ON/OFF]     │  │ [ON/OFF]     │              │
│  └──────────────┘  └──────────────┘              │
└──────────────────────────────────────────────────┘
```

#### 5 Fungsi Utama

**1. `ShouldFail(step, compensate)` — "Harusnya Gagal Nggak?"**

```go
func (f *FaultConfig) ShouldFail(step string, compensate bool) bool {
    if f.FailAtStep == "" { return false }  // panel mati? tidak ada fault

    attempt := f.attempt.Add(1)  // hitung percobaan

    if compensate {
        return f.FailOnCompensate  // kompensasi juga gagal? (S6)
    }

    if step != f.FailAtStep { return false }  // service ini bukan target

    if f.FailAtAttempt > 0 && attempt < int32(f.FailAtAttempt) {
        return false  // belum sampai percobaan yang ditarget
    }

    return true  // GAGAL!
}
```

**Analogi:** Kamu tekan tombol "gagal" → panel cek: "Service ini target? Percobaan ini yang keberapa?" Kalau semua cocok → service gagal.

Counter pakai `atomic.Int32` — artinya **thread-safe** (bisa diakses dari banyak goroutine sekaligus tanpa tabrakan).

**2. `Delay()` — "Tidur Dulu Sebentar"**

```go
func (f *FaultConfig) Delay() {
    if f.DelayMS > 0 {
        time.Sleep(time.Duration(f.DelayMS) * time.Millisecond)
    }
}
```

**Analogi:** "Tidur dulu 3 detik sebelum kerja." Dipakai untuk S4/S5.

**3. `ShouldDropEvent(topic)` — "Surat Ini Jangan Dikirim"**

```go
func (f *FaultConfig) ShouldDropEvent(topic string) bool {
    if f.DropEvent == "" || topic != f.DropEvent { return false }
    if f.dropped.Swap(true) { return false }  // baru sekali aja
    return true
}
```

**Analogi:** "Surat ke gudang ini buang ke tong sampah." Tapi cuma **sekali** — setelah itu normal lagi.

`f.dropped.Swap(true)` memastikan drop hanya terjadi **sekali per process**.

**4. `ShouldDropResponse(step)` — "Jangan Kirim Balasan"**

```go
func (f *FaultConfig) ShouldDropResponse(step string) bool {
    if f.DropResponseAtStep == "" || step != f.DropResponseAtStep { return false }
    if f.droppedResp.Swap(true) { return false }
    return true
}
```

**Analogi:** "Gudang sudah kerja, tapi jangan kirim surat balasan ke WO."

**5. `ResetAttempt()` — "Reset Panel"**

```go
func (f *FaultConfig) ResetAttempt() {
    f.attempt.Store(0)
    f.dropped.Store(false)
    f.droppedResp.Store(false)
}
```

**Analogi:** "Setelah satu percobaan selesai, reset semua tombol ke posisi awal."

### 9.2 Order Service (`internal/business/order.go`)

#### `CreateOrder` — "Catat Pesanan"

```go
func (s *OrderService) CreateOrder(ctx context.Context, sagaID, customerID, productID string, quantity, amount int) (string, error) {
    // 1. Cek fault injection
    if s.Fault != nil {
        s.Fault.Delay()  // tidur dulu (kalau DELAY_MS > 0)
        if s.Fault.ShouldFail("order", false) {
            _ = writeLog(ctx, s.Pool, sagaID, "order", "failed", "injected")
            return "", fmt.Errorf("business: injected order failure")
        }
    }

    // 2. INSERT ke database
    var orderID int64
    err := s.Pool.QueryRow(ctx,
        `INSERT INTO orders (saga_id, customer_id, product_id, quantity, amount, status)
         VALUES ($1, $2, $3, $4, $5, 'committed') RETURNING order_id`,
        sagaID, customerID, productID, quantity, amount,
    ).Scan(&orderID)

    // 3. Tulis ke saga_log
    writeLog(ctx, s.Pool, sagaID, "order", "committed", fmt.Sprintf("order_id=%d", orderID))

    return fmt.Sprintf("%d", orderID), nil
}
```

**Alur:** Cek fault → Delay → ShouldFail? → INSERT orders → INSERT saga_log

#### `CompensateOrder` — "Batalkan Pesanan"

```go
func (s *OrderService) CompensateOrder(ctx context.Context, sagaID string) error {
    // 1. Cek fault injection
    if s.Fault != nil {
        s.Fault.Delay()
        if s.Fault.ShouldFail("order", true) {  // ← parameter "true" = kompensasi
            _ = writeLog(ctx, s.Pool, sagaID, "order", "compensate_failed", "injected")
            return fmt.Errorf("business: injected order compensation failure")
        }
    }

    // 2. UPDATE status ke 'compensated'
    _, err := s.Pool.Exec(ctx,
        `UPDATE orders SET status = 'compensated' WHERE saga_id = $1 AND status = 'committed'`,
        sagaID,
    )

    // 3. Tulis ke saga_log
    return writeLog(ctx, s.Pool, sagaID, "order", "compensated", "")
}
```

**Penting:** `WHERE status = 'committed'` — hanya mengubah yang statusnya committed.

### 9.3 Orchestrator (`internal/orchestration/orchestrator.go`)

#### `Start` — "Mulai Saga"

```go
func (o *Orchestrator) Start(ctx context.Context, req StartRequest) (StartResponse, error) {
    // 1. Generate saga ID unik
    sagaID := business.NewSagaID()

    // 2. Simpan state di Redis: "order"
    o.saveState(ctx, sagaID, "order")

    // 3. Panggil Order Service
    orderResp, err := o.call(ctx, http.MethodPost, o.OrderURL+"/orders/process", step)
    if err != nil {
        return StartResponse{SagaID: sagaID, Status: "compensated"}, o.fail(ctx, sagaID, fmt.Errorf("order: %w", err))
    }

    // 4-9. Ulangi untuk Payment, Inventory, Shipping
    // ...

    // 10. Simpan state: "committed"
    o.saveState(ctx, sagaID, "committed")

    return StartResponse{SagaID: sagaID, Status: "committed"}, nil
}
```

**Alur:** saveState("order") → panggil Order → saveState("payment") → panggil Payment → ... → saveState("committed")

Kalau ADA SAJA gagal → langsung panggil fail() → kompensasi

#### `fail` — "Jalankan Kompensasi"

```go
func (o *Orchestrator) fail(ctx context.Context, sagaID string, cause error) error {
    o.saveState(ctx, sagaID, "compensating")

    // 1. Siapkan 4 service untuk dikompensasi (mundur)
    steps := []compStep{
        {o.ShippingURL + "/shipments/compensate", o.stepCommitted(ctx, o.shippingDB, "shipments", sagaID)},
        {o.InventoryURL + "/inventory/compensate", o.stepCommitted(ctx, o.inventoryDB, "inventory", sagaID)},
        {o.PaymentURL + "/payments/compensate", o.stepCommitted(ctx, o.paymentDB, "payments", sagaID)},
        {o.OrderURL + "/orders/compensate", o.stepCommitted(ctx, o.orderDB, "orders", sagaID)},
    }

    // 2. Untuk setiap service: cek apakah perlu dikompensasi
    for _, c := range steps {
        if o.Fault != nil && o.Fault.SelectCompensate && !c.wasCommitted {
            continue  // selective mode: skip jika tidak committed
        }
        body := StepRequest{SagaID: sagaID}
        o.call(ctx, http.MethodPost, c.url, body)
    }

    o.saveState(ctx, sagaID, "compensated")
    return cause
}
```

**Analogi:** WO telepon vendor mundur dari belakang ke depan:
1. Shipping: "Kamu sudah kerja?" → query DB → "Belum" → SKIP
2. Inventory: "Kamu sudah kerja?" → query DB → "Ya, committed" → KOMPENSASI
3. Payment: "Kamu sudah kerja?" → query DB → "Ya, committed" → KOMPENSASI
4. Order: "Kamu sudah kerja?" → query DB → "Ya, committed" → KOMPENSASI

#### `stepCommitted` — "Cek Database Dulu"

```go
func (o *Orchestrator) stepCommitted(ctx context.Context, pool *pgxpool.Pool, table, sagaID string) bool {
    if pool == nil { return true }  // fallback: kompensasi

    var status string
    err := pool.QueryRow(ctx,
        fmt.Sprintf(`SELECT status FROM %s WHERE saga_id = $1`, table), sagaID).Scan(&status)

    if err == pgx.ErrNoRows { return false }  // tidak ada data
    if err != nil { return true }  // error → fallback: kompensasi

    return status == "committed"
}
```

**Analogi:** "Sebelum telepon vendor, cek arsip dulu: apakah vendor ini benar-benar sudah kerja?"

### 9.4 Checker (`internal/consistency/checker.go`)

#### `Check` — "Klasifikasi Outcome"

```go
func (c *Checker) Check(ctx context.Context, sagaID string) (SagaOutcome, error) {
    // 1. Query status dari 4 database
    order, _ := statusOf(ctx, c.orderDB, "orders", sagaID)
    payment, _ := statusOf(ctx, c.paymentDB, "payments", sagaID)
    inventory, _ := statusOf(ctx, c.inventoryDB, "inventory", sagaID)
    shipping, _ := statusOf(ctx, c.shippingDB, "shipments", sagaID)

    // 2. Cek apakah ada compensate_failed di saga_log
    cf := false
    for _, pool := range []*pgxpool.Pool{c.orderDB, c.paymentDB, c.inventoryDB, c.shippingDB} {
        _, compFailed, _ := sagaLogFlags(ctx, pool, sagaID)
        if compFailed { cf = true }
    }
    if cf { return OutcomeInconsistent, nil }  // S6: kompensasi gagal

    // 3. Hitung committed vs compensated
    committed, compensated := 0, 0
    for _, st := range []string{order, payment, inventory, shipping} {
        switch st {
        case "committed": committed++
        case "compensated": compensated++
        }
    }

    // 4. Klasifikasi
    if committed == 4 { return OutcomeCommitted, nil }         // happy path
    if compensated > 0 && committed == 0 { return OutcomeCompensated, nil }  // S2/S3
    return OutcomeInconsistent, nil                            // S4/S5/S6/S8/S9
}
```

**Logika:**
```
Semua 4 committed?           → COMMITTED
Semua participate compensated? → COMPENSATED
Sebagian committed, sebagian kosong? → INCONSISTENT (S4/S5)
Ada compensate_failed?        → INCONSISTENT (S6)
Mix committed + compensated?  → INCONSISTENT
```

#### `Timeline` — "Hitung Recovery Time"

Query saga_log dari 4 database → cari: timestamp kegagalan, timestamp terakhir, timestamp pertama → Recovery time = final - detection

### 9.5 Choreography Order Service (`internal/choreography/order.go`)

#### Perbandingan dengan Orchestration

```go
func (s *OrderService) CreateOrderAndPublish(ctx context.Context, ...) (string, string, error) {
    // 1. Generate saga ID
    sagaID := business.NewSagaID()

    // 2. Jalankan bisnis logic (SAMA dengan orchestration)
    orderID, err := s.Biz.CreateOrder(ctx, sagaID, ...)

    // 3. Publish event ke Kafka (BEDA dari orchestration)
    ev := OrderCreatedEvent{SagaID: sagaID, OrderID: orderID, ...}
    s.Pub.Publish(TopicOrderCreated, sagaID, ev)

    return sagaID, orderID, nil
}
```

**Yang SAMA:** `s.Biz.CreateOrder(...)` — bisnis logic identik
**Yang BEDA:** Setelah selesai, publish event ke Kafka (bukan lapor ke orchestrator)

#### Subscribe Event

```go
func (s *OrderService) Run(ctx context.Context) error {
    // Radio "sukses": dengar shipping.scheduled → saga sukses
    go func() {
        ConsumerGroup(ctx, s.Brokers, "order-success", []string{TopicShippingScheduled}, ...)
    }()

    // Radio "kompensasi": dengar payment.failed → kompensasi order
    go func() {
        ConsumerGroup(ctx, s.Brokers, "order-compensate", []string{TopicPaymentFailed}, ...)
    }()
}
```

**Analogi:** Order Service punya 2 "radio":
1. "Kalau saya dengar `shipping.scheduled`, berarti semua sukses!"
2. "Kalau saya dengar `payment.failed`, berarti ada yang gagal — saya batalkan pesanan."

### 9.6 Hubungan Antar Kode

```
Workload Generator
    │
    │ HTTP POST /saga
    ▼
┌─────────────────────────────────────────────────────┐
│  CHOREOGRAPHY MODE                                  │
│                                                     │
│  Order Service (choreography/order.go)              │
│    → business/order.go: CreateOrder()              │
│    → common/faultinject.go: ShouldFail()?           │
│    → business/sagalog.go: writeLog()                │
│    → choreography/producer.go: Publish()            │
│                                                     │
│  Payment Service → Inventory Service → Shipping     │
│  (sama: business + faultinject + sagalog + publish) │
└─────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────┐
│  ORCHESTRATION MODE                                 │
│                                                     │
│  Orchestrator (orchestration/orchestrator.go)       │
│    → call Order: business/order.go: CreateOrder()  │
│    → call Payment: business/payment.go              │
│    → call Inventory: business/inventory.go          │
│    → call Shipping: business/shipping.go            │
│    → fail(): stepCommitted() → kompensasi mundur    │
│    → saveState() → Redis                           │
└─────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────┐
│  CHECKER (consistency/checker.go)                   │
│                                                     │
│  → Query 4 database                                 │
│  → Baca saga_log                                   │
│  → Klasifikasi: committed / compensated / inconsistent │
│  → Hitung recovery time                            │
└─────────────────────────────────────────────────────┘
```

---

> Dokumen ini dibuat untuk persiapan seminar proposal (sempro) skripsi:
> **"Analisis Konsistensi Transaksi Data pada Pola Saga: Perbandingan Choreography dan Orchestration dengan Fault Injection Testing"**
>
> Oleh: Ariel Naviandana Putra (235150701111010)
