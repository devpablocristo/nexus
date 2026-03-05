import json
from datetime import datetime, timezone
from pathlib import Path
from threading import Lock


class DeadLetterLog:
    def __init__(self, path: str = "data/dead_letters.jsonl") -> None:
        self.path = Path(path)
        self.path.parent.mkdir(parents=True, exist_ok=True)
        self._lock = Lock()

    def append(self, event_id: str, payload: dict, error: str, attempts: int) -> None:
        entry = {
            "event_id": event_id,
            "payload": payload,
            "error": error,
            "attempts": attempts,
            "failed_at": datetime.now(timezone.utc).isoformat(),
        }
        with self._lock:
            with open(self.path, "a", encoding="utf-8") as f:
                f.write(json.dumps(entry) + "\n")
