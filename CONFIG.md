# Konfigurasi WleoWleo

## Konfigurasi Terpusat

Proyek ini sekarang menggunakan file konfigurasi terpusat (`.env`) yang terletak di root project. File ini berisi semua variabel lingkungan yang dibutuhkan oleh kedua service (scraper dan downloader).

## Konfigurasi Aplikasi

### Konfigurasi Umum

- `APP_NAME`: Nama aplikasi (default: wleowleo)
- `LOG_LEVEL`: Level logging (1-5, dimana 5 adalah level paling detail)
- `APP_ENV`: Environment aplikasi (development/production)

### RabbitMQ Configuration

- `RABBITMQ_URL`: URL koneksi RabbitMQ (format: amqp://user:password@host:port/)
- `RABBITMQ_EXCHANGE`: Nama exchange RabbitMQ (default: task_exchange)
- `RABBITMQ_QUEUE_SCRAPER`: Nama queue untuk scraper (default: scraper_queue)
- `RABBITMQ_QUEUE_DOWNLOADER`: Nama queue untuk downloader (default: downloader_queue)
- `RABBITMQ_QUEUE_DOWNLOAD_TASK`: Nama queue untuk tugas download (default: download_task_queue)
- `RABBITMQ_QUEUE_LOG`: Nama queue untuk log (default: log_queue)
- `RABBITMQ_RECONNECT_RETRIES`: Jumlah percobaan reconnect (default: 5)
- `RABBITMQ_RECONNECT_TIMEOUT`: Timeout untuk reconnect dalam milidetik (default: 10000)

### Scraper Configuration

- `SCRAPER_HOST`: URL dasar website target
- `SCRAPER_USER_AGENT`: User agent untuk bypass cloudflare
- `FROM_PAGES`: Halaman awal yang akan di-scrape (default: 1)
- `TO_PAGES`: Halaman akhir yang akan di-scrape (default: 5)

### Downloader Configuration

- `DOWNLOADER_DEFAULT_WORKER`: Batas jumlah concurrent download (default: 5) - mengontrol jumlah worker paralel untuk mengoptimalkan kinerja download
- `DOWNLOADER_TEMP_DIR`: Direktori untuk menyimpan file sementara (default: ./temp)
- `DOWNLOADER_DOWNLOAD_DIR`: Direktori untuk menyimpan hasil download (default: ./downloads)

### Web Dashboard Configuration

- `WEBPANEL_HOST`: Host untuk web dashboard (default: localhost)
- `WEBPANEL_PORT`: Port untuk web dashboard (default: 8080)

## Cara Menggunakan

### Menggunakan File JSON

1. Salin file `config.example.json` ke `config.json`:

   ```bash
   cp config.example.json config.json
   ```

2. Edit file `config.json` sesuai kebutuhan:
   ```bash
   nano config.json
   ```

### Menggunakan Variabel Lingkungan

Alternatif lain, kamu bisa menggunakan variabel lingkungan untuk mengatur konfigurasi. Variabel lingkungan akan menimpa nilai yang ada di file `config.json`.

1. Buat file `.env` di root project:

   ```bash
   touch .env
   ```

2. Tambahkan variabel lingkungan yang diperlukan:

   ```bash
   nano .env
   ```

3. Jalankan dengan Docker Compose:
   ```bash
   docker compose up
   ```

## Contoh File Konfigurasi

### Contoh config.json

```json
{
  "app": {
    "name": "wleowleo",
    "logLevel": 4,
    "env": "development"
  },
  "rabbitmq": {
    "url": "amqp://guest:guest@localhost:5672/",
    "exchange": "task_exchange",
    "queue": {
      "scraper": "scraper_queue",
      "downloader": "downloader_queue",
      "downloadTask": "download_task_queue",
      "log": "log_queue"
    },
    "reconnectRetries": 5,
    "reconnectTimeout": 10000
  },
  "scraper": {
    "host": "URL_TARGET",
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

Dengan konfigurasi terpusat ini, pengelolaan konfigurasi menjadi lebih mudah karena semua pengaturan berada dalam satu file.
