# Bergamot Translation Service Dockerfile
# Mozilla's fast, client-side translation engine

FROM python:3.11-slim

# Install system dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    cmake \
    curl \
    git \
    && rm -rf /var/lib/apt/lists/*

# Create non-root user
RUN useradd -m -u 1000 appuser

# Set working directory
WORKDIR /app

# Copy requirements
COPY requirements.txt .

# Install Python dependencies
RUN pip install --no-cache-dir -r requirements.txt

# Try to install bergamot-translator if available
# RUN pip install --no-cache-dir bergamot-translator || echo "Bergamot not available, using CTranslate2 fallback"

# Copy application code
COPY main.py .

# Create models directory
RUN mkdir -p /app/models && chown -R appuser:appuser /app

# Set environment variables
ENV PYTHONUNBUFFERED=1 \
    PYTHONDONTWRITEBYTECODE=1 \
    MODEL_CACHE_DIR=/app/models \
    HOST=0.0.0.0 \
    PORT=8080

# Switch to non-root user
USER appuser

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=30s --retries=3 \
    CMD curl -f http://localhost:8080/health || exit 1

# Run the application
CMD ["python", "main.py"]
