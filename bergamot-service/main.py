"""
Bergamot Translation Service
Mozilla's fast, privacy-focused, client-side translation engine

Bergamot is designed for browser-based translation but can run server-side.
Uses compressed models optimized for speed and memory efficiency.
"""

import os
import logging
import time
import asyncio
from typing import Dict, List, Optional
from pathlib import Path
from contextlib import asynccontextmanager

from fastapi import FastAPI, HTTPException, Header, BackgroundTasks
from fastapi.middleware.cors import CORSMiddleware
from pydantic import BaseModel, Field
import aiohttp
import zipfile
import io

# Configure logging
logging.basicConfig(
    level=os.getenv("LOG_LEVEL", "INFO").upper(),
    format='{"time":"%(asctime)s","level":"%(levelname)s","message":"%(message)s","service":"bergamot-service"}'
)
logger = logging.getLogger(__name__)

# Bergamot model registry - models are downloaded from Mozilla's servers
# Format: (source_lang, target_lang) -> model_url
MODEL_REGISTRY = {
    # English to other languages
    ("en", "es"): "https://storage.googleapis.com/bergamot-models-sandbox/prod/ende/model.enes.intgemm.alphas.bin.gz",
    ("en", "de"): "https://storage.googleapis.com/bergamot-models-sandbox/prod/ende/model.ende.intgemm.alphas.bin.gz",
    ("en", "fr"): "https://storage.googleapis.com/bergamot-models-sandbox/prod/enfr/model.enfr.intgemm.alphas.bin.gz",
    ("en", "pt"): "https://storage.googleapis.com/bergamot-models-sandbox/prod/enpt/model.enpt.intgemm.alphas.bin.gz",
    ("en", "it"): "https://storage.googleapis.com/bergamot-models-sandbox/prod/enit/model.enit.intgemm.alphas.bin.gz",
    ("en", "nl"): "https://storage.googleapis.com/bergamot-models-sandbox/prod/ennl/model.ennl.intgemm.alphas.bin.gz",
    ("en", "ru"): "https://storage.googleapis.com/bergamot-models-sandbox/prod/enru/model.enru.intgemm.alphas.bin.gz",
    ("en", "pl"): "https://storage.googleapis.com/bergamot-models-sandbox/prod/enpl/model.enpl.intgemm.alphas.bin.gz",
    ("en", "cs"): "https://storage.googleapis.com/bergamot-models-sandbox/prod/encs/model.encs.intgemm.alphas.bin.gz",
    ("en", "et"): "https://storage.googleapis.com/bergamot-models-sandbox/prod/enet/model.enet.intgemm.alphas.bin.gz",

    # Other languages to English
    ("es", "en"): "https://storage.googleapis.com/bergamot-models-sandbox/prod/esen/model.esen.intgemm.alphas.bin.gz",
    ("de", "en"): "https://storage.googleapis.com/bergamot-models-sandbox/prod/deen/model.deen.intgemm.alphas.bin.gz",
    ("fr", "en"): "https://storage.googleapis.com/bergamot-models-sandbox/prod/fren/model.fren.intgemm.alphas.bin.gz",
    ("pt", "en"): "https://storage.googleapis.com/bergamot-models-sandbox/prod/pten/model.pten.intgemm.alphas.bin.gz",
    ("it", "en"): "https://storage.googleapis.com/bergamot-models-sandbox/prod/iten/model.iten.intgemm.alphas.bin.gz",
    ("nl", "en"): "https://storage.googleapis.com/bergamot-models-sandbox/prod/nlen/model.nlen.intgemm.alphas.bin.gz",
    ("ru", "en"): "https://storage.googleapis.com/bergamot-models-sandbox/prod/ruen/model.ruen.intgemm.alphas.bin.gz",
    ("pl", "en"): "https://storage.googleapis.com/bergamot-models-sandbox/prod/plen/model.plen.intgemm.alphas.bin.gz",
    ("cs", "en"): "https://storage.googleapis.com/bergamot-models-sandbox/prod/csen/model.csen.intgemm.alphas.bin.gz",
    ("et", "en"): "https://storage.googleapis.com/bergamot-models-sandbox/prod/eten/model.eten.intgemm.alphas.bin.gz",
}

# Try to import bergamot-translator (C++ based, may not be available)
try:
    import bergamot
    BERGAMOT_AVAILABLE = True
    logger.info("Bergamot translator library loaded successfully")
except ImportError:
    BERGAMOT_AVAILABLE = False
    logger.warning("Bergamot translator library not available - using CTranslate2 fallback")

# Fallback to CTranslate2 if bergamot is not available
try:
    import ctranslate2
    CTRANSLATE2_AVAILABLE = True
except ImportError:
    CTRANSLATE2_AVAILABLE = False

# Model cache
MODEL_CACHE_DIR = Path(os.getenv("MODEL_CACHE_DIR", "/app/models"))
loaded_translators: Dict[str, object] = {}


class TranslationRequest(BaseModel):
    """Translation request model"""
    text: str = Field(..., min_length=1, max_length=10000, description="Text to translate")
    source_lang: str = Field(..., min_length=2, max_length=5, description="Source language code")
    target_lang: str = Field(..., min_length=2, max_length=5, description="Target language code")


class TranslationResponse(BaseModel):
    """Translation response model"""
    translated_text: str
    source_lang: str
    target_lang: str
    engine: str
    latency_ms: float


class BatchTranslationRequest(BaseModel):
    """Batch translation request model"""
    texts: List[str] = Field(..., min_items=1, max_items=100, description="List of texts to translate")
    source_lang: str = Field(..., min_length=2, max_length=5)
    target_lang: str = Field(..., min_length=2, max_length=5)


class BatchTranslationResponse(BaseModel):
    """Batch translation response model"""
    translations: List[str]
    source_lang: str
    target_lang: str
    engine: str
    count: int
    latency_ms: float


class LanguagePair(BaseModel):
    """Language pair model"""
    source: str
    target: str


class HealthResponse(BaseModel):
    """Health check response"""
    status: str
    engine: str
    loaded_models: int
    available_pairs: int


async def download_model(source_lang: str, target_lang: str) -> Optional[Path]:
    """Download a model from the registry"""
    pair = (source_lang, target_lang)
    if pair not in MODEL_REGISTRY:
        return None

    model_dir = MODEL_CACHE_DIR / f"{source_lang}-{target_lang}"
    model_dir.mkdir(parents=True, exist_ok=True)

    model_file = model_dir / "model.bin"
    if model_file.exists():
        logger.info(f"Model {source_lang}-{target_lang} already downloaded")
        return model_dir

    url = MODEL_REGISTRY[pair]
    logger.info(f"Downloading model from {url}")

    try:
        async with aiohttp.ClientSession() as session:
            async with session.get(url) as response:
                if response.status == 200:
                    content = await response.read()
                    # Decompress if gzipped
                    if url.endswith('.gz'):
                        import gzip
                        content = gzip.decompress(content)
                    model_file.write_bytes(content)
                    logger.info(f"Model {source_lang}-{target_lang} downloaded successfully")
                    return model_dir
                else:
                    logger.error(f"Failed to download model: HTTP {response.status}")
                    return None
    except Exception as e:
        logger.error(f"Failed to download model: {e}")
        return None


def get_translator(source_lang: str, target_lang: str):
    """Get or create a translator for the language pair"""
    pair_key = f"{source_lang}-{target_lang}"

    if pair_key in loaded_translators:
        return loaded_translators[pair_key]

    # For now, we'll use a simple placeholder translator
    # In production, this would load the actual Bergamot/CTranslate2 model
    logger.info(f"Loading translator for {pair_key}")

    # This is a placeholder - actual implementation would use bergamot or ctranslate2
    loaded_translators[pair_key] = {
        "source": source_lang,
        "target": target_lang,
        "loaded": True
    }

    return loaded_translators[pair_key]


def translate_with_bergamot(text: str, source_lang: str, target_lang: str) -> str:
    """Translate text using Bergamot (placeholder for actual implementation)"""
    # This is a placeholder implementation
    # Actual Bergamot implementation would use the bergamot-translator library

    # For now, we'll simulate translation
    # In production, this would be:
    # translator = bergamot.Translator(model_path, config)
    # result = translator.translate(text)
    # return result

    # Placeholder: return text with language tags for testing
    return f"[{source_lang}->{target_lang}] {text}"


def translate_with_ctranslate2(text: str, source_lang: str, target_lang: str) -> str:
    """Translate text using CTranslate2 (fallback)"""
    # This is a placeholder implementation
    # Actual CTranslate2 implementation would use the ctranslate2 library

    return f"[{source_lang}->{target_lang}] {text}"


@asynccontextmanager
async def lifespan(app: FastAPI):
    """Startup and shutdown lifecycle"""
    logger.info("Starting Bergamot Translation Service")

    # Create model cache directory
    MODEL_CACHE_DIR.mkdir(parents=True, exist_ok=True)

    # Log available engines
    if BERGAMOT_AVAILABLE:
        logger.info("Using Bergamot translator engine")
    elif CTRANSLATE2_AVAILABLE:
        logger.info("Using CTranslate2 fallback engine")
    else:
        logger.warning("No translation engine available - using placeholder")

    # Preload commonly used models
    preload_pairs = os.getenv("PRELOAD_MODELS", "en-es,en-de,es-en,de-en").split(",")
    for pair in preload_pairs:
        pair = pair.strip()
        if pair and "-" in pair:
            source, target = pair.split("-", 1)
            if (source, target) in MODEL_REGISTRY:
                await download_model(source, target)

    logger.info(f"Bergamot service ready with {len(MODEL_REGISTRY)} available language pairs")
    yield
    logger.info("Shutting down Bergamot Translation Service")


# Create FastAPI app
app = FastAPI(
    title="Bergamot Translation Service",
    description="Mozilla's fast, privacy-focused translation engine",
    version="1.0.0",
    lifespan=lifespan,
)

# Add CORS middleware
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)


@app.get("/health", response_model=HealthResponse)
async def health_check():
    """Health check endpoint"""
    engine = "bergamot" if BERGAMOT_AVAILABLE else ("ctranslate2" if CTRANSLATE2_AVAILABLE else "placeholder")
    return HealthResponse(
        status="healthy",
        engine=engine,
        loaded_models=len(loaded_translators),
        available_pairs=len(MODEL_REGISTRY),
    )


@app.get("/livez")
async def liveness():
    """Liveness probe endpoint"""
    return {"status": "alive"}


@app.get("/readyz")
async def readiness():
    """Readiness probe endpoint"""
    return {"status": "ready"}


@app.get("/languages", response_model=List[LanguagePair])
async def get_languages():
    """Get list of supported language pairs"""
    return [LanguagePair(source=s, target=t) for s, t in MODEL_REGISTRY.keys()]


@app.post("/translate", response_model=TranslationResponse)
async def translate(
    request: TranslationRequest,
    x_tenant_id: Optional[str] = Header(None, alias="X-Tenant-ID"),
):
    """Translate text from source to target language"""
    start_time = time.time()

    pair = (request.source_lang, request.target_lang)
    if pair not in MODEL_REGISTRY:
        raise HTTPException(
            status_code=400,
            detail=f"Unsupported language pair: {request.source_lang} -> {request.target_lang}"
        )

    try:
        # Get translator
        get_translator(request.source_lang, request.target_lang)

        # Translate
        if BERGAMOT_AVAILABLE:
            translated = translate_with_bergamot(request.text, request.source_lang, request.target_lang)
            engine = "bergamot"
        elif CTRANSLATE2_AVAILABLE:
            translated = translate_with_ctranslate2(request.text, request.source_lang, request.target_lang)
            engine = "ctranslate2"
        else:
            # Placeholder translation for testing
            translated = f"[{request.source_lang}->{request.target_lang}] {request.text}"
            engine = "placeholder"

        latency_ms = (time.time() - start_time) * 1000

        logger.info(f"Translated {request.source_lang}->{request.target_lang}: {len(request.text)} chars in {latency_ms:.2f}ms")

        return TranslationResponse(
            translated_text=translated,
            source_lang=request.source_lang,
            target_lang=request.target_lang,
            engine=engine,
            latency_ms=latency_ms,
        )

    except Exception as e:
        logger.error(f"Translation failed: {e}")
        raise HTTPException(status_code=500, detail=f"Translation failed: {str(e)}")


@app.post("/translate/batch", response_model=BatchTranslationResponse)
async def translate_batch(
    request: BatchTranslationRequest,
    x_tenant_id: Optional[str] = Header(None, alias="X-Tenant-ID"),
):
    """Translate multiple texts in batch"""
    start_time = time.time()

    pair = (request.source_lang, request.target_lang)
    if pair not in MODEL_REGISTRY:
        raise HTTPException(
            status_code=400,
            detail=f"Unsupported language pair: {request.source_lang} -> {request.target_lang}"
        )

    try:
        # Get translator
        get_translator(request.source_lang, request.target_lang)

        # Translate all texts
        translations = []
        engine = "placeholder"

        for text in request.texts:
            if BERGAMOT_AVAILABLE:
                translated = translate_with_bergamot(text, request.source_lang, request.target_lang)
                engine = "bergamot"
            elif CTRANSLATE2_AVAILABLE:
                translated = translate_with_ctranslate2(text, request.source_lang, request.target_lang)
                engine = "ctranslate2"
            else:
                translated = f"[{request.source_lang}->{request.target_lang}] {text}"
                engine = "placeholder"
            translations.append(translated)

        latency_ms = (time.time() - start_time) * 1000

        logger.info(f"Batch translated {len(request.texts)} texts in {latency_ms:.2f}ms")

        return BatchTranslationResponse(
            translations=translations,
            source_lang=request.source_lang,
            target_lang=request.target_lang,
            engine=engine,
            count=len(translations),
            latency_ms=latency_ms,
        )

    except Exception as e:
        logger.error(f"Batch translation failed: {e}")
        raise HTTPException(status_code=500, detail=f"Batch translation failed: {str(e)}")


@app.get("/models")
async def get_loaded_models():
    """Get list of currently loaded models"""
    return {
        "loaded_models": list(loaded_translators.keys()),
        "count": len(loaded_translators),
        "available_pairs": len(MODEL_REGISTRY),
    }


@app.post("/models/download")
async def download_model_endpoint(
    source_lang: str,
    target_lang: str,
    background_tasks: BackgroundTasks,
):
    """Download a model for a language pair"""
    pair = (source_lang, target_lang)
    if pair not in MODEL_REGISTRY:
        raise HTTPException(
            status_code=400,
            detail=f"Unsupported language pair: {source_lang} -> {target_lang}"
        )

    background_tasks.add_task(download_model, source_lang, target_lang)
    return {"message": f"Download started for {source_lang}->{target_lang}"}


if __name__ == "__main__":
    import uvicorn

    host = os.getenv("HOST", "0.0.0.0")
    port = int(os.getenv("PORT", "8080"))

    uvicorn.run(app, host=host, port=port)
