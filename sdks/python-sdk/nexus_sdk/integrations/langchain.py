"""LangChain integration for Nexus Gateway.

Provides NexusTool — a LangChain BaseTool that routes every invocation
through the Nexus gateway so policies, DLP, rate-limits and audit apply
transparently.

Usage::

    from nexus_sdk import NexusClient
    from nexus_sdk.integrations.langchain import NexusTool

    client = NexusClient(base_url="http://localhost:8080", api_key="sk-...")
    tool = NexusTool(client=client, tool_name="weather-lookup")

    # use inside any LangChain agent / chain
    from langchain.agents import AgentExecutor, create_openai_tools_agent
    agent = create_openai_tools_agent(llm, [tool], prompt)
"""

from __future__ import annotations

from typing import Any, Type

from nexus_sdk.client import NexusClient
from nexus_sdk.types import NexusError

try:
    from langchain_core.tools import BaseTool
    from langchain_core.callbacks import CallbackManagerForToolRun
    from pydantic import BaseModel, Field
except ImportError as exc:
    raise ImportError(
        "langchain-core and pydantic are required for the LangChain integration. "
        "Install with: pip install 'nexus-sdk[langchain]'"
    ) from exc


class _DynamicInput(BaseModel):
    """Accepts arbitrary JSON input forwarded to Nexus."""

    input: dict[str, Any] = Field(default_factory=dict, description="Tool input payload")
    context: dict[str, Any] = Field(default_factory=dict, description="Optional call context")


class NexusTool(BaseTool):
    """LangChain tool backed by the Nexus gateway.

    Every call goes through policy evaluation, DLP, rate-limiting, egress
    checks and audit — exactly like a direct ``POST /v1/run``.
    """

    name: str = ""
    description: str = "Nexus-governed tool"
    args_schema: Type[BaseModel] = _DynamicInput
    client: Any = None  # NexusClient
    tool_name: str = ""
    idempotency_key: str | None = None
    timeout_ms: int | None = None
    raise_on_deny: bool = False

    def __init__(
        self,
        client: NexusClient,
        tool_name: str,
        *,
        description: str = "",
        idempotency_key: str | None = None,
        timeout_ms: int | None = None,
        raise_on_deny: bool = False,
    ):
        super().__init__(
            name=tool_name,
            description=description or f"Nexus tool: {tool_name}",
            client=client,
            tool_name=tool_name,
            idempotency_key=idempotency_key,
            timeout_ms=timeout_ms,
            raise_on_deny=raise_on_deny,
        )

    def _run(
        self,
        input: dict[str, Any] | None = None,
        context: dict[str, Any] | None = None,
        run_manager: CallbackManagerForToolRun | None = None,
        **kwargs: Any,
    ) -> str:
        merged_input = {**(input or {}), **kwargs}
        resp = self.client.run(
            self.tool_name,
            input=merged_input,
            context=context,
            idempotency_key=self.idempotency_key,
            timeout_ms=self.timeout_ms,
        )
        if resp.denied and self.raise_on_deny:
            raise NexusError(403, "DENIED", resp.reason or "denied by policy")
        if resp.denied:
            return f"[DENIED] {resp.reason}"
        if resp.error:
            return f"[ERROR] {resp.error}"
        import json
        return json.dumps(resp.result) if resp.result is not None else ""


class NexusToolkit:
    """Fetch all tools from Nexus and expose them as LangChain tools."""

    def __init__(
        self,
        client: NexusClient,
        *,
        raise_on_deny: bool = False,
        timeout_ms: int | None = None,
    ):
        self.client = client
        self.raise_on_deny = raise_on_deny
        self.timeout_ms = timeout_ms

    def get_tools(self) -> list[NexusTool]:
        tools = self.client.list_tools()
        return [
            NexusTool(
                client=self.client,
                tool_name=t.name,
                description=t.description or t.name,
                raise_on_deny=self.raise_on_deny,
                timeout_ms=self.timeout_ms,
            )
            for t in tools
            if t.enabled
        ]
