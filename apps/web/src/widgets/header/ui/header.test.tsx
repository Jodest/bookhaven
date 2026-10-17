import { render, screen } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { Header } from './header';

let authValue: { user: { name: string; email: string } | null; isAuthenticated: boolean; isLoading: boolean };

vi.mock('@/entities/session', () => ({
  useAuthStore: () => ({ ...authValue, logout: vi.fn() }),
}));

vi.mock('next/navigation', () => ({
  useRouter: () => ({ push: vi.fn() }),
  useSearchParams: () => new URLSearchParams(),
  usePathname: () => '/',
}));

vi.mock('./header-search', () => ({ HeaderSearch: () => null }));
vi.mock('./header-login-link', () => ({ HeaderLoginLink: () => null }));

describe('Header', () => {
  beforeEach(() => {
    authValue = { user: null, isAuthenticated: false, isLoading: false };
  });

  it('shows the Подборки link to anonymous visitors', () => {
    render(<Header />);
    expect(screen.getByRole('link', { name: 'Подборки' })).toHaveAttribute('href', '/top');
    expect(screen.getByRole('link', { name: 'Каталог' })).toHaveAttribute('href', '/books');
  });

  it('shows it while the session is still loading', () => {
    authValue = { user: null, isAuthenticated: false, isLoading: true };
    render(<Header />);
    expect(screen.getByRole('link', { name: 'Подборки' })).toHaveAttribute('href', '/top');
  });

  it('shows it to signed-in users too', () => {
    authValue = { user: { name: 'Аня', email: 'a@b.c' }, isAuthenticated: true, isLoading: false };
    render(<Header />);
    expect(screen.getByRole('link', { name: 'Подборки' })).toHaveAttribute('href', '/top');
    expect(screen.queryByRole('link', { name: 'Мои списки' })).not.toBeInTheDocument();
  });
});
