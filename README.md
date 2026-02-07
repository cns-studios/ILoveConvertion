# ILoveConvertion

### Will be available soon!

**ILoveConvertion** (also known as FileForge) is a high-performance, privacy-focused file processing engine. It allows users to convert, compress, and transform images, audio, video, and PDF files through a modern, responsive web interface or a RESTful API.

All files are **encrypted at rest** using AES-256 with unique per-job keys derived from a master secret. Files are automatically deleted from the server after 24 hours.

## Features

- **Image Transformation**:
  - Convert between formats: JPEG, PNG, WebP, TIFF, GIF, AVIF, HEIF, BMP.
  - Smart compression with quality control and lossless options.
  - **AI Background Removal**: Powered by `rembg` (Silueta model).
- **Audio Processing**:
  - Convert between: MP3, WAV, FLAC, OGG, OPUS, AAC, M4A, AIFF.
  - Compression with VBR/CBR control.
- **Video Compression**:
  - Optimize MP4, MKV, and WebM using H.264 and VP9 codecs.
- **PDF Optimization**:
  - Multi-stage compression using Ghostscript and QPDF.
  - Resolution and quality presets (Screen, Ebook, Printer, Prepress).
- **Security First**:
  - End-to-end storage encryption.
  - Session-based job isolation.

## Architecture

FileForge is built as a distributed system using Docker:

- **API (Go)**: Handles file uploads, job creation, and file serving.
- **Worker (Go)**: Processes the task queue, utilizing system tools (`ffmpeg`, `libvips`, `gs`, `qpdf`).
- **Rembg Service (Python/Flask)**: A specialized microservice for AI-powered background removal.
- **Queue (Redis)**: Manages job distribution between API and Workers.
- **Database (PostgreSQL)**: Stores job metadata and session information.
- **Nginx**: Acts as a reverse proxy and serves the static frontend.

## Deployment (Self-Hosting)

### Prerequisites

- [Docker](https://docs.docker.com/get-docker/)
- [Docker Compose](https://docs.docker.com/compose/install/)

### Quick Start

1. **Clone the repository**:
   ```bash
   git clone https://github.com/cns-studios/ILoveConvertion.git
   cd ILoveConvertion
   ```

2. **Configure environment**:
   ```bash
   cp .env.example .env
   # Edit .env and set a strong MASTER_KEY
   ```

3. **Launch the stack**:
   ```bash
   docker-compose up -d --build
   ```

The application will be available at `http://localhost:8080` (or the port specified in your `.env`).

## Configuration

Key settings in `.env`:

- `ENCRYPTION_MASTER_KEY`: 32-character hex string used for file encryption (generate with `openssl rand -hex 32`).
- `MAX_FILE_SIZE`: Maximum upload size in bytes (default 500MB).
- `FILE_RETENTION_HOURS`: How long to keep files before auto-deletion (default 24).
- `WORKER_CONCURRENCY`: Number of simultaneous jobs per worker (default 4).
- `TMPFS_SIZE`: Size of the RAM-disk used for processing (default 1GB).

## API Usage

### Create a Job
`POST /api/jobs` (Multipart Form)
- `file`: The file to process.
- `operation`: e.g., `image_convert`, `pdf_compress`.
- `output_format`: desired extension.
- `quality`: 1-100 (optional).

### Check Status
`GET /api/jobs/{id}`

### Download Result
`GET /api/jobs/{id}/download`
