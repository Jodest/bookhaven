import { fireEvent, render, screen } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { BookTabs, type BookTabDef } from './book-tabs';

vi.mock('next/navigation', () => ({
  useSearchParams: () => new URLSearchParams(window.location.search),
}));

const tabs: BookTabDef[] = [
  { id: 'adaptations', label: 'Экранизации (2)', panel: <p>adaptations content</p> },
  { id: 'reviews', label: 'Рецензии', panel: <p>reviews content</p> },
  { id: 'similar', label: 'Похожие книги', panel: <p>similar content</p> },
];

beforeEach(() => {
  window.history.replaceState(null, '', '/books/x');
});

describe('BookTabs', () => {
  it('exposes a tab list with the selected state, a single tab stop and labelled panels', () => {
    render(<BookTabs tabs={tabs} initialTab="adaptations" />);

    expect(screen.getByRole('tablist')).toBeInTheDocument();
    const [first, second, third] = screen.getAllByRole('tab');
    expect(first).toHaveAttribute('aria-selected', 'true');
    expect(first).toHaveAttribute('tabindex', '0');
    expect(second).toHaveAttribute('aria-selected', 'false');
    expect(second).toHaveAttribute('tabindex', '-1');
    expect(third).toHaveAttribute('tabindex', '-1');
    expect(first).toHaveAttribute('aria-controls', 'book-tabpanel-adaptations');
    const panel = screen.getByRole('tabpanel', { name: 'Экранизации (2)' });
    expect(panel).toHaveAttribute('id', 'book-tabpanel-adaptations');
    // Inactive panels stay in the DOM, hidden.
    expect(document.getElementById('book-tabpanel-reviews')).toHaveAttribute('hidden');
    expect(document.getElementById('book-tabpanel-reviews')).toHaveTextContent('reviews content');
  });

  it('switches the panel and puts the tab into the address without navigation or a history entry', () => {
    const replaceState = vi.spyOn(window.history, 'replaceState');
    const pushState = vi.spyOn(window.history, 'pushState');
    const lengthBefore = window.history.length;
    render(<BookTabs tabs={tabs} initialTab="adaptations" />);

    fireEvent.click(screen.getByRole('tab', { name: 'Похожие книги' }));

    expect(screen.getByRole('tab', { name: 'Похожие книги' })).toHaveAttribute('aria-selected', 'true');
    expect(screen.getByRole('tabpanel', { name: 'Похожие книги' })).toBeVisible();
    expect(document.getElementById('book-tabpanel-adaptations')).toHaveAttribute('hidden');
    expect(window.location.search).toBe('?tab=similar');
    expect(replaceState).toHaveBeenCalled();
    expect(pushState).not.toHaveBeenCalled();
    expect(window.history.length).toBe(lengthBefore);
    replaceState.mockRestore();
    pushState.mockRestore();
  });

  it('moves with the arrow keys, wrapping, and with Home and End', () => {
    render(<BookTabs tabs={tabs} initialTab="adaptations" />);
    const [first, second, third] = screen.getAllByRole('tab');
    first.focus();

    fireEvent.keyDown(first, { key: 'ArrowRight' });
    expect(second).toHaveFocus();
    expect(second).toHaveAttribute('aria-selected', 'true');

    fireEvent.keyDown(second, { key: 'ArrowLeft' });
    fireEvent.keyDown(first, { key: 'ArrowLeft' });
    expect(third).toHaveFocus();
    expect(third).toHaveAttribute('aria-selected', 'true');

    fireEvent.keyDown(third, { key: 'ArrowRight' });
    expect(first).toHaveFocus();

    fireEvent.keyDown(first, { key: 'End' });
    expect(third).toHaveFocus();
    fireEvent.keyDown(third, { key: 'Home' });
    expect(first).toHaveFocus();
    expect(first).toHaveAttribute('aria-selected', 'true');
  });

  it('selects the tab from ?tab= and ignores a tab that does not exist', () => {
    window.history.replaceState(null, '', '/books/x?tab=similar');
    const { unmount } = render(<BookTabs tabs={tabs} initialTab="adaptations" />);
    expect(screen.getByRole('tab', { name: 'Похожие книги' })).toHaveAttribute('aria-selected', 'true');
    unmount();

    window.history.replaceState(null, '', '/books/x?tab=adaptations');
    render(<BookTabs tabs={tabs.slice(1)} initialTab="reviews" />);
    expect(screen.getByRole('tab', { name: 'Рецензии' })).toHaveAttribute('aria-selected', 'true');
  });

  it('selects the reviews for an old #reviews link', () => {
    window.HTMLElement.prototype.scrollIntoView = vi.fn();
    window.history.replaceState(null, '', '/books/x#reviews');
    render(<BookTabs tabs={tabs} initialTab="adaptations" />);

    expect(screen.getByRole('tab', { name: 'Рецензии' })).toHaveAttribute('aria-selected', 'true');
  });
});
