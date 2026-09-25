"""Settings, loaded from environment variables (see .env.example at the repo
root). db_url defaults to a local SQLite file for zero-setup dev;
docker-compose overrides LOGKRAMA_DB_URL to the real Postgres DSN. Presto's
`meta` catalog only ever points at that Postgres database directly — SQLite
is dev-only and never referenced by any Presto catalog config (see
docs/DECISIONS.md D-004).
"""

from pathlib import Path

from pydantic_settings import BaseSettings, SettingsConfigDict

# Local dev: services/control-plane/app/config.py -> repo root is 3 parents
# up. Containerized (deploy/control-plane/Dockerfile copies app/ straight to
# /app/app/): that tree is only 1 level deep, so parents[3] would raise
# IndexError before Settings ever gets a chance to read LOGKRAMA_REPO_ROOT from
# the environment — fall back to cwd (Dockerfile sets WORKDIR /app) instead
# of crashing on a directory structure this default was never meant to see.
try:
    _REPO_ROOT = Path(__file__).resolve().parents[3]
except IndexError:
    _REPO_ROOT = Path.cwd()


class Settings(BaseSettings):
    model_config = SettingsConfigDict(env_prefix="LOGKRAMA_", extra="ignore")

    db_url: str = "sqlite:///./logkrama_dev.db"
    jwt_secret: str = "change_me_dev_only"
    jwt_algorithm: str = "HS256"
    jwt_expire_minutes: int = 60 * 8

    api_addr: str = "0.0.0.0:8000"

    presto_host: str = "localhost"
    presto_port: int = 8080
    presto_user: str = "logkrama"

    kafka_brokers: str = "localhost:29092"
    kafka_topic_control_parsers: str = "logkrama.control.parsers"
    kafka_topic_raw_refs: str = "logkrama.raw.refs"

    # /v1/stats/pipeline scrapes these directly (see routers/stats.py). Local
    # dev: the collector/processor run as bare processes on localhost.
    # Containerized: docker-compose.yml overrides these to the internal
    # service DNS names (http://logkrama-collector:9100/metrics etc.) — "localhost"
    # from inside the control-plane container is the container's own loopback,
    # not a sibling container's.
    collector_metrics_url: str = "http://localhost:9100/metrics"
    processor_metrics_url: str = "http://localhost:9101/metrics"

    vault_local_dir: str = "./data/vault"
    minio_endpoint: str = ""
    minio_access_key: str = ""
    minio_secret_key: str = ""
    minio_raw_bucket: str = "logkrama-raw"
    minio_use_ssl: bool = False

    repo_root: str = str(_REPO_ROOT)
    packs_dir: str = ""  # defaults to <repo_root>/packs if empty
    ollama_url: str = ""
    ollama_model: str = "llama3.2"


settings = Settings()
if not settings.packs_dir:
    settings.packs_dir = str(Path(settings.repo_root) / "packs")
