import { render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { Footer } from './footer';

vi.mock('@/entities/genre', () => ({ genresApi: { list: () => Promise.resolve({ items: [] }) } }));
vi.mock('@/entities/session', () => ({ useAuthStore: () => ({ user: null, isAuthenticated: false }) }));

describe('Footer', () => {
  it('always shows the TMDB attribution notice with a link to TMDB', () => {
    render(<Footer />);

    expect(
      screen.getByText(/This product uses the TMDB API but is not endorsed or certified by TMDB\./),
    ).toBeInTheDocument();
    expect(screen.getByText(/Постеры фильмов — Wikimedia Commons и/)).toBeInTheDocument();
    const link = screen.getByRole('link', { name: 'TMDB' });
    expect(link).toHaveAttribute('href', 'https://www.themoviedb.org');
    expect(link).toHaveAttribute('target', '_blank');
  });
});
