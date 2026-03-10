FROM python:3.14-slim

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1

RUN apt-get update \
    && apt-get install -y --no-install-recommends wget \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /workspace

COPY ai-runtime /workspace/ai-runtime
WORKDIR /workspace/ai-runtime

RUN pip install --no-cache-dir --upgrade pip && pip install --no-cache-dir .

RUN useradd -m -u 1000 app \
    && chown -R app:app /workspace
USER app

EXPOSE 8000
HEALTHCHECK --interval=10s --timeout=3s --retries=3 \
  CMD wget -qO- http://localhost:8000/readyz >/dev/null 2>&1 || exit 1
CMD ["uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "8000"]
