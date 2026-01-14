"""
Self-hosted Hugging Face Machine Translation Service
Uses Helsinki-NLP OPUS-MT models for translation

This service provides a REST API for translation using locally loaded models.
Models are downloaded at startup and cached for subsequent requests.
"""

import os
import logging
import time
from typing import Dict, List, Optional
from contextlib import asynccontextmanager
from functools import lru_cache

from fastapi import FastAPI, HTTPException, Header
from fastapi.middleware.cors import CORSMiddleware
from pydantic import BaseModel, Field
from transformers import MarianMTModel, MarianTokenizer
import torch

# Configure logging
logging.basicConfig(
    level=os.getenv("LOG_LEVEL", "INFO").upper(),
    format='{"time":"%(asctime)s","level":"%(levelname)s","message":"%(message)s","service":"huggingface-mt-service"}'
)
logger = logging.getLogger(__name__)

# Model configuration
# Helsinki-NLP OPUS-MT models for various language pairs
MODEL_MAPPING = {
    # English to other languages
    "en-hi": "Helsinki-NLP/opus-mt-en-hi",
    "en-es": "Helsinki-NLP/opus-mt-en-es",
    "en-fr": "Helsinki-NLP/opus-mt-en-fr",
    "en-de": "Helsinki-NLP/opus-mt-en-de",
    "en-it": "Helsinki-NLP/opus-mt-en-it",
    "en-pt": "Helsinki-NLP/opus-mt-en-pt",
    "en-nl": "Helsinki-NLP/opus-mt-en-nl",
    "en-ru": "Helsinki-NLP/opus-mt-en-ru",
    "en-zh": "Helsinki-NLP/opus-mt-en-zh",
    "en-ja": "Helsinki-NLP/opus-mt-en-jap",
    "en-ko": "Helsinki-NLP/opus-mt-en-ko",
    "en-ar": "Helsinki-NLP/opus-mt-en-ar",
    "en-tr": "Helsinki-NLP/opus-mt-en-tr",
    "en-vi": "Helsinki-NLP/opus-mt-en-vi",
    "en-th": "Helsinki-NLP/opus-mt-en-th",
    "en-id": "Helsinki-NLP/opus-mt-en-id",
    "en-pl": "Helsinki-NLP/opus-mt-en-pl",
    "en-he": "Helsinki-NLP/opus-mt-en-he",
    # Multilingual model for Indian languages (Tamil, Telugu, Bengali, etc.)
    "en-mul": "Helsinki-NLP/opus-mt-en-mul",

    # Other languages to English
    "hi-en": "Helsinki-NLP/opus-mt-hi-en",
    "es-en": "Helsinki-NLP/opus-mt-es-en",
    "fr-en": "Helsinki-NLP/opus-mt-fr-en",
    "de-en": "Helsinki-NLP/opus-mt-de-en",
    "it-en": "Helsinki-NLP/opus-mt-it-en",
    "pt-en": "Helsinki-NLP/opus-mt-pt-en",
    "nl-en": "Helsinki-NLP/opus-mt-nl-en",
    "ru-en": "Helsinki-NLP/opus-mt-ru-en",
    "zh-en": "Helsinki-NLP/opus-mt-zh-en",
    "ja-en": "Helsinki-NLP/opus-mt-jap-en",
    "ko-en": "Helsinki-NLP/opus-mt-ko-en",
    "ar-en": "Helsinki-NLP/opus-mt-ar-en",
    "tr-en": "Helsinki-NLP/opus-mt-tr-en",

    # Cross-language pairs
    "es-fr": "Helsinki-NLP/opus-mt-es-fr",
    "fr-es": "Helsinki-NLP/opus-mt-fr-es",
    "es-it": "Helsinki-NLP/opus-mt-es-it",
    "it-es": "Helsinki-NLP/opus-mt-it-es",
    "es-pt": "Helsinki-NLP/opus-mt-es-pt",
    "pt-es": "Helsinki-NLP/opus-mt-pt-es",
}

# Indian languages that use the multilingual model
MULTILINGUAL_TARGETS = {"ta", "te", "bn", "mr", "gu", "kn", "ml", "pa", "or"}

# Global model cache
loaded_models: Dict[str, tuple] = {}
device = "cuda" if torch.cuda.is_available() else "cpu"


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
    model: str
    latency_ms: float


class BatchTranslationRequest(BaseModel):
    """Batch translation request model"""
    texts: List[str] = Field(..., min_items=1, max_items=50, description="List of texts to translate")
    source_lang: str = Field(..., min_length=2, max_length=5)
    target_lang: str = Field(..., min_length=2, max_length=5)


class BatchTranslationResponse(BaseModel):
    """Batch translation response model"""
    translations: List[str]
    source_lang: str
    target_lang: str
    model: str
    count: int
    latency_ms: float


class LanguagePair(BaseModel):
    """Language pair model"""
    source: str
    target: str
    model: str


class HealthResponse(BaseModel):
    """Health check response"""
    status: str
    device: str
    loaded_models: int
    available_pairs: int


def get_model_name(source_lang: str, target_lang: str) -> Optional[str]:
    """Get the model name for a language pair"""
    # Check for Indian language targets that use multilingual model
    if source_lang == "en" and target_lang in MULTILINGUAL_TARGETS:
        return MODEL_MAPPING.get("en-mul")

    pair_key = f"{source_lang}-{target_lang}"
    return MODEL_MAPPING.get(pair_key)


def load_model(model_name: str) -> tuple:
    """Load a model and tokenizer, with caching"""
    if model_name in loaded_models:
        return loaded_models[model_name]

    logger.info(f"Loading model: {model_name}")
    start = time.time()

    tokenizer = MarianTokenizer.from_pretrained(model_name)
    model = MarianMTModel.from_pretrained(model_name)
    model = model.to(device)
    model.eval()  # Set to evaluation mode

    loaded_models[model_name] = (model, tokenizer)
    logger.info(f"Model {model_name} loaded in {time.time() - start:.2f}s on {device}")

    return model, tokenizer


def translate_text(text: str, model, tokenizer) -> str:
    """Translate text using the given model and tokenizer"""
    # Tokenize input
    inputs = tokenizer(text, return_tensors="pt", padding=True, truncation=True, max_length=512)
    inputs = {k: v.to(device) for k, v in inputs.items()}

    # Generate translation
    with torch.no_grad():
        translated_ids = model.generate(**inputs, max_length=512, num_beams=4, early_stopping=True)

    # Decode output
    translated_text = tokenizer.decode(translated_ids[0], skip_special_tokens=True)
    return translated_text


def translate_batch(texts: List[str], model, tokenizer) -> List[str]:
    """Translate multiple texts in batch"""
    # Tokenize all inputs
    inputs = tokenizer(texts, return_tensors="pt", padding=True, truncation=True, max_length=512)
    inputs = {k: v.to(device) for k, v in inputs.items()}

    # Generate translations
    with torch.no_grad():
        translated_ids = model.generate(**inputs, max_length=512, num_beams=4, early_stopping=True)

    # Decode all outputs
    translations = [tokenizer.decode(ids, skip_special_tokens=True) for ids in translated_ids]
    return translations


# Preload commonly used models at startup
PRELOAD_MODELS = os.getenv("PRELOAD_MODELS", "en-hi,en-es,en-fr,hi-en").split(",")


@asynccontextmanager
async def lifespan(app: FastAPI):
    """Startup and shutdown lifecycle"""
    # Startup: preload models
    logger.info(f"Starting Hugging Face MT Service on {device}")
    logger.info(f"CUDA available: {torch.cuda.is_available()}")
    if torch.cuda.is_available():
        logger.info(f"CUDA device: {torch.cuda.get_device_name(0)}")

    # Preload specified models
    for pair in PRELOAD_MODELS:
        pair = pair.strip()
        if pair:
            parts = pair.split("-")
            if len(parts) == 2:
                model_name = get_model_name(parts[0], parts[1])
                if model_name:
                    try:
                        load_model(model_name)
                    except Exception as e:
                        logger.error(f"Failed to preload model {model_name}: {e}")

    logger.info(f"Preloaded {len(loaded_models)} models")
    yield
    # Shutdown
    logger.info("Shutting down Hugging Face MT Service")


# Create FastAPI app
app = FastAPI(
    title="Hugging Face MT Service",
    description="Self-hosted machine translation using Helsinki-NLP OPUS-MT models",
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
    return HealthResponse(
        status="healthy",
        device=device,
        loaded_models=len(loaded_models),
        available_pairs=len(MODEL_MAPPING),
    )


@app.get("/livez")
async def liveness():
    """Liveness probe endpoint"""
    return {"status": "alive"}


@app.get("/readyz")
async def readiness():
    """Readiness probe endpoint"""
    # Ready if at least one model is loaded or we can load one
    if len(loaded_models) > 0:
        return {"status": "ready"}

    # Try to load a simple model to verify readiness
    try:
        model_name = MODEL_MAPPING.get("en-hi")
        if model_name:
            load_model(model_name)
            return {"status": "ready"}
    except Exception as e:
        logger.error(f"Readiness check failed: {e}")

    raise HTTPException(status_code=503, detail="Service not ready")


@app.get("/languages", response_model=List[LanguagePair])
async def get_languages():
    """Get list of supported language pairs"""
    pairs = []
    for pair_key, model_name in MODEL_MAPPING.items():
        parts = pair_key.split("-")
        if len(parts) == 2:
            pairs.append(LanguagePair(source=parts[0], target=parts[1], model=model_name))

    # Add multilingual targets
    for target in MULTILINGUAL_TARGETS:
        pairs.append(LanguagePair(source="en", target=target, model=MODEL_MAPPING["en-mul"]))

    return pairs


@app.post("/translate", response_model=TranslationResponse)
async def translate(
    request: TranslationRequest,
    x_tenant_id: Optional[str] = Header(None, alias="X-Tenant-ID"),
):
    """Translate text from source to target language"""
    start_time = time.time()

    # Get model for this language pair
    model_name = get_model_name(request.source_lang, request.target_lang)
    if not model_name:
        raise HTTPException(
            status_code=400,
            detail=f"Unsupported language pair: {request.source_lang} -> {request.target_lang}"
        )

    try:
        # Load model (cached if already loaded)
        model, tokenizer = load_model(model_name)

        # Translate
        translated = translate_text(request.text, model, tokenizer)

        latency_ms = (time.time() - start_time) * 1000

        logger.info(f"Translated {request.source_lang}->{request.target_lang}: {len(request.text)} chars in {latency_ms:.2f}ms")

        return TranslationResponse(
            translated_text=translated,
            source_lang=request.source_lang,
            target_lang=request.target_lang,
            model=model_name,
            latency_ms=latency_ms,
        )

    except Exception as e:
        logger.error(f"Translation failed: {e}")
        raise HTTPException(status_code=500, detail=f"Translation failed: {str(e)}")


@app.post("/translate/batch", response_model=BatchTranslationResponse)
async def translate_batch_endpoint(
    request: BatchTranslationRequest,
    x_tenant_id: Optional[str] = Header(None, alias="X-Tenant-ID"),
):
    """Translate multiple texts in batch"""
    start_time = time.time()

    # Get model for this language pair
    model_name = get_model_name(request.source_lang, request.target_lang)
    if not model_name:
        raise HTTPException(
            status_code=400,
            detail=f"Unsupported language pair: {request.source_lang} -> {request.target_lang}"
        )

    try:
        # Load model (cached if already loaded)
        model, tokenizer = load_model(model_name)

        # Translate batch
        translations = translate_batch(request.texts, model, tokenizer)

        latency_ms = (time.time() - start_time) * 1000

        logger.info(f"Batch translated {len(request.texts)} texts {request.source_lang}->{request.target_lang} in {latency_ms:.2f}ms")

        return BatchTranslationResponse(
            translations=translations,
            source_lang=request.source_lang,
            target_lang=request.target_lang,
            model=model_name,
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
        "loaded_models": list(loaded_models.keys()),
        "count": len(loaded_models),
        "device": device,
    }


if __name__ == "__main__":
    import uvicorn

    host = os.getenv("HOST", "0.0.0.0")
    port = int(os.getenv("PORT", "8080"))

    uvicorn.run(app, host=host, port=port)
