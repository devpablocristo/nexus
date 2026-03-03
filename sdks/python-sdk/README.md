# nexus-sdk (Python)

Python SDK for [Nexus Gateway](https://github.com/your-org/nexus) — AI agent tool governance.

## Install

```bash
pip install nexus-sdk
```

## Quick start

```python
from nexus_sdk import NexusClient

client = NexusClient(base_url="http://localhost:8080", api_key="your-key")

# Execute a tool
resp = client.run("echo", input={"hello": "world"}, context={"user_id": "u_1"})
print(resp.decision, resp.result)

# Simulate (dry-run)
sim = client.simulate("transfer", input={"amount": 5000}, context={"user_id": "u_1"})
print(sim.allowed, sim.explain)
```
