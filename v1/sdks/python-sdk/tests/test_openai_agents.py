"""Tests for the OpenAI Agents SDK integration."""

from __future__ import annotations

from unittest.mock import MagicMock

import pytest

from nexus_sdk.types import RunResponse, Tool

try:
    from nexus_sdk.integrations.openai_agents import nexus_function_tool, nexus_function_tools

    HAS_AGENTS = True
except ImportError:
    HAS_AGENTS = False

pytestmark = pytest.mark.skipif(not HAS_AGENTS, reason="openai-agents not installed")


def _mock_client(run_resp: dict | None = None, tools: list | None = None) -> MagicMock:
    client = MagicMock()
    if run_resp is not None:
        client.run.return_value = RunResponse.from_dict(run_resp)
    if tools is not None:
        client.list_tools.return_value = tools
    return client


class TestNexusFunctionTool:
    def test_creates_tool(self):
        client = _mock_client()
        ft = nexus_function_tool(client, "weather", description="Get weather")
        assert ft.name == "weather"
        assert ft.description == "Get weather"

    def test_fetches_all_tools(self):
        tools = [
            Tool(id="1", name="a", kind="http", method="POST", url="http://a", action_type="read", enabled=True, description="Tool A"),
            Tool(id="2", name="b", kind="http", method="POST", url="http://b", action_type="read", enabled=False),
        ]
        client = _mock_client(tools=tools)
        fts = nexus_function_tools(client)
        assert len(fts) == 1
        assert fts[0].name == "a"
