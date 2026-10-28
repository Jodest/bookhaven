import { render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { Footer } from './footer';

vi.mock('@/entities/genre', () => ({ genresApi: { list: () => Promise.resolve({ items: [] }) } }));
vi.mock('@/entities/session', () => ({ useAuthStore: () => ({ user: null, isAuthenticated: false }) }));

describe('Footer', () => {
  it('shows the TMDB logo link and the notice from the TMDB terms', () => {
    render(<Footer />);

    expect(
      screen.getByText(
        /BookHaven uses TMDB and the TMDB APIs but is not endorsed, certified, or otherwise\s+approved by TMDB\./,
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText(/This product uses the TMDB API/)).not.toBeInTheDocument();
    expect(screen.getByText('Постеры фильмов — Wikimedia Commons и TMDB.')).toBeInTheDocument();
    const link = screen.getByRole('link', { name: 'TMDB' });
    expect(link).toHaveAttribute('href', 'https://www.themoviedb.org');
    expect(link).toHaveAttribute('target', '_blank');
    expect(link).toHaveAttribute('rel', expect.stringContaining('noopener'));
    expect(link.querySelector('img')).not.toBeNull();
  });
});
