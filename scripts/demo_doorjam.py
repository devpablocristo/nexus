#!/usr/bin/env python3
"""Run a reproducible Door Jam sim-engine demo through nexus-core only."""

from __future__ import annotations

import argparse
import json
import os
import sys
import time
from collections import defaultdict
from pathlib import Path
from typing import Any
from urllib import error, parse, request


def _json_dumps(data: Any) -> bytes:
    return json.dumps(data, separators=(",", ":"), ensure_ascii=True).encode("utf-8")


class CoreClient:
    def __init__(self, base_url: str, api_key: str, scopes: str, actor: str, timeout_s: float) -> None:
        self.base_url = base_url.rstrip("/")
        self.timeout_s = timeout_s
        self.base_headers = {
            "Content-Type": "application/json",
            "X-NEXUS-GATEWAY-KEY": api_key,
            "X-NEXUS-SCOPES": scopes,
            "X-NEXUS-ACTOR": actor,
        }

    def call(self, method: str, path: str, payload: Any | None = None) -> tuple[int, dict[str, Any]]:
        body = None if payload is None else _json_dumps(payload)
        req = request.Request(self.base_url + path, data=body, method=method, headers=self.base_headers)
        last_exc: Exception | None = None
        for attempt in range(5):
            try:
                with request.urlopen(req, timeout=self.timeout_s) as resp:
                    raw = resp.read()
                    data = json.loads(raw.decode("utf-8")) if raw else {}
                    return int(resp.status), data
            except error.HTTPError as exc:
                raw = exc.read()
                try:
                    data = json.loads(raw.decode("utf-8")) if raw else {}
                except Exception:
                    data = {"error": {"message": raw.decode("utf-8", errors="replace")}}
                return int(exc.code), data
            except Exception as exc:  # pragma: no cover - transient infra/network path
                last_exc = exc
                if attempt == 4:
                    raise
                time.sleep(0.2 * (attempt + 1))
        if last_exc is not None:
            raise last_exc
        raise RuntimeError("unexpected call retry state")


def require_status(status: int, body: dict[str, Any], expected: int, what: str) -> dict[str, Any]:
    if status != expected:
        raise RuntimeError(f"{what} failed ({status}): {json.dumps(body, ensure_ascii=True)}")
    return body


def create_run(client: CoreClient, seed: int, agent_count: int) -> str:
    status, body = client.call("POST", "/v1/world/run/create", {"seed": seed, "agent_count": agent_count})
    data = require_status(status, body, 200, "create run")
    run_id = str(data.get("run_id", "")).strip()
    if not run_id:
        raise RuntimeError(f"create run missing run_id: {json.dumps(data, ensure_ascii=True)}")
    return run_id


def resolve_org_id(client: CoreClient, run_id: str) -> str:
    for _ in range(8):
        status, body = client.call("GET", "/v1/world/runs?limit=200")
        data = require_status(status, body, 200, "list runs")
        for item in data.get("items", []):
            if str(item.get("run_id", "")) == run_id:
                org_id = str(item.get("org_id", ""))
                if org_id:
                    return org_id
        time.sleep(0.2)
    raise RuntimeError(f"org_id for run {run_id} not found")


def run_moves(
    client: CoreClient,
    run_id: str,
    org_id: str,
    steps: int,
    agent_count: int,
    coordinated: bool,
    gate_mod: int,
) -> dict[str, int]:
    counts: dict[str, int] = {
        "attempted_calls": 0,
        "success_calls": 0,
        "policy_denied": 0,
        "rate_limited": 0,
        "other_failures": 0,
    }
    for step in range(1, steps + 1):
        for idx in range(1, agent_count + 1):
            if coordinated and ((idx + step) % max(2, gate_mod) != 0):
                continue
            agent_id = f"agent-{idx:03d}"
            counts["attempted_calls"] += 1
            status, body = client.call(
                "POST",
                "/v1/run",
                {
                    "tool_name": "world.move",
                    "input": {
                        "org_id": org_id,
                        "agent_id": agent_id,
                        "run_id": run_id,
                        "step_id": step,
                        "target": {"x": 30, "y": 15},
                        "speed": 1.0,
                    },
                    "context": {"scenario": "door_jam_coord" if coordinated else "door_jam_baseline"},
                },
            )
            tool_status = str(body.get("status", ""))
            if status == 200 and tool_status == "success":
                counts["success_calls"] += 1
                continue

            err = body.get("error", {})
            code = str(err.get("code", ""))
            if code == "POLICY_DENIED":
                counts["policy_denied"] += 1
            elif code == "RATE_LIMITED":
                counts["rate_limited"] += 1
            else:
                counts["other_failures"] += 1
    return counts


def run_observe_burst(
    client: CoreClient,
    run_id: str,
    org_id: str,
    step_id: int,
    agent_count: int,
    calls: int,
) -> dict[str, int]:
    counts: dict[str, int] = {
        "attempted_calls": 0,
        "success_calls": 0,
        "policy_denied": 0,
        "rate_limited": 0,
        "other_failures": 0,
    }
    if calls <= 0:
        return counts
    for idx in range(calls):
        agent_id = f"agent-{(idx % max(1, agent_count)) + 1:03d}"
        counts["attempted_calls"] += 1
        status, body = client.call(
            "POST",
            "/v1/run",
            {
                "tool_name": "world.observe",
                "input": {
                    "org_id": org_id,
                    "agent_id": agent_id,
                    "run_id": run_id,
                    "step_id": step_id,
                },
                "context": {"scenario": "door_jam_observe_burst"},
            },
        )
        tool_status = str(body.get("status", ""))
        if status == 200 and tool_status == "success":
            counts["success_calls"] += 1
            continue

        err = body.get("error", {})
        code = str(err.get("code", ""))
        if code == "POLICY_DENIED":
            counts["policy_denied"] += 1
        elif code == "RATE_LIMITED":
            counts["rate_limited"] += 1
        else:
            counts["other_failures"] += 1
    return counts


def merge_counts(base: dict[str, int], extra: dict[str, int]) -> dict[str, int]:
    out = dict(base)
    for key in ("attempted_calls", "success_calls", "policy_denied", "rate_limited", "other_failures"):
        out[key] = int(out.get(key, 0)) + int(extra.get(key, 0))
    return out


def fetch_world_events(client: CoreClient, run_id: str) -> list[dict[str, Any]]:
    all_items: list[dict[str, Any]] = []
    from_seq = 0
    limit = 200
    while True:
        q = parse.urlencode({"run_id": run_id, "from_seq": from_seq, "limit": limit})
        status, body = client.call("GET", f"/v1/world/events?{q}")
        data = require_status(status, body, 200, "list world events")
        items = data.get("items", [])
        if not items:
            break
        all_items.extend(items)
        next_seq = int(data.get("next_seq", from_seq))
        if next_seq <= from_seq:
            break
        from_seq = next_seq
        if len(items) < limit:
            break
    return all_items


def state_hashes(client: CoreClient, run_id: str, steps: int) -> tuple[dict[str, str], str]:
    key_steps = sorted(set([0, max(1, steps // 3), max(2, (2 * steps) // 3), steps]))
    hashes: dict[str, str] = {}
    latest_hash = ""
    for step in key_steps:
        q = parse.urlencode({"run_id": run_id, "step_id": step})
        status, body = client.call("GET", f"/v1/world/state?{q}")
        if status == 200 and body.get("state_hash"):
            hashes[str(step)] = str(body["state_hash"])
            latest_hash = str(body["state_hash"])
    status, body = client.call("GET", f"/v1/world/state?{parse.urlencode({'run_id': run_id})}")
    if status == 200 and body.get("state_hash"):
        latest_hash = str(body["state_hash"])
    return hashes, latest_hash


def analyze_events(items: list[dict[str, Any]]) -> dict[str, Any]:
    by_type: dict[str, int] = defaultdict(int)
    heatmap: dict[str, int] = defaultdict(int)
    loops_by_agent: dict[str, int] = defaultdict(int)
    trace_by_agent: dict[str, list[str]] = defaultdict(list)

    for item in items:
        payload = item.get("payload", {}) if isinstance(item.get("payload"), dict) else {}
        event_type = str(payload.get("event_type", item.get("tool_name", "")))
        by_type[event_type] += 1
        agent_id = str(payload.get("agent_id", item.get("agent_id", "")))

        result = payload.get("result", {})
        if isinstance(result, dict):
            new_state = result.get("new_state", {})
            if isinstance(new_state, dict) and "x" in new_state and "y" in new_state:
                key = f"{int(float(new_state['x']))}:{int(float(new_state['y']))}"
                heatmap[key] += 1

        if agent_id and event_type.startswith("agent."):
            trace = trace_by_agent[agent_id]
            trace.append(event_type)
            if len(trace) >= 4 and trace[-1] == trace[-3] and trace[-2] == trace[-4]:
                loops_by_agent[agent_id] += 1

    collisions = by_type.get("agent.collided", 0) + by_type.get("agent.blocked", 0)
    loops_total = sum(loops_by_agent.values())
    heat = sorted(heatmap.items(), key=lambda kv: kv[1], reverse=True)[:8]

    return {
        "events_total": len(items),
        "by_type": dict(sorted(by_type.items(), key=lambda kv: kv[0])),
        "collisions_and_blocked": collisions,
        "loops_detected": loops_total,
        "top_congestion_cells": [{"cell": k, "hits": v} for k, v in heat],
    }


def replay_run(client: CoreClient, run_id: str) -> dict[str, Any]:
    status, body = client.call("POST", "/v1/world/replay", {"run_id": run_id})
    return require_status(status, body, 200, "replay run")


def write_golden(baseline_events: list[dict[str, Any]], checksums: dict[str, Any], max_step: int) -> None:
    out_dir = Path("golden_runs/door_jam")
    out_dir.mkdir(parents=True, exist_ok=True)

    def sanitize(value: Any) -> Any:
        if isinstance(value, dict):
            out: dict[str, Any] = {}
            for k, v in value.items():
                if k == "run_id":
                    out[k] = "RUN_BASELINE"
                    continue
                if k == "org_id":
                    out[k] = "ORG_DEMO"
                    continue
                if k == "request_id":
                    out[k] = "REQ"
                    continue
                if k == "policy_id" and v is not None:
                    out[k] = "POLICY"
                    continue
                if k in ("timestamp", "created_at"):
                    out[k] = "1970-01-01T00:00:00Z"
                    continue
                out[k] = sanitize(v)
            return out
        if isinstance(value, list):
            return [sanitize(v) for v in value]
        return value

    def normalize_event(item: dict[str, Any]) -> dict[str, Any]:
        out = sanitize(json.loads(json.dumps(item)))
        out["id"] = int(out.get("seq", 0))
        return out

    baseline_path = out_dir / "baseline.jsonl"
    with baseline_path.open("w", encoding="utf-8") as fh:
        for item in baseline_events:
            try:
                step_id = int(item.get("step_id", 0))
            except Exception:
                step_id = 0
            if step_id > max_step:
                continue
            fh.write(json.dumps(normalize_event(item), ensure_ascii=True, sort_keys=True))
            fh.write("\n")

    checksums_path = out_dir / "checksums.json"
    with checksums_path.open("w", encoding="utf-8") as fh:
        json.dump(checksums, fh, ensure_ascii=True, sort_keys=True, indent=2)
        fh.write("\n")


def main() -> int:
    parser = argparse.ArgumentParser(description="Run Door Jam demo for sim-engine through nexus-core.")
    parser.add_argument("--core-url", default=os.getenv("NEXUS_CORE_URL", "http://localhost:8080"))
    parser.add_argument("--api-key", default=os.getenv("NEXUS_DEMO_API_KEY", "nexus-tower-local-key"))
    parser.add_argument(
        "--scopes",
        default=os.getenv(
            "NEXUS_SCOPES",
            "gateway:run,admin:console:read,admin:console:write,audit:read,tools:read,tools:write,policy:read,policy:write,egress:read,egress:write",
        ),
    )
    parser.add_argument("--actor", default="tower/doorjam-demo")
    parser.add_argument("--timeout", type=float, default=20.0)
    parser.add_argument("--agents", type=int, default=50)
    parser.add_argument("--steps", type=int, default=48)
    parser.add_argument("--coord-gate-mod", type=int, default=4)
    parser.add_argument("--seed-baseline", type=int, default=424242)
    parser.add_argument("--seed-coord", type=int, default=424243)
    parser.add_argument("--skip-golden", action="store_true")
    args = parser.parse_args()

    client = CoreClient(args.core_url, args.api_key, args.scopes, args.actor, args.timeout)

    baseline_run = create_run(client, args.seed_baseline, args.agents)
    coordinated_run = create_run(client, args.seed_coord, args.agents)

    baseline_org = resolve_org_id(client, baseline_run)
    coordinated_org = resolve_org_id(client, coordinated_run)

    baseline_calls = run_moves(client, baseline_run, baseline_org, args.steps, args.agents, coordinated=False, gate_mod=args.coord_gate_mod)
    coord_calls = run_moves(client, coordinated_run, coordinated_org, args.steps, args.agents, coordinated=True, gate_mod=args.coord_gate_mod)
    baseline_observe = run_observe_burst(client, baseline_run, baseline_org, args.steps + 1, args.agents, calls=140)
    coord_observe = run_observe_burst(client, coordinated_run, coordinated_org, args.steps + 1, args.agents, calls=80)
    baseline_calls = merge_counts(baseline_calls, baseline_observe)
    coord_calls = merge_counts(coord_calls, coord_observe)

    baseline_events = fetch_world_events(client, baseline_run)
    coordinated_events = fetch_world_events(client, coordinated_run)

    baseline_analysis = analyze_events(baseline_events)
    coordinated_analysis = analyze_events(coordinated_events)

    baseline_hashes, baseline_latest = state_hashes(client, baseline_run, args.steps)
    coordinated_hashes, coordinated_latest = state_hashes(client, coordinated_run, args.steps)

    replay_baseline = replay_run(client, baseline_run)
    replay_coord = replay_run(client, coordinated_run)

    checksums = {
        "baseline": baseline_hashes,
        "coordinated": coordinated_hashes,
        "determinism_check": {
            "baseline_latest_hash": baseline_latest,
            "baseline_replay_hash": str(replay_baseline.get("state_hash", "")),
            "coordinated_latest_hash": coordinated_latest,
            "coordinated_replay_hash": str(replay_coord.get("state_hash", "")),
        },
    }

    if not args.skip_golden:
        write_golden(baseline_events, checksums, args.steps)

    summary = {
        "baseline": {
            "run_id": baseline_run,
            "calls": baseline_calls,
            "analysis": baseline_analysis,
            "hashes": baseline_hashes,
            "latest_hash": baseline_latest,
            "replay_hash": replay_baseline.get("state_hash", ""),
        },
        "coordinated": {
            "run_id": coordinated_run,
            "calls": coord_calls,
            "analysis": coordinated_analysis,
            "hashes": coordinated_hashes,
            "latest_hash": coordinated_latest,
            "replay_hash": replay_coord.get("state_hash", ""),
        },
    }
    print(json.dumps(summary, ensure_ascii=True, indent=2, sort_keys=True))

    if baseline_latest and str(replay_baseline.get("state_hash", "")) and baseline_latest != str(replay_baseline.get("state_hash", "")):
        print("determinism mismatch for baseline run", file=sys.stderr)
        return 2
    if coordinated_latest and str(replay_coord.get("state_hash", "")) and coordinated_latest != str(replay_coord.get("state_hash", "")):
        print("determinism mismatch for coordinated run", file=sys.stderr)
        return 2
    if baseline_calls.get("rate_limited", 0) == 0 and coord_calls.get("rate_limited", 0) == 0:
        print("expected at least one rate_limited call in door jam demo", file=sys.stderr)
        return 2

    return 0


if __name__ == "__main__":
    raise SystemExit(main())
