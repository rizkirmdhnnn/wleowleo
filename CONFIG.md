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
- `PAGES`: Jumlah halaman yang akan di-scrape
- `AUTO_DOWNLOAD`: Apakah akan otomatis mendownload video (true/false)
- `LIMIT_CONCURRENT`: Batas jumlah concurrent scraping

### Downloader Configuration
- `LIMIT_CONCURRENT_DOWNLOAD`: Batas jumlah concurrent download

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

Dengan konfigurasi terpusat ini, pengelolaan variabel lingkungan menjadi lebih mudah karena semua konfigurasi berada dalam satu file.