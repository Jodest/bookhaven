import type { CuratedCollection } from '@bookhaven/api-client';
import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { CuratedCollectionsRow } from './curated-collections-row';

function col(n: number): CuratedCollection {
  return {
    id: `c${n}`,
    slug: `s${n}`,
    title: `Подборка ${n}`,
    description: '',
    kind: 'manual',
    size: 10,
    shown_count: 5,
    cover_urls: [],
    source: null,
    source_url: null,
    genre: null,
    rule: null,
    computed_at: null,
  } as unknown as CuratedCollection;
}

describe('CuratedCollectionsRow', () => {
  it('renders cards with h3 titles and the all-collections link', () => {
    render(<CuratedCollectionsRow collections={[col(1), col(2), col(3)]} />);
    expect(screen.getByRole('heading', { level: 2, name: 'Подборки' })).toBeInTheDocument();
    expect(screen.getAllByRole('heading', { level: 3 })).toHaveLength(3);
    expect(screen.getByRole('link', { name: 'Все подборки' })).toHaveAttribute('href', '/top');
  });

  it('renders nothing when empty', () => {
    const { container } = render(<CuratedCollectionsRow collections={[]} />);
    expect(container).toBeEmptyDOMElement();
  });
});
