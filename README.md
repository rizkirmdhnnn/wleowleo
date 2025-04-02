# WleoWleo Scraper

<div align="center">
<img src="https://pbs.twimg.com/media/Fjv-7DQVIAAnqdn.jpg" alt="drawing" width="250"/>

_Hai! Ini adalah aplikasi Go buat nyedot dan download video-video wleowleo yang mantap-mantap itu lho. Kalian gak usah ngerti linknya ya, nanti dosa 😉. Tool ini bakal otomatis nyari link video dan download-nya dalam format MP4 buat kalian._

</div>

## Fitur Keren

- Web scraping pake browser headless (browser tanpa tampilan) dengan ChromeDP
- Nyedot link video dari halaman web secara otomatis dengan sistem retry
- Download video langsung dalam format MP4 dengan optimasi multi-worker
- Simpen semua link yang udah di-scrape ke file JSON
- Arsitektur microservices dengan RabbitMQ untuk komunikasi antar service
- Bisa diatur-atur lewat file konfigurasi terpusat
- Pembersihan otomatis file sementara setelah konversi berhasil

## Yang Harus Ada

- Docker dan Docker Compose (untuk instalasi)

## Cara Pasang dengan Docker

### Persiapan

1. Pastiin [Docker](https://www.docker.com/products/docker-desktop/) dan [Docker Compose](https://docs.docker.com/compose/install/) udah terpasang di komputer kamu.

2. Clone repo-nya dulu:

```bash
git clone https://github.com/rizkirmdhnnn/wleowleo.git
cd wleowleo
```

3. Bikin file konfigurasi (lihat bagian "Konfigurasi" di bawah):

```bash
cp config.example.json config.json
# Edit file config.json sesuai kebutuhan
```

4. Bikin folder `downloads` dan `temp` (kalo belum ada):

```bash
mkdir -p downloads temp
```

### Cara Jalanin

**Pake Docker Compose (Direkomendasikan):**

```bash
# Jalankan dari root project
docker compose -f deployments/docker-compose.yaml up
```

Atau kalo mau jalanin di background:

```bash
docker compose -f deployments/docker-compose.yaml up -d
```

Semua hasil scraping dan video bakal disimpen di folder `downloads` di komputer kamu (sesuai konfigurasi di docker-compose.yaml).

## Konfigurasi

Proyek ini menggunakan file konfigurasi terpusat (`config.json`) yang terletak di root project. File ini berisi semua konfigurasi yang dibutuhkan oleh semua service (scraper, downloader, dan web dashboard).

Bikin file `config.json` di folder utama. Contek aja dari `config.example.json` yang udah ada:

```json
{
  "app": {
    "name": "wleowleo",
    "logLevel": 4,
    "env": "development"
  },
  "rabbitmq": {
    "url": "amqp://guest:guest@localhost:5672/",
    "exchange": {
      "task": "task_exchange",
      "log": "log_exchange"
    },
    "queue": {
      "commandQueue": "scraper_commands",
      "downloaderQueue": "scraper_video_queue",
      "logQueue": "web_log_queue"
    },
    "reconnectRetries": 5,
    "reconnectTimeout": 10000
  },
  "scraper": {
    "host": "" /* berdosa loh gaboleh */,
    "userAgent": "Mozilla/5.0 (Linux; Android 6.0; Nexus 5 Build/MRA58N) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/134.0.0.0 Mobile Safari/537.36"
  },
  "downloader": {
    "defaultWorker": 5,
    "tempDir": "./temp",
    "downloadDir": "./downloads"
  },
  "webpanel": {
    "host": "localhost",
    "port": 8080
  }
}
```

Untuk penjelasan lebih detail tentang konfigurasi, lihat file [CONFIG.md](CONFIG.md).

## Cara Pake

### Menjalankan sebagai Microservices

Cara paling mudah adalah menggunakan Docker Compose:

```bash
# Jalankan dari root project
docker compose -f deployments/docker-compose.yaml up
```

Ini akan menjalankan empat service:

1. **RabbitMQ** - Message broker untuk komunikasi antar service
2. **Scraper** - Service untuk scraping link video
3. **Downloader** - Service untuk download dan konversi video
4. **Web Dashboard** - Antarmuka web untuk mengontrol dan memantau proses scraping dan download

### Mengakses Web Dashboard

Setelah menjalankan aplikasi, kamu bisa mengakses dashboard web di:

```
http://localhost:8080
```

Melalui dashboard ini, kamu bisa:

- Memulai dan menghentikan proses scraping
- Mengatur konfigurasi seperti halaman yang akan di-scrape dan jumlah concurrent download
- Memantau proses scraping dan download secara real-time melalui WebSocket

## Gimana Cara Kerjanya?

1. **Persiapan**: Aplikasi bakal muat konfigurasi dari file konfigurasi dan nyiapin komponen yang dibutuhkan.

2. **Scraper Service**:

   - Nyiapin browser Chrome tanpa tampilan
   - Ngunjungin beberapa halaman dari URL target (dari halaman FROM_PAGES sampai TO_PAGES)
   - Nyari link ke halaman video
   - Nyari URL stream video M3U8 dari setiap halaman dengan sistem retry otomatis
   - Kirim data video ke RabbitMQ queue
   - Ekspor semua link yang udah di-scrape ke file JSON
   - Kirim log dan statistik ke service web melalui RabbitMQ

3. **Downloader Service**:

   - Terima data video dari RabbitMQ queue
   - Download file playlist M3U8 dengan mekanisme retry
   - Download semua potongan video secara paralel dengan multi-worker
   - Pake FFmpeg buat gabungin jadi satu file MP4 utuh
   - Bersihin file-file sementara secara otomatis setelah konversi berhasil
   - Kirim log dan progress download ke service web melalui RabbitMQ

4. **Web Dashboard Service**:
   - Nyediain antarmuka web untuk mengontrol dan memantau proses
   - Terima perintah dari user (mulai/stop scraping, update konfigurasi)
   - Kirim perintah ke service lain melalui RabbitMQ
   - Terima dan tampilkan log dan statistik dari service lain secara real-time melalui WebSocket

## Struktur Folder

```
├── CONFIG.md                # Dokumentasi konfigurasi
├── README.md                # Dokumentasi utama
├── cmd/                     # Entry points aplikasi
│   ├── downloader/          # Entry point service downloader
│   ├── scraper/             # Entry point service scraper
│   └── web/                 # Entry point service web dashboard
├── config.example.json      # Contoh file konfigurasi JSON
├── deployments/             # File-file deployment
│   ├── docker-compose.yaml  # Konfigurasi Docker Compose
│   └── docker/              # Dockerfile untuk setiap service
├── internal/                # Kode internal aplikasi
│   ├── common/              # Komponen yang digunakan bersama
│   │   ├── config/          # Konfigurasi aplikasi
│   │   ├── logger/          # Logger aplikasi
│   │   └── messaging/       # Komunikasi dengan RabbitMQ
│   ├── downloader/          # Service downloader
│   ├── scraper/             # Service scraper
│   └── web/                 # Service web dashboard
│       ├── handler/         # Handler HTTP
│       ├── static/          # File statis (CSS, JS)
│       ├── templates/       # Template HTML
│       └── websocket/       # Implementasi WebSocket
├── pkg/                     # Package yang bisa digunakan publik
│   ├── models/              # Model data
│   └── utils/               # Utilitas
└── static/                  # File statis publik
```
