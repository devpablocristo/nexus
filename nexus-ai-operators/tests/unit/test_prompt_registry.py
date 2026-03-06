from app.services.prompt_registry import PromptRegistry


def test_prompt_registry_loads_versioned_prompt() -> None:
    registry = PromptRegistry()

    prompt = registry.get("assistant_system", "v1")

    assert prompt.prompt_id == "assistant_system"
    assert prompt.version == "v1"
    assert prompt.owner == "nexus-ai-operators"
    assert "Nexus Operator assistant" in prompt.body
