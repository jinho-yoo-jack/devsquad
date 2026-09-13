from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    """환경 변수는 infra/.env.example 참고. 시크릿은 코드에 두지 않는다."""

    model_config = SettingsConfigDict(env_prefix="DEVSQUAD_", env_file=".env", extra="ignore")

    db_url: str = "postgresql://devsquad:devsquad@localhost:5432/devsquad"
    redis_url: str = "redis://localhost:6379/0"
    workspace_root: str = "/tmp/devsquad-workspace"
    internal_token: str = "dev-internal-token"
    fake_llm: bool = False
    events_stream_key: str = "devsquad:events"


settings = Settings()
