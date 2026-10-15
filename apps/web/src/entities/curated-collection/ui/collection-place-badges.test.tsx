import type { BookCollectionPlace } from '@bookhaven/api-client';
import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { CollectionPlaceBadges } from './collection-place-badges';

function place(n: number, title = `Подборка ${n}`, slug = `s${n}`): BookCollectionPlace {
  return { place: n, collection: { id: `c${n}`, slug, title, kind: 'manual', size: 100 } };
}

describe('CollectionPlaceBadges', () => {
  it('renders the badge text and href', () => {
    render(<CollectionPlaceBadges places={[place(3, 'Топ-100 книг всех времён', 'top-100')]} />);
    const link = screen.getByRole('link', { name: '№3 в „Топ-100 книг всех времён“' });
    expect(link).toHaveAttribute('href', '/top/top-100');
  });

  it('links to the page that holds the place', () => {
    render(<CollectionPlaceBadges places={[place(73, 'Большая', 'big')]} />);
    expect(screen.getByRole('link')).toHaveAttribute('href', '/top/big?page=2');
  });

  it('shows 3 badges and expands the rest', () => {
    render(<CollectionPlaceBadges places={[1, 2, 3, 4, 5].map((n) => place(n))} />);
    expect(screen.getAllByRole('link')).toHaveLength(3);
    fireEvent.click(screen.getByRole('button', { name: 'Ещё 2' }));
    expect(screen.getAllByRole('link')).toHaveLength(5);
    expect(screen.queryByRole('button')).not.toBeInTheDocument();
  });

  it('renders nothing for an empty array', () => {
    const { container } = render(<CollectionPlaceBadges places={[]} />);
    expect(container).toBeEmptyDOMElement();
  });
});
