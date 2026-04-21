import { describe, expect, it } from 'vitest';
import { parseOAuthCallbackUrl } from '../lib/oauth';

describe('parseOAuthCallbackUrl', () => {
  it('extracts tokens from callback url hash', () => {
    const parsed = parseOAuthCallbackUrl('http://127.0.0.1:1455/callback#access_token=at&id_token=it&refresh_token=rt');
    expect(parsed.refreshToken).toBe('rt');
    expect(parsed.idToken).toBe('it');
  });

  it('accepts callback urls with access token only', () => {
    const parsed = parseOAuthCallbackUrl('http://127.0.0.1:1455/callback#access_token=at');
    expect(parsed.accessToken).toBe('at');
    expect(parsed.refreshToken).toBe('');
  });

  it('throws when no oauth token fields exist', () => {
    expect(() => parseOAuthCallbackUrl('http://127.0.0.1:1455/callback#state=abc')).toThrow(/至少需一项/i);
  });
});
