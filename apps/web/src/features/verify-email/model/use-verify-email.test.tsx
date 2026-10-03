import { act, render, screen, waitFor } from '@testing-library/react';
import { StrictMode } from 'react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { useVerifyEmail } from './use-verify-email';

const verifyEmailMock = vi.fn();

vi.mock('@bookhaven/api-client', async () => {
  const actual = await vi.importActual<typeof import('@bookhaven/api-client')>(
    '@bookhaven/api-client'
  );
  return {
    ApiError: actual.ApiError,
    isApiError: actual.isApiError,
    api: { auth: { verifyEmail: (token: string) => verifyEmailMock(token) } },
  };
});

let retryRef: (() => void) | null = null;

function Probe({ token }: { token: string | null }) {
  const { state, retry } = useVerifyEmail(token);
  retryRef = retry;
  return <div data-testid="state">{state}</div>;
}

function renderStrict(token: string | null) {
  return render(
    <StrictMode>
      <Probe token={token} />
    </StrictMode>
  );
}

describe('useVerifyEmail', () => {
  beforeEach(() => {
    verifyEmailMock.mockReset();
    retryRef = null;
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('posts the token exactly once under StrictMode and ends in verified', async () => {
    verifyEmailMock.mockResolvedValue({ status: 'verified' });
    renderStrict('tok-1');

    await waitFor(() => expect(screen.getByTestId('state')).toHaveTextContent('verified'));
    expect(screen.getByTestId('state')).not.toHaveTextContent('already_verified');
    expect(verifyEmailMock).toHaveBeenCalledTimes(1);
    expect(verifyEmailMock).toHaveBeenCalledWith('tok-1');
  });

  it('ends in already_verified', async () => {
    verifyEmailMock.mockResolvedValue({ status: 'already_verified' });
    renderStrict('tok-2');

    await waitFor(() =>
      expect(screen.getByTestId('state')).toHaveTextContent('already_verified')
    );
  });

  it('does not call the API without a token', () => {
    renderStrict(null);

    expect(screen.getByTestId('state')).toHaveTextContent('missing_token');
    expect(verifyEmailMock).not.toHaveBeenCalled();
  });

  it('retry() re-posts the token after an error', async () => {
    verifyEmailMock.mockRejectedValueOnce(new TypeError('Failed to fetch'));
    verifyEmailMock.mockResolvedValueOnce({ status: 'verified' });
    renderStrict('tok-3');

    await waitFor(() => expect(screen.getByTestId('state')).toHaveTextContent('error'));

    act(() => retryRef?.());

    await waitFor(() => expect(screen.getByTestId('state')).toHaveTextContent('verified'));
    expect(verifyEmailMock).toHaveBeenCalledTimes(2);
    expect(verifyEmailMock).toHaveBeenLastCalledWith('tok-3');
  });
});
