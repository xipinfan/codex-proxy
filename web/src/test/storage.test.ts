import { describe, expect, it } from 'vitest';
import { loadConsoleSettings, saveConsoleSettings } from '../lib/storage';

describe('console settings storage', () => {
  it('loads defaults when localStorage is empty', () => {
    expect(loadConsoleSettings()).toMatchObject({
      baseUrl: '',
      apiKey: '',
      pageSize: 20,
      autoRefreshSeconds: 0,
      includeQuota: true,
    });
  });

  it('round-trips persisted settings', () => {
    saveConsoleSettings({
      baseUrl: 'http://127.0.0.1:8080',
      apiKey: 'sk-test',
      pageSize: 50,
      autoRefreshSeconds: 30,
      includeQuota: false,
    });

    expect(loadConsoleSettings()).toMatchObject({
      baseUrl: 'http://127.0.0.1:8080',
      apiKey: 'sk-test',
      pageSize: 50,
      autoRefreshSeconds: 30,
      includeQuota: false,
    });
  });
});
