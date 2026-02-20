from pydantic import Field
from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    model_config = SettingsConfigDict(env_file='.env', env_file_encoding='utf-8', extra='ignore')

    app_name: str = Field(default='nexus-operator', alias='OPERATOR_APP_NAME')
    app_env: str = Field(default='dev', alias='OPERATOR_ENV')
    app_port: int = Field(default=8000, alias='OPERATOR_PORT')

    core_base_url: str = Field(default='http://nexus-core:8080', alias='NEXUS_CORE_BASE_URL')
    core_api_key: str = Field(default='nexus-operator-local-key', alias='NEXUS_CORE_API_KEY')
    core_timeout_seconds: float = Field(default=5.0, alias='NEXUS_CORE_TIMEOUT_SECONDS')

    poll_interval_seconds: int = Field(default=10, alias='OPERATOR_POLL_INTERVAL_SECONDS')
    poll_batch_size: int = Field(default=100, alias='OPERATOR_POLL_BATCH_SIZE')
    deny_ratio_threshold: float = Field(default=0.35, alias='OPERATOR_DENY_RATIO_THRESHOLD')
    min_events_for_signal: int = Field(default=20, alias='OPERATOR_MIN_EVENTS_FOR_SIGNAL')
    action_cooldown_seconds: int = Field(default=300, alias='OPERATOR_ACTION_COOLDOWN_SECONDS')
    action_ttl_seconds: int = Field(default=300, alias='OPERATOR_ACTION_TTL_SECONDS')

    operator_internal_key: str = Field(default='operator-internal-key', alias='OPERATOR_INTERNAL_KEY')


settings = Settings()
