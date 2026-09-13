from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    """환경 변수는 infra/.env.example 참고. 시크릿은 코드에 두지 않는다.

    - fake_llm=True 이면 LLM 은 FakeLLM, checkpointer 는 memory (force_db=True 로 postgres 강제 가능).
    - use_redis=False 이면 이벤트는 <workspace_root>/_events/<task_id>.jsonl 로 떨어진다 (Control Plane 없이 개발).
    """

    model_config = SettingsConfigDict(env_prefix="DEVSQUAD_", env_file=".env", extra="ignore")

    db_url: str = "postgresql://devsquad:devsquad@localhost:5432/devsquad"
    redis_url: str = "redis://localhost:6379/0"
    use_redis: bool = True
    force_db: bool = False
    workspace_root: str = "/tmp/devsquad-workspace"
    internal_token: str = "dev-internal-token"
    fake_llm: bool = False
    events_stream_key: str = "devsquad:events"


settings = Settings()
