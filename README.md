# WleoWleo Scraper

<div align="center">
<img src="https://pbs.twimg.com/media/Fjv-7DQVIAAnqdn.jpg" alt="drawing" width="250"/>

*Hai! Ini adalah aplikasi Go buat nyedot dan download video-video wleowleo yang mantap-mantap itu lho. Kalian gak usah ngerti linknya ya, nanti dosa 😉. Tool ini bakal otomatis nyari link video dan download-nya dalam format MP4 buat kalian.*

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

3. Bikin file `.env` (lihat bagian "Konfigurasi" di bawah):

```bash
cp .env.example .env
# Edit file .env sesuai kebutuhan
```

4. Bikin folder `output` dan `temp` (kalo belum ada):

```bash
mkdir -p output temp
```

### Cara Jalanin

**Pake Docker Compose (Direkomendasikan):**

```bash
docker compose up
```

Atau kalo mau jalanin di background:

```bash
docker compose up -d
```

Semua hasil scraping dan video bakal disimpen di folder `output` di komputer kamu.

## Konfigurasi

Proyek ini menggunakan file konfigurasi terpusat (`.env`) yang terletak di root project. File ini berisi semua variabel lingkungan yang dibutuhkan oleh kedua service (scraper dan downloader).

Bikin file `.env` di folder utama. Contek aja dari `.env.example` yang udah ada:

```
# RabbitMQ Configuration
RABBITMQ_HOST=rabbitmq
RABBITMQ_PORT=5672
RABBITMQ_USER=guest
RABBITMQ_PASSWORD=guest
RABBITMQ_QUEUE=video_queue

# Scraper Configuration
BASE_URL="" #berdosa loh gaboleh
USERAGENT="Mozilla/5.0 (Linux; Android 6.0; Nexus 5 Build/MRA58N) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/134.0.0.0 Mobile Safari/537.36"
FROM_PAGES=1
TO_PAGES=5
LOG_LEVEL="4"

# Downloader Configuration
LIMIT_CONCURRENT_DOWNLOAD=5
```

Untuk penjelasan lebih detail tentang konfigurasi, lihat file [CONFIG.md](CONFIG.md).

## Cara Pake

### Menjalankan sebagai Microservices

Cara paling mudah adalah menggunakan Docker Compose:

```bash
docker compose up
```

Ini akan menjalankan tiga service:
1. **RabbitMQ** - Message broker untuk komunikasi antar service
2. **Scraper** - Service untuk scraping link video
3. **Downloader** - Service untuk download dan konversi video

## Gimana Cara Kerjanya?

1. **Persiapan**: Aplikasi bakal muat konfigurasi dari file `.env` dan nyiapin komponen yang dibutuhkan.

2. **Scraper Service**:
   - Nyiapin browser Chrome tanpa tampilan
   - Ngunjungin beberapa halaman dari URL target (dari halaman FROM_PAGES sampai TO_PAGES)
   - Nyari link ke halaman video
   - Nyari URL stream video M3U8 dari setiap halaman dengan sistem retry otomatis
   - Kirim data video ke RabbitMQ queue
   - Ekspor semua link yang udah di-scrape ke file JSON

3. **Downloader Service**:
   - Terima data video dari RabbitMQ queue
   - Download file playlist M3U8 dengan mekanisme retry
   - Download semua potongan video secara paralel dengan multi-worker
   - Pake FFmpeg buat gabungin jadi satu file MP4 utuh
   - Bersihin file-file sementara secara otomatis setelah konversi berhasil

## Struktur Folder

```
├── CONFIG.md                # Dokumentasi konfigurasi
├── README.md                # Dokumentasi utama
├── docker-compose.yaml      # Konfigurasi Docker Compose
├── .env.example             # Contoh file konfigurasi
├── downloader/              # Service downloader
│   ├── Dockerfile           # Dockerfile untuk downloader
│   ├── config/              # Konfigurasi downloader
│   ├── logger/              # Logger untuk downloader
│   ├── message/             # Komunikasi dengan RabbitMQ
│   ├── scraper/             # Modul download dan konversi video
│   └── main.go              # Entry point downloader
├── scraper/                 # Service scraper
│   ├── Dockerfile           # Dockerfile untuk scraper
│   ├── config/              # Konfigurasi scraper
│   ├── logger/              # Logger untuk scraper
│   ├── message/             # Komunikasi dengan RabbitMQ
│   ├── scraper/             # Modul scraping dan ekspor data
│   └── main.go              # Entry point scraper
```
