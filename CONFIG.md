# Konfigurasi WleoWleo

## Konfigurasi Terpusat

Proyek ini sekarang menggunakan file konfigurasi terpusat (`.env`) yang terletak di root project. File ini berisi semua variabel lingkungan yang dibutuhkan oleh kedua service (scraper dan downloader).

## Variabel Lingkungan

### RabbitMQ Configuration
- `RABBITMQ_HOST`: Host RabbitMQ (default: rabbitmq)
- `RABBITMQ_PORT`: Port RabbitMQ (default: 5672)
- `RABBITMQ_USER`: Username RabbitMQ (default: guest)
- `RABBITMQ_PASSWORD`: Password RabbitMQ (default: guest)
- `RABBITMQ_QUEUE`: Nama queue RabbitMQ (default: video_queue)

### Scraper Configuration
- `BASE_URL`: URL dasar website target
- `USERAGENT`: User agent untuk bypass cloudflare
- `FROM_PAGES`: Halaman awal yang akan di-scrape (default: 1)
- `TO_PAGES`: Halaman akhir yang akan di-scrape (default: 5)
- `LOG_LEVEL`: Level logging (1-5, dimana 5 adalah level paling detail)

### Downloader Configuration
- `LIMIT_CONCURRENT_DOWNLOAD`: Batas jumlah concurrent download (default: 5) - mengontrol jumlah worker paralel untuk mengoptimalkan kinerja download

## Cara Menggunakan

1. Salin file `.env.example` ke `.env`:
   ```bash
   cp .env.example .env
   ```

2. Edit file `.env` sesuai kebutuhan:
   ```bash
   nano .env
   ```

3. Jalankan dengan Docker Compose:
   ```bash
   docker compose up
   ```

## Contoh File .env

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

Dengan konfigurasi terpusat ini, pengelolaan variabel lingkungan menjadi lebih mudah karena semua konfigurasi berada dalam satu file.