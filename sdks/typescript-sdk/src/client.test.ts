import { describe, expect, it, vi } from 'vitest';

import { NexusClient } from './client.js';
import { NexusError } from './types.js';

function jsonResponse(body: unknown, init: ResponseInit = {}): Response {
  return new Response(JSON.stringify(body), {
    status: init.status ?? 200,
    headers: {
      'content-type': 'application/json',
      ...(init.headers ?? {}),
    },
  });
}

describe('NexusClient', () => {
  it('sends auth headers and decodes tool lists', async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValue(
      jsonResponse({
        items: [
          {
            id: 'tool-1',
            name: 'echo',
            kind: 'http',
            method: 'POST',
            url: 'http://example.test/echo',
            action_type: 'read',
            enabled: true,
          },
        ],
      }),
    );

    const client = new NexusClient({
      baseUrl: 'http://localhost:8080/',
      apiKey: 'key-1',
      actor: 'agent-1',
      role: 'bot',
      scopes: 'tools:run',
      fetch: fetchMock,
    });

    const tools = await client.listTools();

    expect(tools).toHaveLength(1);
    expect(tools[0]?.name).toBe('echo');
    expect(fetchMock).toHaveBeenCalledWith(
      'http://localhost:8080/v1/tools',
      expect.objectContaining({
        method: 'GET',
        headers: expect.objectContaining({
          'Content-Type': 'application/json',
          'X-NEXUS-CORE-KEY': 'key-1',
          'X-NEXUS-ACTOR': 'agent-1',
          'X-NEXUS-ROLE': 'bot',
          'X-NEXUS-SCOPES': 'tools:run',
        }),
      }),
    );
  });

  it('maps 429 responses to blocked run payloads', async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValue(
      jsonResponse(
        {
          error: {
            code: 'RATE_LIMITED',
            message: 'tenant run rate limit exceeded',
          },
        },
        { status: 429 },
      ),
    );

    const client = new NexusClient({
      baseUrl: 'http://localhost:8080',
      fetch: fetchMock,
    });

    const result = await client.run('echo', { message: 'hello' });

    expect(result).toEqual({
      request_id: '',
      decision: 'deny',
      tool_name: 'echo',
      status: 'blocked',
      error: {
        code: 'RATE_LIMITED',
        message: 'tenant run rate limit exceeded',
      },
      latency_ms: 0,
    });
  });

  it('throws NexusError for non-blocking server failures', async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValue(
      jsonResponse(
        {
          error: {
            code: 'INTERNAL',
            message: 'boom',
          },
        },
        { status: 500 },
      ),
    );

    const client = new NexusClient({
      baseUrl: 'http://localhost:8080',
      fetch: fetchMock,
    });

    await expect(client.listTools()).rejects.toEqual(new NexusError(500, 'INTERNAL', 'boom'));
  });
});
