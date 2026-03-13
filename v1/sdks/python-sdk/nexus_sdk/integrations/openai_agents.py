"""OpenAI Agents SDK integration for Nexus Gateway.

Wraps Nexus-registered tools as ``FunctionTool`` objects compatible with
the OpenAI Agents SDK (``openai-agents``), so every tool call flows
through the Nexus pipeline.

Usage::

    from nexus_sdk import NexusClient
    from nexus_sdk.integrations.openai_agents import nexus_function_tools
    from agents import Agent, Runner

    client = NexusClient(base_url="http://localhost:8080", api_key="sk-...")
    tools = nexus_function_tools(client)

    agent = Agent(name="demo", instructions="...", tools=tools)
    result = Runner.run_sync(agent, "What's the weather in Madrid?")
"""

from __future__ import annotations

import json
from typing import Any

from nexus_sdk.client import NexusClient

try:
    from agents import FunctionTool, RunContext
except ImportError as exc:
    raise ImportError(
        "openai-agents is required for the OpenAI Agents integration. "
        "Install with: pip install 'nexus-sdk[openai-agents]'"
    ) from exc


def _make_runner(client: NexusClient, tool_name: str):
    """Return an async callable that invokes Nexus and returns a string."""

    async def _run(ctx: RunContext, **kwargs: Any) -> str:
        resp = client.run(tool_name, input=kwargs)
        if resp.denied:
            return f"[DENIED] {resp.reason}"
        if resp.error:
            return f"[ERROR] {resp.error}"
        return json.dumps(resp.result) if resp.result is not None else ""

    return _run


def nexus_function_tool(
    client: NexusClient,
    tool_name: str,
    *,
    description: str = "",
) -> FunctionTool:
    """Create a single FunctionTool backed by a Nexus tool."""
    return FunctionTool(
        name=tool_name,
        description=description or f"Nexus tool: {tool_name}",
        params_json_schema={"type": "object", "additionalProperties": True},
        on_invoke_tool=_make_runner(client, tool_name),
    )


def nexus_function_tools(
    client: NexusClient,
    *,
    filter_enabled: bool = True,
) -> list[FunctionTool]:
    """Fetch all tools from Nexus and return them as FunctionTool list."""
    tools = client.list_tools()
    out: list[FunctionTool] = []
    for t in tools:
        if filter_enabled and not t.enabled:
            continue
        schema = t.input_schema if t.input_schema else {"type": "object", "additionalProperties": True}
        out.append(
            FunctionTool(
                name=t.name,
                description=t.description or t.name,
                params_json_schema=schema,
                on_invoke_tool=_make_runner(client, t.name),
            )
        )
    return out
