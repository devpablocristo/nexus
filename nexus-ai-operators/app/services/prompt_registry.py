from __future__ import annotations

from dataclasses import dataclass
from pathlib import Path

from app.prompts.registry import PROMPT_FILES


class PromptRegistryError(RuntimeError):
    pass


@dataclass(frozen=True)
class RuntimePrompt:
    prompt_id: str
    version: str
    owner: str
    purpose: str
    body: str
    path: Path


class PromptRegistry:
    def __init__(self, prompts_dir: Path | None = None) -> None:
        self._prompts_dir = prompts_dir or Path(__file__).resolve().parents[1] / "prompts"
        self._cache: dict[tuple[str, str], RuntimePrompt] = {}

    def get(self, prompt_id: str, version: str) -> RuntimePrompt:
        key = (prompt_id, version)
        cached = self._cache.get(key)
        if cached is not None:
            return cached

        try:
            rel_path = PROMPT_FILES[prompt_id][version]
        except KeyError as exc:
            raise PromptRegistryError(f"prompt not registered: {prompt_id}@{version}") from exc

        prompt = self._load_prompt(prompt_id, version, self._prompts_dir / rel_path)
        self._cache[key] = prompt
        return prompt

    def _load_prompt(self, prompt_id: str, version: str, path: Path) -> RuntimePrompt:
        if not path.exists():
            raise PromptRegistryError(f"prompt file missing: {path}")

        raw = path.read_text(encoding="utf-8")
        meta, body = self._split_front_matter(raw)
        loaded_id = meta.get("id", "").strip()
        loaded_version = meta.get("version", "").strip()
        owner = meta.get("owner", "").strip()
        purpose = meta.get("purpose", "").strip()

        if loaded_id != prompt_id:
            raise PromptRegistryError(f"prompt id mismatch in {path}: expected {prompt_id}, got {loaded_id}")
        if loaded_version != version:
            raise PromptRegistryError(
                f"prompt version mismatch in {path}: expected {version}, got {loaded_version}"
            )
        if not owner or not purpose:
            raise PromptRegistryError(f"prompt metadata incomplete in {path}")
        if not body:
            raise PromptRegistryError(f"prompt body is empty in {path}")

        return RuntimePrompt(
            prompt_id=loaded_id,
            version=loaded_version,
            owner=owner,
            purpose=purpose,
            body=body,
            path=path,
        )

    @staticmethod
    def _split_front_matter(raw: str) -> tuple[dict[str, str], str]:
        if not raw.startswith("---\n"):
            raise PromptRegistryError("prompt file must start with front matter")

        end = raw.find("\n---\n", 4)
        if end == -1:
            raise PromptRegistryError("prompt file front matter is not closed")

        front_matter = raw[4:end].strip()
        body = raw[end + 5 :].strip()
        meta: dict[str, str] = {}
        for line in front_matter.splitlines():
            if ":" not in line:
                raise PromptRegistryError(f"invalid prompt metadata line: {line}")
            key, value = line.split(":", 1)
            meta[key.strip()] = value.strip()
        return meta, body
