# WleoWleo Scraper

<div align="center">
<img src="https://pbs.twimg.com/media/Fjv-7DQVIAAnqdn.jpg" alt="drawing" width="250"/>

*Hai! Ini adalah aplikasi Go buat nyedot dan download video-video wleowleo yang mantap-mantap itu lho. Kalian gak usah ngerti linknya ya, nanti dosa 😉. Tool ini bakal otomatis nyari link video dan download-nya dalam format MP4 buat kalian.*

</div>


## Fitur Keren

- Web scraping pake browser headless (browser tanpa tampilan) dengan ChromeDP
- Nyedot link video dari halaman web secara otomatis
- Download video langsung dalam format MP4
- Simpen semua link yang udah di-scrape ke file JSON
- Bisa diatur-atur lewat file konfigurasi

## Yang Harus Ada

- Go 1.23 ke atas
- FFmpeg (buat proses download video)
- Browser Chrome/Chromium

## Cara Pasang

### Cara 1: Pasang Langsung

1. Clone repo-nya dulu:

```bash
git clone https://github.com/rizkirmdhnnn/wleowleo.git
cd wleowleo
```

2. Install yang dibutuhin:

```bash
go mod download
```

3. Pastiin FFmpeg udah terpasang di komputer kamu (penting buat download video):

```bash
# Buat pengguna macOS (pake Homebrew)
brew install ffmpeg

# Buat pengguna Ubuntu/Debian
sudo apt-get install ffmpeg

# Buat pengguna Windows
# Download aja dari https://ffmpeg.org/download.html
```

### Cara 2: Pake Docker

Kalo males install Go, Chrome, dan FFmpeg, bisa pake Docker aja. Lebih gampang!

#### Persiapan

1. Pastiin [Docker](https://www.docker.com/products/docker-desktop/) dan [Docker Compose](https://docs.docker.com/compose/install/) udah terpasang di komputer kamu.

2. Clone repo-nya dulu:

```bash
git clone https://github.com/rizkirmdhnnn/wleowleo.git
cd wleowleo
```

3. Bikin file `.env` (lihat bagian "Setting Konfigurasi" di bawah).

4. Bikin folder `output` dan `temp` (kalo belum ada):

```bash
mkdir -p output temp
```

#### Cara Jalanin

**Pake Docker Compose (Direkomendasikan):**

```bash
docker compose up
```

Atau kalo mau jalanin di background:

```bash
docker compose up -d
```

**Pake Docker Langsung:**

```bash
# Build image dulu
docker build -t wleowleo .

# Jalanin container
docker run --rm -v "$(pwd)/output:/app/output" -v "$(pwd)/temp:/app/temp" -v "$(pwd)/.env:/app/.env:ro" wleowleo
```

Semua hasil scraping dan video bakal disimpen di folder `output` di komputer kamu.

## Setting Konfigurasi

Bikin file `.env` di folder utama. Contek aja dari `.env.example` yang udah ada:

```
# Mau download video otomatis? (true/false)
AUTO_DOWNLOAD = true

# URL website target
BASE_URL = "dosa gaboleh tau"

# User agent buat ngehindarin Cloudflare
USERAGENT = "Mozilla/5.0 (Linux; Android 6.0; Nexus 5 Build/MRA58N) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/134.0.0.0 Mobile Safari/537.36"

# Mau scrape berapa halaman?
PAGES = "1"
```

## Cara Pake

Jalanin aplikasinya:

```bash
go run main.go
```

Atau build dulu terus jalanin:

```bash
go build -o wleowleo
./wleowleo
```

## Gimana Cara Kerjanya?

1. **Persiapan**: Aplikasi bakal muat konfigurasi dari file `.env` dan nyiapin browser Chrome tanpa tampilan.

2. **Nyedot Halaman**: Aplikasi bakal ngunjungin beberapa halaman dari URL target dan nyari link ke halaman video.

3. **Nyari Link Video**: Buat setiap link halaman, aplikasi bakal buka halaman itu dan nyomot URL stream video M3U8-nya.

4. **Ekspor**: Semua link yang udah di-scrape bakal disimpen ke file JSON di folder `output`.

5. **Download** (kalo diaktifin): Kalo `AUTO_DOWNLOAD` diset `true`, aplikasi bakal download tiap video:
   - Download file playlist M3U8
   - Download semua potongan video
   - Pake FFmpeg buat gabungin jadi satu file MP4 utuh
   - Bersihin file-file sementara

## Struktur Folder

- `config/`: Buat ngurus konfigurasi
- `logger/`: Buat nyatet log aktivitas
- `scraper/`: Inti dari aplikasi buat scraping dan download
  - `scraper.go`: Nyedot halaman dan link video
  - `downloader.go`: Download dan konversi video
  - `export.go`: Ekspor data ke JSON
- `output/`: Tempat nyimpen video hasil download dan file JSON
- `temp/`: Folder sementara buat nyimpen potongan video selama proses
