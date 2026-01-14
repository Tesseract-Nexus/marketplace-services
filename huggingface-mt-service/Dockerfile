# Hugging Face MT Service Dockerfile
# Supports both CPU and GPU inference

# Use NVIDIA CUDA base for GPU support, or Python base for CPU-only
ARG BASE_IMAGE=python:3.11-slim

FROM ${BASE_IMAGE}

# Install system dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    curl \
    && rm -rf /var/lib/apt/lists/*

# Create non-root user for security
RUN useradd -m -u 1000 appuser

# Set working directory
WORKDIR /app

# Copy requirements first for layer caching
COPY requirements.txt .

# Install Python dependencies
# For GPU: pip install torch with CUDA support
RUN pip install --no-cache-dir -r requirements.txt

# Pre-download commonly used models during build (optional, increases image size)
# This can be done at runtime instead via PRELOAD_MODELS env var
ARG PREDOWNLOAD_MODELS=""
RUN if [ -n "$PREDOWNLOAD_MODELS" ]; then \
    python -c "from transformers import MarianMTModel, MarianTokenizer; \
    models = '$PREDOWNLOAD_MODELS'.split(','); \
    [MarianMTModel.from_pretrained(m.strip()) or MarianTokenizer.from_pretrained(m.strip()) for m in models if m.strip()]"; \
    fi

# Copy application code
COPY main.py .

# Create cache directory for Hugging Face models
RUN mkdir -p /app/.cache && chown -R appuser:appuser /app

# Set environment variables
ENV PYTHONUNBUFFERED=1 \
    PYTHONDONTWRITEBYTECODE=1 \
    HF_HOME=/app/.cache \
    TRANSFORMERS_CACHE=/app/.cache \
    HOST=0.0.0.0 \
    PORT=8080

# Switch to non-root user
USER appuser

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=60s --retries=3 \
    CMD curl -f http://localhost:8080/health || exit 1

# Run the application
CMD ["python", "main.py"]
