import type { OAuthCallbackPayload, TokenFilePayload } from './types';

export const codexOAuthPortalUrl = 'https://auth.openai.com/';

export function parseOAuthCallbackUrl(input: string): OAuthCallbackPayload {
  const url = new URL(input.trim());
  const hash = url.hash.startsWith('#') ? url.hash.slice(1) : url.hash;
  const source = hash || url.search.slice(1);
  const params = new URLSearchParams(source);
  const refreshToken = params.get('refresh_token') ?? params.get('rk') ?? '';
  const accessToken = params.get('access_token') ?? '';
  const idToken = params.get('id_token') ?? '';

  if (!refreshToken && !accessToken && !idToken) {
    throw new Error('回调地址中至少需要一个令牌字段');
  }

  return {
    refreshToken,
    accessToken,
    idToken,
    accountId: params.get('account_id') ?? '',
    email: params.get('email') ?? '',
    expiresAt: params.get('expires_at') ?? params.get('expired') ?? '',
  };
}

export function buildOAuthTokenFilePayload(input: string): TokenFilePayload {
  const parsed = parseOAuthCallbackUrl(input);

  return {
    type: 'codex',
    refresh_token: parsed.refreshToken,
    rk: parsed.refreshToken,
    access_token: parsed.accessToken,
    id_token: parsed.idToken,
    account_id: parsed.accountId,
    email: parsed.email,
    expired: parsed.expiresAt,
  };
}

export function buildManualTokenFilePayload(input: Partial<TokenFilePayload>): TokenFilePayload {
  const refreshToken = input.refresh_token?.trim() ?? input.rk?.trim() ?? '';

  return {
    ...(input.type?.trim() ? { type: input.type.trim() } : {}),
    ...(refreshToken ? { refresh_token: refreshToken, rk: refreshToken } : {}),
    ...(input.access_token?.trim() ? { access_token: input.access_token.trim() } : {}),
    ...(input.id_token?.trim() ? { id_token: input.id_token.trim() } : {}),
    ...(input.account_id?.trim() ? { account_id: input.account_id.trim() } : {}),
    ...(input.email?.trim() ? { email: input.email.trim() } : {}),
    ...(input.expired?.trim() ? { expired: input.expired.trim() } : {}),
  };
}
