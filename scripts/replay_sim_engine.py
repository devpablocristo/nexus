#!/usr/bin/env python3
"""Replay a sim-engine run via nexus-core /v1/world/replay."""

from __future__ import annotations

import argparse
import json
import os
import sys
from urllib import error, request


def call(core_url: str, api_key: str, scopes: str, actor: str, run_id: str, timeout_s: float) -> tuple[int, dict]:
    payload = json.dumps({"run_id": run_id}, separators=(",", ":"), ensure_ascii=True).encode("utf-8")
    req = request.Request(
        core_url.rstrip("/") + "/v1/world/replay",
        method="POST",
        data=payload,
        headers={
            "Content-Type": "application/json",
            "X-NEXUS-CORE-KEY": api_key,
            "X-NEXUS-SCOPES": scopes,
            "X-NEXUS-ACTOR": actor,
        },
    )
    try:
        with request.urlopen(req, timeout=timeout_s) as resp:
            raw = resp.read()
            return int(resp.status), json.loads(raw.decode("utf-8")) if raw else {}
    except error.HTTPError as exc:
        raw = exc.read()
        try:
            payload = json.loads(raw.decode("utf-8")) if raw else {}
        except Exception:
            payload = {"error": {"message": raw.decode("utf-8", errors="replace")}}
        return int(exc.code), payload


def main() -> int:
    parser = argparse.ArgumentParser(description="Replay one sim-engine run through nexus-core.")
    parser.add_argument("--run-id", required=True)
    parser.add_argument("--core-url", default=os.getenv("NEXUS_CORE_URL", "http://localhost:8080"))
    parser.add_argument("--api-key", default=os.getenv("NEXUS_DEMO_API_KEY", "nexus-core-local-key"))
    parser.add_argument("--scopes", default=os.getenv("NEXUS_SCOPES", "admin:console:write,admin:console:read"))
    parser.add_argument("--actor", default="tower/world-replay")
    parser.add_argument("--timeout", type=float, default=15.0)
    args = parser.parse_args()

    status, payload = call(args.core_url, args.api_key, args.scopes, args.actor, args.run_id, args.timeout)
    if status != 200:
        print(json.dumps({"status": status, "error": payload}, ensure_ascii=True, indent=2))
        return 1

    print(json.dumps(payload, ensure_ascii=True, indent=2, sort_keys=True))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
