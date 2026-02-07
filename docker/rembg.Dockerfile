FROM python:3.11-slim-bookworm

RUN apt-get update && \
    apt-get install -y --no-install-recommends curl && \
    rm -rf /var/lib/apt/lists/*

RUN groupadd -r appgroup && \
    useradd -r -g appgroup -m -d /home/appuser appuser

WORKDIR /app

COPY rembg-service/requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

ENV U2NET_HOME=/app/models
ENV NUMBA_CACHE_DIR=/app/numba_cache
ENV MPLCONFIGDIR=/app/mpl_config

RUN mkdir -p /app/models /app/numba_cache /app/mpl_config

RUN python -c "\
from rembg import new_session; \
new_session('silueta'); \
print('Model + numba ready')"

COPY rembg-service/app.py .

RUN chown -R appuser:appgroup /app /home/appuser

USER appuser

EXPOSE 5000

CMD ["gunicorn", "--bind", "0.0.0.0:5000", "--workers", "1", "--timeout", "120", "app:app"]