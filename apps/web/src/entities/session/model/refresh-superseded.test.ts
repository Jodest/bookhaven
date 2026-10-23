import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { ApiError, api } from '@bookhaven/api-client';

// The shared api-client retries POST /auth/refresh once, after ~1 s, when the
// backend answers 401 `refresh_token_superseded` (auth/session spec).

const fetchMock = vi.fn();

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
}

const superseded = () =>
  jsonResponse(401, { error: 'refresh token superseded', code: 'refresh_token_superseded' });
const invalid = () => jsonResponse(401, { error: 'invalid or expired refresh token' });
const ok = () =>
  jsonResponse(200, { access_token: 'access-2', refresh_token: 'refresh-2', expires_in: 900 });

describe('refresh with refresh_token_superseded retry', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    fetchMock.mockReset();
    vi.stubGlobal('fetch', fetchMock);
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
  });

  it('retries once after a superseded answer and resolves with the second tokens', async () => {
    fetchMock.mockResolvedValueOnce(superseded()).mockResolvedValueOnce(ok());

    const promise = api.auth.refresh();
    await vi.advanceTimersByTimeAsync(1100);

    await expect(promise).resolves.toMatchObject({ accessToken: 'access-2', refreshToken: 'refresh-2' });
    expect(fetchMock).toHaveBeenCalledTimes(2);
  });

  it('rejects after exactly two calls when superseded twice', async () => {
    fetchMock.mockResolvedValueOnce(superseded()).mockResolvedValueOnce(superseded());

    const promise = api.auth.refresh();
    const assertion = expect(promise).rejects.toBeInstanceOf(ApiError);
    await vi.advanceTimersByTimeAsync(1100);
    await assertion;
    expect(fetchMock).toHaveBeenCalledTimes(2);
  });

  it('does not retry on a plain 401', async () => {
    fetchMock.mockResolvedValueOnce(invalid());

    await expect(api.auth.refresh()).rejects.toBeInstanceOf(ApiError);
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  it('keeps concurrent callers on one in-flight attempt including the retry', async () => {
    fetchMock.mockResolvedValueOnce(superseded()).mockResolvedValueOnce(ok());

    const first = api.auth.refresh();
    const second = api.auth.refresh();
    await vi.advanceTimersByTimeAsync(1100);

    await expect(first).resolves.toMatchObject({ accessToken: 'access-2' });
    await expect(second).resolves.toMatchObject({ accessToken: 'access-2' });
    expect(fetchMock).toHaveBeenCalledTimes(2);
  });
});
