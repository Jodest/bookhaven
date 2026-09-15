import { NextResponse } from 'next/server';
import type { NextRequest } from 'next/server';

export function middleware(request: NextRequest) {
  const { pathname } = request.nextUrl;
  const accessToken = request.cookies.get('access_token')?.value;
  // Non-sensitive marker cookie set/cleared alongside refresh_token (same
  // 30-day lifetime). Lets a still-possibly-valid session through even after
  // the short-lived access_token has expired, so the client-side reactive
  // refresh (see auth-store.ts) gets a chance to run instead of
  // hard-redirecting at the Edge before React ever mounts.
  const hasSession = Boolean(request.cookies.get('has_session')?.value);

  // Protected admin routes require an access token cookie, unless a
  // still-possibly-valid session marker is present.
  if (!accessToken && !hasSession) {
    const loginUrl = new URL('/login', request.url);
    loginUrl.searchParams.set('redirect', pathname);
    return NextResponse.redirect(loginUrl);
  }

  return NextResponse.next();
}

export const config = {
  matcher: ['/((?!api|_next/static|_next/image|favicon.ico|login|.*\\.).*)'],
};
