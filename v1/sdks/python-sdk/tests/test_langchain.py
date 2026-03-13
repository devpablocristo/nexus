"""Tests for the LangChain integration."""

from __future__ import annotations

from unittest.mock import MagicMock

import pytest

from nexus_sdk.types import RunResponse, Tool, NexusError

try:
    from nexus_sdk.integrations.langchain import NexusTool, NexusToolkit

    HAS_LANGCHAIN = True
except ImportError:
    HAS_LANGCHAIN = False

pytestmark = pytest.mark.skipif(not HAS_LANGCHAIN, reason="langchain-core not installed")


def _mock_client(run_resp: dict | None = None, tools: list | None = None) -> MagicMock:
    client = MagicMock()
    if run_resp is not None:
        client.run.return_value = RunResponse.from_dict(run_resp)
    if tools is not None:
        client.list_tools.return_value = tools
    return client


class TestNexusTool:
    def test_allow(self):
        client = _mock_client(run_resp={
            "request_id": "r1",
            "decision": "allow",
            "tool_name": "weather",
            "status": "success",
            "result": {"temp": 22},
        })
        tool = NexusTool(client=client, tool_name="weather")
        result = tool._run(input={"city": "Madrid"})
        assert "22" in result
        client.run.assert_called_once()

    def test_deny_returns_string(self):
        client = _mock_client(run_resp={
            "request_id": "r2",
            "decision": "deny",
            "tool_name": "secret",
            "status": "blocked",
            "reason": "policy violation",
        })
        tool = NexusTool(client=client, tool_name="secret")
        result = tool._run(input={})
        assert "[DENIED]" in result
        assert "policy violation" in result

    def test_deny_raises(self):
        client = _mock_client(run_resp={
            "request_id": "r3",
            "decision": "deny",
            "tool_name": "secret",
            "status": "blocked",
            "reason": "forbidden",
        })
        tool = NexusTool(client=client, tool_name="secret", raise_on_deny=True)
        with pytest.raises(NexusError):
            tool._run(input={})


class TestNexusToolkit:
    def test_fetches_enabled_tools(self):
        tools = [
            Tool(id="1", name="a", kind="http", method="POST", url="http://a", action_type="read", enabled=True),
            Tool(id="2", name="b", kind="http", method="POST", url="http://b", action_type="read", enabled=False),
            Tool(id="3", name="c", kind="http", method="GET", url="http://c", action_type="read", enabled=True),
        ]
        client = _mock_client(tools=tools)
        tk = NexusToolkit(client)
        lc_tools = tk.get_tools()
        assert len(lc_tools) == 2
        names = {t.name for t in lc_tools}
        assert names == {"a", "c"}
