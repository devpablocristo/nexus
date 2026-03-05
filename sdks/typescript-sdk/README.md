# Nexus TypeScript SDK

TypeScript client for Nexus Core API (gateway, tools, policies, audit, approvals, alerts).

## Installation

```bash
npm install @nexus/sdk
```

## Quick Start

```ts
import { NexusClient } from '@nexus/sdk';

const client = new NexusClient({
  baseUrl: 'https://api.nexus.io',
  apiKey: process.env.NEXUS_API_KEY,
});

const run = await client.run('echo', { message: 'hello' });
console.log(run.status, run.result);
```

## API Reference

Core gateway:

- `run(toolName, input?, context?, opts?)`
- `simulate(toolName, input?, context?)`

Tools:

- `listTools()`
- `getTool(name)`
- `createTool(options)`
- `updateTool(name, fields)`

Policies and egress:

- `listPolicies(toolName)`
- `createPolicy(toolName, options)`
- `listEgressRules(toolName)`
- `addEgressRule(toolName, host, enabled?)`

Audit and approvals:

- `queryAudit(options?)`
- `listApprovals(limit?)`
- `getApproval(id)`
- `approve(id, decidedBy?)`
- `reject(id, decidedBy?)`

Alerts and sessions:

- `listAlertRules()`
- `createAlertRule(options)`
- `deleteAlertRule(id)`
- `getSession(sessionId)`

Onboarding:

- `createOrg(name, scopes?)`
- `health()`

## Configuration

`NexusClient` options:

- `baseUrl`: Core API base URL (default: `http://localhost:8080`)
- `apiKey`: value for `X-NEXUS-CORE-KEY`
- `actor`: value for `X-NEXUS-ACTOR`
- `role`: value for `X-NEXUS-ROLE`
- `scopes`: value for `X-NEXUS-SCOPES`
- `fetch`: custom fetch implementation for Node runtimes

## Error Handling

Errors are raised as `NexusError` with:

- `status` (HTTP status)
- `code` (API error code)
- `errorMessage` (API message)
