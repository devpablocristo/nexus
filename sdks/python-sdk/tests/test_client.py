import httpx
import pytest
import respx

from nexus_sdk import NexusClient, NexusError


BASE = "http://nexus-test:8080"


@respx.mock
def test_run_allow():
    respx.post(f"{BASE}/v1/run").mock(
        return_value=httpx.Response(200, json={
            "request_id": "r1",
            "decision": "allow",
            "tool_name": "echo",
            "status": "success",
            "result": {"echoed": True},
            "latency_ms": 42,
        })
    )
    with NexusClient(base_url=BASE, api_key="key") as c:
        resp = c.run("echo", {"hello": "world"})
    assert resp.allowed
    assert resp.status == "success"
    assert resp.result == {"echoed": True}


@respx.mock
def test_run_deny():
    respx.post(f"{BASE}/v1/run").mock(
        return_value=httpx.Response(403, json={
            "request_id": "r2",
            "decision": "deny",
            "status": "blocked",
            "reason": "amount too high",
            "error": {"code": "POLICY_DENIED", "message": "amount too high"},
        })
    )
    with NexusClient(base_url=BASE, api_key="key") as c:
        resp = c.run("transfer", {"amount": 5000})
    assert resp.denied
    assert resp.status == "blocked"


@respx.mock
def test_simulate():
    respx.post(f"{BASE}/v1/run/simulate").mock(
        return_value=httpx.Response(200, json={
            "request_id": "s1",
            "decision": "allow",
            "tool_name": "echo",
            "status": "success",
            "explain": {"would_execute": True},
            "latency_ms": 5,
        })
    )
    with NexusClient(base_url=BASE, api_key="key") as c:
        resp = c.simulate("echo", {"a": 1})
    assert resp.allowed
    assert resp.explain["would_execute"] is True


@respx.mock
def test_list_tools():
    respx.get(f"{BASE}/v1/tools").mock(
        return_value=httpx.Response(200, json={
            "items": [
                {"id": "1", "name": "echo", "kind": "http", "method": "POST", "url": "http://x/echo", "action_type": "read", "enabled": True},
            ]
        })
    )
    with NexusClient(base_url=BASE, api_key="key") as c:
        tools = c.list_tools()
    assert len(tools) == 1
    assert tools[0].name == "echo"


@respx.mock
def test_query_audit():
    respx.get(f"{BASE}/v1/audit").mock(
        return_value=httpx.Response(200, json={
            "items": [
                {"request_id": "r1", "tool_name": "echo", "decision": "allow", "status": "success", "latency_ms": 10, "created_at": "2025-01-01T00:00:00Z"},
            ]
        })
    )
    with NexusClient(base_url=BASE, api_key="key") as c:
        events = c.query_audit(tool_name="echo")
    assert len(events) == 1
    assert events[0].decision == "allow"


@respx.mock
def test_server_error_raises():
    respx.get(f"{BASE}/v1/tools").mock(
        return_value=httpx.Response(500, json={"error": {"code": "INTERNAL", "message": "boom"}})
    )
    with NexusClient(base_url=BASE, api_key="key") as c:
        with pytest.raises(NexusError) as exc_info:
            c.list_tools()
    assert exc_info.value.status == 500


@respx.mock
def test_idempotency_headers():
    route = respx.post(f"{BASE}/v1/run").mock(
        return_value=httpx.Response(200, json={
            "request_id": "r3",
            "decision": "allow",
            "tool_name": "transfer",
            "status": "success",
            "result": {"ok": True},
            "latency_ms": 15,
            "idempotency": {"present": True, "outcome": "new"},
        })
    )
    with NexusClient(base_url=BASE, api_key="key") as c:
        resp = c.run("transfer", {"amount": 100}, idempotency_key="idem-1", timeout_ms=5000)
    assert resp.idempotency is not None
    assert resp.idempotency.present is True
    req = route.calls[0].request
    assert req.headers["Idempotency-Key"] == "idem-1"
    assert req.headers["X-Timeout-Ms"] == "5000"
