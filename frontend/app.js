(function () {
    'use strict';

    const state = {
        activeTool: 'image_convert',
        selectedFile: null,
        jobId: null,
        pollTimer: null,
        formats: null,
        uploading: false,
    };

    const $ = (sel) => document.querySelector(sel);
    const $$ = (sel) => document.querySelectorAll(sel);

    const dom = {
        tabs: () => $$('.tab'),
        dropzone: $('#dropzone'),
        fileInput: $('#file-input'),
        fileInfo: $('#file-info'),
        fileName: $('#file-name'),
        fileSize: $('#file-size'),
        fileClear: $('#file-clear'),
        acceptedFormats: $('#accepted-formats'),
        optionsPanel: $('#options-panel'),
        btnProcess: $('#btn-process'),
        btnText: $('#btn-text'),
        progressSection: $('#progress-section'),
        progressLabel: $('#progress-label'),
        progressPct: $('#progress-pct'),
        progressBar: $('#progress-bar'),
        progressDetail: $('#progress-detail'),
        resultSection: $('#result-section'),
        resultSuccess: $('#result-success'),
        resultError: $('#result-error'),
        resultInputSize: $('#result-input-size'),
        resultOutputSize: $('#result-output-size'),
        resultSavings: $('#result-savings'),
        resultDownload: $('#result-download'),
        errorMessage: $('#error-message'),
        btnRetry: $('#btn-retry'),
    };

    const toolConfig = {
        image_convert: {
            label: 'Convert Image',
            action: 'Convert',
            options: ['output_format'],
        },
        image_compress: {
            label: 'Compress Image',
            action: 'Compress',
            options: ['quality', 'lossless'],
        },
        image_remove_bg: {
            label: 'Remove Background',
            action: 'Remove Background',
            options: ['output_format_bg'],
        },
        pdf_compress: {
            label: 'Compress PDF',
            action: 'Compress',
            options: ['image_dpi', 'image_quality'],
        },
        audio_convert: {
            label: 'Convert Audio',
            action: 'Convert',
            options: ['output_format'],
        },
        audio_compress: {
            label: 'Compress Audio',
            action: 'Compress',
            options: ['quality', 'lossless'],
        },
        video_compress: {
            label: 'Compress Video',
            action: 'Compress',
            options: ['output_format', 'quality'],
        },
    };

    async function init() {
        await loadFormats();
        bindEvents();
        switchTool('image_convert');
    }

    async function loadFormats() {
        try {
            const res = await fetch('/api/formats');
            if (res.ok) {
                state.formats = await res.json();
            }
        } catch (e) {
        }

        if (!state.formats) {
            state.formats = {
                image_convert: {
                    input: ['jpeg', 'jpg', 'png', 'webp', 'tiff', 'tif', 'gif', 'avif', 'heif', 'heic', 'bmp'],
                    output: ['jpeg', 'png', 'webp', 'tiff', 'gif', 'avif', 'heif', 'bmp'],
                },
                image_compress: {
                    input: ['jpeg', 'jpg', 'png', 'webp', 'tiff', 'tif', 'gif', 'avif', 'heif', 'heic', 'bmp'],
                },
                image_remove_bg: {
                    input: ['jpeg', 'jpg', 'png', 'webp', 'tiff', 'tif', 'bmp'],
                    output: ['png', 'webp'],
                    default_output: 'png',
                },
                pdf_compress: { input: ['pdf'], output: ['pdf'] },
                audio_convert: {
                    input: ['mp3', 'wav', 'flac', 'ogg', 'opus', 'aac', 'm4a', 'aiff', 'wma'],
                    output: ['mp3', 'wav', 'flac', 'ogg', 'opus', 'aac', 'm4a', 'aiff'],
                },
                audio_compress: {
                    input: ['mp3', 'wav', 'flac', 'ogg', 'opus', 'aac', 'm4a', 'aiff', 'wma'],
                },
                video_compress: {
                    input: ['mp4', 'mkv', 'webm', 'avi', 'mov'],
                    output: ['mp4', 'mkv', 'webm'],
                },
            };
        }
    }

    function bindEvents() {
        dom.tabs().forEach((tab) => {
            tab.addEventListener('click', () => switchTool(tab.dataset.tool));
        });

        dom.fileInput.addEventListener('change', handleFileSelect);

        dom.dropzone.addEventListener('click', () => dom.fileInput.click());
        dom.dropzone.addEventListener('dragover', (e) => {
            e.preventDefault();
            dom.dropzone.classList.add('drag-over');
        });
        dom.dropzone.addEventListener('dragleave', () => {
            dom.dropzone.classList.remove('drag-over');
        });
        dom.dropzone.addEventListener('drop', (e) => {
            e.preventDefault();
            dom.dropzone.classList.remove('drag-over');
            if (e.dataTransfer.files.length > 0) {
                setFile(e.dataTransfer.files[0]);
            }
        });

        dom.fileClear.addEventListener('click', (e) => {
            e.stopPropagation();
            clearFile();
        });

        dom.btnProcess.addEventListener('click', startProcessing);

        dom.btnRetry.addEventListener('click', () => {
            resetUI();
        });
    }

    function switchTool(tool) {
        state.activeTool = tool;

        dom.tabs().forEach((tab) => {
            const isActive = tab.dataset.tool === tool;
            tab.classList.toggle('active', isActive);
            tab.setAttribute('aria-selected', isActive);
        });

        updateAcceptedFormats();
        buildOptionsPanel();
        clearFile();
        resetUI();
        updateFileInputAccept();

        const workspace = $('.workspace');
        workspace.classList.remove('slide-content');
        void workspace.offsetWidth;
        workspace.classList.add('slide-content');
    }

    function updateAcceptedFormats() {
        const toolFormats = state.formats[state.activeTool];
        if (toolFormats && toolFormats.input) {
            const exts = toolFormats.input.map((f) => '.' + f).join(', ');
            dom.acceptedFormats.textContent = 'Accepts: ' + exts;
        } else {
            dom.acceptedFormats.textContent = '';
        }
    }

    function updateFileInputAccept() {
        const toolFormats = state.formats[state.activeTool];
        if (toolFormats && toolFormats.input) {
            const mimeMap = {
                jpeg: 'image/jpeg', jpg: 'image/jpeg', png: 'image/png',
                webp: 'image/webp', tiff: 'image/tiff', tif: 'image/tiff',
                gif: 'image/gif', avif: 'image/avif', heif: 'image/heif',
                heic: 'image/heic', bmp: 'image/bmp', pdf: 'application/pdf',
                mp3: 'audio/mpeg', wav: 'audio/wav', flac: 'audio/flac',
                ogg: 'audio/ogg', opus: 'audio/opus', aac: 'audio/aac',
                m4a: 'audio/mp4', aiff: 'audio/aiff', wma: 'audio/x-ms-wma',
                mp4: 'video/mp4', mkv: 'video/x-matroska', webm: 'video/webm',
                avi: 'video/x-msvideo', mov: 'video/quicktime',
            };
            const accepts = toolFormats.input
                .map((f) => mimeMap[f] || '.' + f)
                .join(',');
            dom.fileInput.setAttribute('accept', accepts);
        }
    }

    function buildOptionsPanel() {
        const config = toolConfig[state.activeTool];
        if (!config || !config.options || config.options.length === 0) {
            dom.optionsPanel.classList.add('hidden');
            return;
        }

        let html = '';

        config.options.forEach((opt) => {
            switch (opt) {
                case 'output_format':
                    html += buildFormatSelect();
                    break;
                case 'output_format_bg':
                    html += buildBgFormatSelect();
                    break;
                case 'quality':
                    html += buildQualitySlider();
                    break;
                case 'image_quality':
                    html += buildImageQualitySlider();
                    break;
                case 'lossless':
                    html += buildLosslessCheckbox();
                    break;
                case 'image_dpi':
                    html += buildDpiSelect();
                    break;
            }
        });

        dom.optionsPanel.innerHTML = html;
        dom.optionsPanel.classList.remove('hidden');
        bindOptionEvents();
    }

    function buildFormatSelect() {
        const toolFormats = state.formats[state.activeTool];
        const outputs = toolFormats && toolFormats.output ? toolFormats.output : [];
        if (outputs === 'same_as_input' || outputs.length === 0) return '';

        const options = outputs
            .map((f) => `<option value="${f}">${f.toUpperCase()}</option>`)
            .join('');

        return `
            <div class="option-group">
                <label class="option-label">Output Format</label>
                <select class="option-select" id="opt-output-format">${options}</select>
            </div>`;
    }

    function buildBgFormatSelect() {
        const toolFormats = state.formats[state.activeTool];
        const outputs = toolFormats && toolFormats.output ? toolFormats.output : ['png', 'webp'];
        const defaultOut = (toolFormats && toolFormats.default_output) || 'png';

        const options = outputs
            .map((f) => `<option value="${f}" ${f === defaultOut ? 'selected' : ''}>${f.toUpperCase()}</option>`)
            .join('');

        return `
            <div class="option-group">
                <label class="option-label">Output Format</label>
                <select class="option-select" id="opt-output-format">${options}</select>
            </div>`;
    }

    function buildQualitySlider() {
        const defaults = { image_compress: 80, audio_compress: 70, video_compress: 65 };
        const defaultVal = defaults[state.activeTool] || 75;

        return `
            <div class="option-group">
                <label class="option-label">Quality</label>
                <div class="option-range-row">
                    <span class="option-range-value" style="min-width:5ch;text-align:left;color:var(--text-muted);font-weight:400">Smaller</span>
                    <input type="range" class="option-range" id="opt-quality" min="1" max="100" value="${defaultVal}">
                    <span class="option-range-value" id="opt-quality-value">${defaultVal}</span>
                    <span class="option-range-value" style="min-width:5ch;text-align:right;color:var(--text-muted);font-weight:400">Better</span>
                </div>
            </div>`;
    }

    function buildImageQualitySlider() {
        return `
            <div class="option-group">
                <label class="option-label">Image Quality in PDF</label>
                <div class="option-range-row">
                    <span class="option-range-value" style="min-width:5ch;text-align:left;color:var(--text-muted);font-weight:400">Smaller</span>
                    <input type="range" class="option-range" id="opt-image-quality" min="1" max="100" value="75">
                    <span class="option-range-value" id="opt-image-quality-value">75</span>
                    <span class="option-range-value" style="min-width:5ch;text-align:right;color:var(--text-muted);font-weight:400">Better</span>
                </div>
            </div>`;
    }

    function buildLosslessCheckbox() {
        return `
            <div class="option-group">
                <div class="option-checkbox-row">
                    <input type="checkbox" class="option-checkbox" id="opt-lossless">
                    <label class="option-checkbox-label" for="opt-lossless">Lossless compression (if supported by format)</label>
                </div>
            </div>`;
    }

    function buildDpiSelect() {
        return `
            <div class="option-group">
                <label class="option-label">Image DPI in PDF</label>
                <select class="option-select" id="opt-image-dpi">
                    <option value="72">72 DPI — Smallest</option>
                    <option value="150" selected>150 DPI — Balanced</option>
                    <option value="300">300 DPI — High Quality</option>
                    <option value="600">600 DPI — Maximum</option>
                </select>
            </div>`;
    }

    function bindOptionEvents() {
        const qualitySlider = $('#opt-quality');
        if (qualitySlider) {
            qualitySlider.addEventListener('input', () => {
                $('#opt-quality-value').textContent = qualitySlider.value;
            });
        }

        const imgQualitySlider = $('#opt-image-quality');
        if (imgQualitySlider) {
            imgQualitySlider.addEventListener('input', () => {
                $('#opt-image-quality-value').textContent = imgQualitySlider.value;
            });
        }

        const losslessCheckbox = $('#opt-lossless');
        if (losslessCheckbox && qualitySlider) {
            losslessCheckbox.addEventListener('change', () => {
                qualitySlider.disabled = losslessCheckbox.checked;
                qualitySlider.style.opacity = losslessCheckbox.checked ? '0.3' : '1';
            });
        }
    }

    function gatherParams() {
        const params = {};
        const formatSelect = $('#opt-output-format');
        if (formatSelect) params.output_format = formatSelect.value;

        const quality = $('#opt-quality');
        if (quality) params.quality = parseInt(quality.value, 10);

        const lossless = $('#opt-lossless');
        if (lossless) params.lossless = lossless.checked;

        const dpi = $('#opt-image-dpi');
        if (dpi) params.image_dpi = parseInt(dpi.value, 10);

        const imgQuality = $('#opt-image-quality');
        if (imgQuality) params.image_quality = parseInt(imgQuality.value, 10);

        return params;
    }

    function handleFileSelect(e) {
        if (e.target.files.length > 0) {
            setFile(e.target.files[0]);
        }
    }

    function setFile(file) {
        const toolFormats = state.formats[state.activeTool];
        if (toolFormats && toolFormats.input) {
            const ext = getExtension(file.name);
            if (!toolFormats.input.includes(ext)) {
                showInlineError(
                    `Unsupported format: .${ext}\nAccepted: ${toolFormats.input.map((f) => '.' + f).join(', ')}`
                );
                return;
            }
        }

        state.selectedFile = file;
        dom.fileName.textContent = file.name;
        dom.fileSize.textContent = formatBytes(file.size);
        dom.fileInfo.classList.remove('hidden');
        dom.dropzone.classList.add('hidden');
        dom.btnProcess.disabled = false;
        dom.btnText.textContent = toolConfig[state.activeTool].action;
    }

    function clearFile() {
        state.selectedFile = null;
        dom.fileInput.value = '';
        dom.fileInfo.classList.add('hidden');
        dom.dropzone.classList.remove('hidden');
        dom.btnProcess.disabled = true;
        dom.btnText.textContent = 'Select a file to start';
    }

    function startProcessing() {
        if (!state.selectedFile || state.uploading) return;

        state.uploading = true;
        dom.btnProcess.disabled = true;
        dom.optionsPanel.classList.add('hidden');
        dom.resultSection.classList.add('hidden');
        dom.progressSection.classList.remove('hidden');
        dom.progressLabel.textContent = 'Uploading...';
        dom.progressPct.textContent = '0%';
        dom.progressBar.style.width = '0%';
        dom.progressDetail.textContent = '';

        const formData = new FormData();
        formData.append('file', state.selectedFile);
        formData.append('operation', state.activeTool);

        const params = gatherParams();
        Object.keys(params).forEach((key) => {
            formData.append(key, params[key]);
        });

        const xhr = new XMLHttpRequest();

        xhr.upload.addEventListener('progress', (e) => {
            if (e.lengthComputable) {
                const pct = Math.round((e.loaded / e.total) * 100);
                dom.progressBar.style.width = pct + '%';
                dom.progressPct.textContent = pct + '%';
                dom.progressDetail.textContent = `${formatBytes(e.loaded)} / ${formatBytes(e.total)}`;
            }
        });

        xhr.upload.addEventListener('load', () => {
            dom.progressLabel.textContent = 'Processing...';
            dom.progressPct.textContent = '';
            dom.progressBar.style.width = '100%';
            dom.progressDetail.textContent = 'Your file is being processed. This may take a moment.';
        });

        xhr.addEventListener('load', () => {
            state.uploading = false;
            if (xhr.status >= 200 && xhr.status < 300) {
                try {
                    const data = JSON.parse(xhr.responseText);
                    state.jobId = data.id;
                    startPolling();
                } catch (e) {
                    showError('Invalid response from server.');
                }
            } else {
                try {
                    const err = JSON.parse(xhr.responseText);
                    showError(err.error || `Upload failed (HTTP ${xhr.status})`);
                } catch (e) {
                    showError(`Upload failed (HTTP ${xhr.status})`);
                }
            }
        });

        xhr.addEventListener('error', () => {
            state.uploading = false;
            showError('Network error. Please check your connection and try again.');
        });

        xhr.addEventListener('abort', () => {
            state.uploading = false;
            showError('Upload was cancelled.');
        });

        xhr.open('POST', '/api/jobs');
        xhr.send(formData);
    }

    function startPolling() {
        if (state.pollTimer) clearInterval(state.pollTimer);

        dom.progressLabel.textContent = 'Processing...';
        dom.progressBar.style.width = '100%';

        dom.progressBar.style.transition = 'none';
        dom.progressBar.style.width = '30%';
        requestAnimationFrame(() => {
            dom.progressBar.style.transition = 'width 30s ease-out';
            dom.progressBar.style.width = '90%';
        });

        state.pollTimer = setInterval(pollJob, 2000);
    }

    async function pollJob() {
        if (!state.jobId) return;

        try {
            const res = await fetch(`/api/jobs/${state.jobId}`);
            if (!res.ok) {
                const err = await res.json().catch(() => ({}));
                showError(err.error || `Status check failed (HTTP ${res.status})`);
                stopPolling();
                return;
            }

            const job = await res.json();

            switch (job.status) {
                case 'pending':
                    dom.progressLabel.textContent = 'Queued — waiting for worker...';
                    dom.progressDetail.textContent = 'Your job is in the queue.';
                    break;

                case 'processing':
                    dom.progressLabel.textContent = 'Processing...';
                    dom.progressDetail.textContent = 'Your file is being processed.';
                    break;

                case 'completed':
                    stopPolling();
                    showSuccess(job);
                    break;

                case 'failed':
                    stopPolling();
                    showError(job.error_message || 'Processing failed. Please try again.');
                    break;
            }
        } catch (e) {
        }
    }

    function stopPolling() {
        if (state.pollTimer) {
            clearInterval(state.pollTimer);
            state.pollTimer = null;
        }
    }

    function showSuccess(job) {
        dom.progressSection.classList.add('hidden');

        dom.resultInputSize.textContent = formatBytes(job.input_size);
        dom.resultOutputSize.textContent = formatBytes(job.output_size);

        if (job.input_size && job.output_size && job.input_size > 0) {
            const savings = ((1 - job.output_size / job.input_size) * 100).toFixed(1);
            if (savings > 0) {
                dom.resultSavings.textContent = `(${savings}% smaller)`;
                dom.resultSavings.style.color = 'var(--success)';
            } else if (savings < 0) {
                dom.resultSavings.textContent = `(${Math.abs(savings)}% larger)`;
                dom.resultSavings.style.color = 'var(--warning)';
            } else {
                dom.resultSavings.textContent = '(same size)';
                dom.resultSavings.style.color = 'var(--text-muted)';
            }
        } else {
            dom.resultSavings.textContent = '';
        }

        dom.resultDownload.href = `/api/jobs/${state.jobId}/download`;

        const origName = state.selectedFile ? state.selectedFile.name : 'output';
        const baseName = origName.substring(0, origName.lastIndexOf('.')) || origName;
        const outExt = job.output_filename
            ? job.output_filename.substring(job.output_filename.lastIndexOf('.'))
            : '';
        dom.resultDownload.setAttribute('download', baseName + '-iloveconvertion' + outExt);

        dom.resultSuccess.classList.remove('hidden');
        dom.resultError.classList.add('hidden');
        dom.resultSection.classList.remove('hidden');
    }

    function showError(message) {
        dom.progressSection.classList.add('hidden');
        dom.errorMessage.textContent = message;
        dom.resultSuccess.classList.add('hidden');
        dom.resultError.classList.remove('hidden');
        dom.resultSection.classList.remove('hidden');
    }

    function showInlineError(message) {
        alert(message);
    }

    function resetUI() {
        stopPolling();
        state.jobId = null;
        state.uploading = false;

        dom.progressSection.classList.add('hidden');
        dom.resultSection.classList.add('hidden');
        dom.progressBar.style.transition = 'width 200ms ease';
        dom.progressBar.style.width = '0%';

        buildOptionsPanel();

        if (state.selectedFile) {
            dom.btnProcess.disabled = false;
            dom.btnText.textContent = toolConfig[state.activeTool].action;
        } else {
            clearFile();
        }
    }

    function formatBytes(bytes) {
        if (bytes === 0 || bytes == null) return '0 B';
        const k = 1024;
        const sizes = ['B', 'KB', 'MB', 'GB'];
        const i = Math.floor(Math.log(bytes) / Math.log(k));
        return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
    }

    function getExtension(filename) {
        return (filename || '').split('.').pop().toLowerCase();
    }

    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', init);
    } else {
        init();
    }
})();