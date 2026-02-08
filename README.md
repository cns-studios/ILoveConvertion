# ILoveConvertion

### Will be available soon!

**ILoveConvertion** is a high-performance, privacy-focused file processing engine designed for modern web environments. It provides a robust infrastructure to convert, compress, and transform various media types while maintaining strict security standards.

All files are **encrypted at rest** using AES-256-GCM with unique keys derived for every single job. The system is built for speed, utilizing RAM-disks (tmpfs) for all intermediate processing to ensure zero data traces on physical disks during transformation.

## Features

- **Image Transformation**:
  - Format Conversion: JPEG, PNG, WebP, TIFF, GIF, AVIF, HEIF, BMP.
  - High-Efficiency Compression: Lossless and lossy modes with fine-grained quality control.
  - **AI Background Removal**: Seamless integration with the `rembg` microservice using the Silueta model.
- **PDF Optimization**:
  - Multi-stage pipeline using Ghostscript and QPDF.
  - Smart presets: Screen (72 DPI), Ebook (150 DPI), Printer (300 DPI), and Prepress.
- **Audio Processing**:
  - Support for MP3, WAV, FLAC, OGG, OPUS, AAC, M4A, AIFF.
  - Variable and Constant Bitrate (VBR/CBR) control.
- **Video Compression**:
  - Optimized for the web using H.264 (MP4/MKV) and VP9 (WebM).
  - Advanced CRF-based bitrate management and metadata stripping.
- **Privacy First**:
  - Every file is encrypted immediately upon arrival.
  - Automatic 24-hour retention policy with secure deletion.
  - Session-based isolation to prevent cross-user data leakage.

## Architecture

ILC operates as a distributed microservices architecture:

- **API (Go)**: A gateway handling uploads, job management, and file serving.
- **Worker (Go)**: It manages the processing lifecycle and orchestrates system tools like `libvips`, `ffmpeg`, `ghostscript`, and `qpdf`.
- **Rembg Service (Python)**: An AI service dedicated to background removal tasks.
- **Redis**: The backbone for the asynchronous job queue and internal messaging.
- **PostgreSQL**: Stores job metadata, session states, and audit trails.
- **Nginx**: Provides reverse proxying and serves the frontend.

## Quick Start (Docker)

1. **Clone & Enter**:
   ```bash
   git clone https://github.com/cns-studios/ILoveConvertion.git && cd ILoveConvertion
   ```

2. **Environment Setup**:
   ```bash
   cp .env.example .env
   # Ensure ENCRYPTION_MASTER_KEY is set to a 32-character hex string
   ```

3. **Deployment**:
   ```bash
   docker-compose up -d --build
   ```

## Data Processing Pipeline

### 1. Ingestion & Encryption
When a file is uploaded via the `/api/jobs` endpoint:
- The system generates a cryptographically secure **Job ID**.
- A unique **Encryption Key** is derived using HKDF-SHA256 from the global `MASTER_KEY` and the `JobID`.
- The raw stream is encrypted on-the-fly using **AES-256-GCM** in 64KB chunks before it ever touches the persistent storage (`/storage/inputs`).

### 2. Asynchronous Queuing
Once the encrypted input is stored, a job manifest is recorded in PostgreSQL, and the `JobID` is pushed into a **Redis-backed queue**. This allows the API to remain responsive regardless of the file size or processing complexity.

### 3. Secure Worker Processing
A Worker picks up the `JobID` and performs the following:
- **Sandbox Creation**: A temporary directory is created in a RAM-disk (`tmpfs`). This ensures that intermediate, unencrypted files never touch a physical SSD/HDD.
- **Decryption**: The worker derives the job-specific key and decrypts the input file from storage into the RAM-disk.
- **Orchestration**: Depending on the requested operation, the worker invokes the appropriate processor:
    - **Images**: Utilizes `libvips` (via `bimg`) for memory-efficient transformations or `pngquant` for lossy optimization.
    - **PDFs**: Runs a two-pass optimization using `Ghostscript` for content downsampling and `QPDF` for linearization and stream compression.
    - **Audio/Video**: Leverages `ffmpeg` with optimized presets for high-quality, low-bitrate output.
    - **AI Tasks**: For background removal, the file is securely streamed to the internal Rembg microservice.

### 4. Finalization & Output
The resulting file in the RAM-disk is:
- **Re-encrypted**: Using the same job-specific key before being moved to the persistent output storage (`/storage/outputs`).
- **Validated**: The system checks the integrity and size of the output.
- **Cleaned Up**: The RAM-disk sandbox is immediately wiped.

### 5. Automated Cleanup
A background task runs every 15 minutes to identify jobs older than the configured `FILE_RETENTION_HOURS` (default 24h). It removes the database records and triggers a secure deletion of both input and output encrypted files from the storage volume.