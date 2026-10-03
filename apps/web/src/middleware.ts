import { NextResponse } from 'next/server';
import type { NextRequest } from 'next/server';

// `/verify-email`, `/forgot-password` and `/reset-password` must stay public
// (in neither list) and must not redirect logged-in users: the emailed links
// have to work logged in and logged out, in any browser, and the forgot form
// is harmless for a logged-in user.
const protectedRoutes = ['/profile', '/shelves'];
const authRoutes = ['/login', '/register'];

export function middleware(request: NextRequest) {
  const { pathname, search } = request.nextUrl;
  const accessToken = request.cookies.get('access_token')?.value;
  const isAuthenticated = Boolean(accessToken);
  // Non-sensitive marker cookie set/cleared alongside refresh_token (same
  // 30-day lifetime). Lets a still-possibly-valid session through even after
  // the short-lived access_token has expired, so the client-side reactive
  // refresh (see auth-store.ts/session-initializer.tsx) gets a chance to run
  // instead of hard-redirecting at the Edge before React ever mounts.
  const hasSession = Boolean(request.cookies.get('has_session')?.value);

  if (protectedRoutes.some((route) => pathname.startsWith(route)) && !isAuthenticated && !hasSession) {
    const loginUrl = new URL('/login', request.url);
    loginUrl.searchParams.set('redirect', pathname + search);
    return NextResponse.redirect(loginUrl);
  }

  if (authRoutes.includes(pathname) && isAuthenticated) {
    return NextResponse.redirect(new URL('/profile', request.url));
  }

  return NextResponse.next();
}

export const config = {
  matcher: ['/((?!api|_next/static|_next/image|favicon.ico|.*\\.).*)'],
};
