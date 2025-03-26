# WleoWleo Web Dashboard

Web dashboard for WleoWleo scraper and downloader services. This dashboard allows you to configure and monitor the scraping and downloading processes in real-time.

## Features

- Configure scraping parameters (from page, to page, concurrent downloads)
- Real-time monitoring of scraper and downloader logs via WebSocket
- Statistics display for total pages scraped, links found, and videos downloaded
- Start/stop scraping and downloading processes
- Modern UI with Tailwind CSS

## Architecture

The web dashboard is built with:

- Backend: Go with Gin framework
- Frontend: HTML, JavaScript, and Tailwind CSS
- Real-time updates: WebSocket for live log streaming
- Configuration: Shared .env file with other services

## Usage

The web dashboard is included in the docker-compose.yaml file and will start automatically when you run:

```bash
docker compose up
```

Access the dashboard at http://localhost:8080

## Configuration

The web service uses the same .env file as the other services. The following environment variables are specific to the web service:

```
# Web Configuration
WEB_PORT=8080
```

Other configuration options are shared with the scraper and downloader services.