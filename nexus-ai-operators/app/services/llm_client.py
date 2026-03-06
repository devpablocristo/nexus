from __future__ import annotations

from dataclasses import dataclass
import logging
from typing import Any

import httpx

from app.core.config import Settings

logger = logging.getLogger(__name__)

ANTHROPIC_MESSAGES_URL = 'https://api.anthropic.com/v1/messages'
ANTHROPIC_VERSION = '2023-06-01'
TIMEOUT_SECONDS = 30.0


@dataclass(frozen=True)
class LLMUsage:
    input_tokens: int = 0
    output_tokens: int = 0


@dataclass(frozen=True)
class LLMResult:
    text: str
    backend: str
    usage: LLMUsage
    fallback_used: bool
    fallback_reason: str | None = None


class LLMClient:
    """Unified LLM client — backend is selected entirely by env vars.

    Supported backends (via ``LLM_BACKEND``):
      - ``anthropic`` — Anthropic Messages API (requires ``ANTHROPIC_API_KEY``)
      - ``ollama``    — Local Ollama instance (requires ``OLLAMA_BASE_URL``)
      - ``fallback``  — Deterministic stub, no external calls (default)

    Same code everywhere; infra decides behaviour.
    """

    def __init__(self, settings: Settings) -> None:
        self._backend = settings.llm_backend.lower().strip()
        # Anthropic
        self._anthropic_api_key = settings.anthropic_api_key
        self._anthropic_model = settings.anthropic_model
        # Ollama
        self._ollama_base_url = settings.ollama_base_url.rstrip('/')
        self._ollama_model = settings.ollama_model

        logger.info('LLMClient initialised with backend=%s', self._backend)

    @property
    def backend(self) -> str:
        return self._backend

    @property
    def is_configured(self) -> bool:
        if self._backend == 'anthropic':
            return bool(self._anthropic_api_key)
        if self._backend == 'ollama':
            return bool(self._ollama_base_url)
        return True  # fallback is always "configured"

    async def query(
        self,
        system_prompt: str,
        user_message: str,
        prompt_id: str,
        prompt_version: str,
        max_output_tokens: int,
    ) -> LLMResult:
        """Route to the configured backend; fall back on any error."""
        if self._backend == 'anthropic':
            return await self._query_anthropic(
                system_prompt,
                user_message,
                prompt_id=prompt_id,
                prompt_version=prompt_version,
                max_output_tokens=max_output_tokens,
            )
        if self._backend == 'ollama':
            return await self._query_ollama(
                system_prompt,
                user_message,
                prompt_id=prompt_id,
                prompt_version=prompt_version,
                max_output_tokens=max_output_tokens,
            )
        return self._fallback(reason='backend_selected_fallback')

    # ------------------------------------------------------------------
    # Anthropic backend
    # ------------------------------------------------------------------
    async def _query_anthropic(
        self,
        system_prompt: str,
        user_message: str,
        *,
        prompt_id: str,
        prompt_version: str,
        max_output_tokens: int,
    ) -> LLMResult:
        if not self._anthropic_api_key:
            logger.warning('Anthropic backend selected but ANTHROPIC_API_KEY is empty')
            return self._fallback(reason='anthropic_unconfigured')

        headers: dict[str, str] = {
            'x-api-key': self._anthropic_api_key,
            'anthropic-version': ANTHROPIC_VERSION,
            'content-type': 'application/json',
        }
        payload: dict[str, Any] = {
            'model': self._anthropic_model,
            'max_tokens': max_output_tokens,
            'system': system_prompt,
            'metadata': {
                'prompt_id': prompt_id,
                'prompt_version': prompt_version,
            },
            'messages': [{'role': 'user', 'content': user_message}],
        }

        try:
            async with httpx.AsyncClient(timeout=TIMEOUT_SECONDS) as client:
                response = await client.post(ANTHROPIC_MESSAGES_URL, headers=headers, json=payload)
                response.raise_for_status()
                data = response.json()
                blocks: list[dict[str, Any]] = data.get('content', [])
                texts = [b['text'] for b in blocks if b.get('type') == 'text']
                text = ' '.join(texts).strip()
                if not text:
                    return self._fallback(reason='anthropic_invalid_payload')
                usage = data.get('usage', {})
                return LLMResult(
                    text=text,
                    backend='anthropic',
                    usage=LLMUsage(
                        input_tokens=int(usage.get('input_tokens') or 0),
                        output_tokens=int(usage.get('output_tokens') or 0),
                    ),
                    fallback_used=False,
                )
        except httpx.HTTPStatusError as exc:
            logger.error('Anthropic API %s: %s', exc.response.status_code, exc.response.text[:200])
            return self._fallback(reason=f'anthropic_http_{exc.response.status_code}')
        except (httpx.RequestError, Exception) as exc:  # noqa: BLE001
            logger.error('Anthropic request failed: %s', exc)
            return self._fallback(reason='anthropic_request_failed')

    # ------------------------------------------------------------------
    # Ollama backend (/api/chat)
    # ------------------------------------------------------------------
    async def _query_ollama(
        self,
        system_prompt: str,
        user_message: str,
        *,
        prompt_id: str,
        prompt_version: str,
        max_output_tokens: int,
    ) -> LLMResult:
        url = f'{self._ollama_base_url}/api/chat'
        payload: dict[str, Any] = {
            'model': self._ollama_model,
            'stream': False,
            'options': {'num_predict': max_output_tokens},
            'metadata': {'prompt_id': prompt_id, 'prompt_version': prompt_version},
            'messages': [
                {'role': 'system', 'content': system_prompt},
                {'role': 'user', 'content': user_message},
            ],
        }

        try:
            async with httpx.AsyncClient(timeout=TIMEOUT_SECONDS) as client:
                response = await client.post(url, json=payload)
                response.raise_for_status()
                data = response.json()
                content: str = data.get('message', {}).get('content', '').strip()
                if not content:
                    return self._fallback(reason='ollama_invalid_payload')
                return LLMResult(
                    text=content,
                    backend='ollama',
                    usage=LLMUsage(
                        input_tokens=int(data.get('prompt_eval_count') or 0),
                        output_tokens=int(data.get('eval_count') or 0),
                    ),
                    fallback_used=False,
                )
        except httpx.HTTPStatusError as exc:
            logger.error('Ollama API %s: %s', exc.response.status_code, exc.response.text[:200])
            return self._fallback(reason=f'ollama_http_{exc.response.status_code}')
        except (httpx.RequestError, Exception) as exc:  # noqa: BLE001
            logger.error('Ollama request failed: %s', exc)
            return self._fallback(reason='ollama_request_failed')

    # ------------------------------------------------------------------
    # Deterministic fallback — always works, no external deps
    # ------------------------------------------------------------------
    def _fallback(self, reason: str) -> LLMResult:
        return LLMResult(
            text='',
            backend=self._backend,
            usage=LLMUsage(),
            fallback_used=True,
            fallback_reason=reason,
        )
