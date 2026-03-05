from pydantic import AliasChoices, Field
from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    model_config = SettingsConfigDict(env_file='.env', env_file_encoding='utf-8', extra='ignore')

    app_name: str = Field(default='nexus-ai-operators', alias='OPERATOR_APP_NAME')
    app_env: str = Field(default='dev', alias='OPERATOR_ENV')
    app_port: int = Field(default=8000, alias='OPERATOR_PORT')

    core_base_url: str = Field(
        default='http://nexus-core:8080',
        validation_alias=AliasChoices('NEXUS_CORE_BASE_URL', 'NEXUS_SAAS_BASE_URL'),
    )
    core_api_key: str = Field(
        default='operator-internal-key',
        validation_alias=AliasChoices('NEXUS_CORE_API_KEY', 'NEXUS_SAAS_API_KEY'),
    )
    core_timeout_seconds: float = Field(
        default=5.0,
        validation_alias=AliasChoices('NEXUS_CORE_TIMEOUT_SECONDS', 'NEXUS_SAAS_TIMEOUT_SECONDS'),
    )

    poll_interval_seconds: int = Field(default=10, alias='OPERATOR_POLL_INTERVAL_SECONDS')
    poll_batch_size: int = Field(default=100, alias='OPERATOR_POLL_BATCH_SIZE')
    deny_ratio_threshold: float = Field(default=0.35, alias='OPERATOR_DENY_RATIO_THRESHOLD')
    min_events_for_signal: int = Field(default=20, alias='OPERATOR_MIN_EVENTS_FOR_SIGNAL')
    action_cooldown_seconds: int = Field(default=300, alias='OPERATOR_ACTION_COOLDOWN_SECONDS')
    action_ttl_seconds: int = Field(default=300, alias='OPERATOR_ACTION_TTL_SECONDS')

    operator_internal_key: str = Field(default='operator-internal-key', alias='OPERATOR_INTERNAL_KEY')
    cors_allowed_origins: str = Field(
        default='http://localhost:5173,http://localhost:5174',
        alias='NEXUS_CORS_ALLOWED_ORIGINS',
    )
    max_body_bytes: int = Field(default=1048576, alias='OPERATOR_MAX_BODY_BYTES')
    assistant_rate_limit_per_min: int = Field(default=30, alias='OPERATOR_ASSISTANT_RATE_LIMIT_PER_MIN')
    dlq_path: str = Field(default='data/dead_letters.jsonl', alias='NEXUS_DLQ_PATH')

    # LLM configuration — infra decides, code stays the same
    llm_backend: str = Field(default='fallback', alias='LLM_BACKEND')  # ollama | anthropic | fallback
    anthropic_api_key: str = Field(default='', alias='ANTHROPIC_API_KEY')
    anthropic_model: str = Field(default='claude-sonnet-4-20250514', alias='ANTHROPIC_MODEL')
    ollama_base_url: str = Field(default='http://ollama:11434', alias='OLLAMA_BASE_URL')
    ollama_model: str = Field(default='llama3.1:8b', alias='OLLAMA_MODEL')


settings = Settings()
