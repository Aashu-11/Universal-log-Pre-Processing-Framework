from pathlib import Path

from pydantic_settings import BaseSettings, SettingsConfigDict

# See services/control-plane/app/config.py for why this needs a fallback —
# same issue: local dev's app/config.py is 3 parents below repo root,
# containerized it's 1, so parents[3] would crash before Settings can read
# ULPF_REPO_ROOT from the environment.
try:
    _REPO_ROOT = Path(__file__).resolve().parents[3]
except IndexError:
    _REPO_ROOT = Path.cwd()


class Settings(BaseSettings):
    model_config = SettingsConfigDict(env_prefix="ULPF_", extra="ignore")

    api_addr: str = "0.0.0.0:8001"
    repo_root: str = str(_REPO_ROOT)
    control_plane_url: str = "http://localhost:8000"


settings = Settings()
