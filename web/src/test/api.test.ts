import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { fetchStats, ingestAccounts, runProgressAction } from '../lib/api';
import { defaultSettings, type ProgressEvent } from '../lib/types';

const originalFetch = globalThis.fetch;
const encoder = new TextEncoder();

function createSseResponse(chunks: string[]): Response {
  const stream = new ReadableStream({
    start(controller) {
      for (const chunk of chunks) {
        controller.enqueue(encoder.encode(chunk));
      }
      controller.close();
    },
  });

  return new Response(stream, {
    status: 200,
    headers: {
      'Content-Type': 'text/event-stream',
    },
  });
}

describe('fetchStats', () => {
  beforeEach(() => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ summary: { total: 1 }, accounts: [] }),
    } as Response);
  });

  afterEach(() => {
    globalThis.fetch = originalFetch;
  });

  it('sends admin api key and query params', async () => {
    await fetchStats(
      {
        ...defaultSettings,
        baseUrl: 'http://127.0.0.1:8080',
        apiKey: 'sk-test',
      },
      {
        page: 2,
        pageSize: 50,
        query: 'alice',
        includeQuota: true,
      },
    );

    expect(globalThis.fetch).toHaveBeenCalledWith(
      'http://127.0.0.1:8080/stats?page=2&page_size=50&include_quota=true&q=alice',
      expect.objectContaining({
        headers: expect.objectContaining({ Authorization: 'Bearer sk-test' }),
      }),
    );
  });
});

describe('ingestAccounts', () => {
  beforeEach(() => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ added: 1, updated: 0, failed: 0, pool_total: 1, errors: [] }),
    } as Response);
  });

  afterEach(() => {
    globalThis.fetch = originalFetch;
  });

  it('posts JSON payload to admin ingest endpoint', async () => {
    await ingestAccounts(
      {
        ...defaultSettings,
        baseUrl: 'http://127.0.0.1:8080',
      },
      [{ refresh_token: 'rt' }],
    );

    expect(globalThis.fetch).toHaveBeenCalledWith(
      'http://127.0.0.1:8080/admin/accounts/ingest',
      expect.objectContaining({
        method: 'POST',
        headers: expect.objectContaining({ 'Content-Type': 'application/json' }),
        body: JSON.stringify([{ refresh_token: 'rt' }]),
      }),
    );
  });
});

describe('runProgressAction', () => {
  afterEach(() => {
    globalThis.fetch = originalFetch;
  });

  it('consumes sse progress events and returns the done payload', async () => {
    const seen: ProgressEvent[] = [];
    globalThis.fetch = vi.fn().mockResolvedValue(
      createSseResponse([
        'event: item\ndata: {"type":"item","email":"a@example.com","success":true,"current":1,"total":2}\n\n',
        'event: done\ndata: {"type":"done","message":"刷新完成","total":2,"success_count":1,"failed_count":1,"remaining":2,"duration":"1s"}\n\n',
      ]),
    );

    const done = await runProgressAction(
      { ...defaultSettings, baseUrl: 'http://127.0.0.1:8080' },
      '/refresh',
      (event) => seen.push(event),
    );

    expect(seen).toHaveLength(2);
    expect(done.type).toBe('done');
    expect(done.successCount).toBe(1);
    expect(globalThis.fetch).toHaveBeenCalledWith(
      'http://127.0.0.1:8080/refresh',
      expect.objectContaining({ method: 'POST' }),
    );
  });
});
