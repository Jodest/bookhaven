import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { AuthorNameLink } from './author-name-link';

describe('AuthorNameLink', () => {
  it('links a published author to the author page', () => {
    render(<AuthorNameLink author={{ name: 'Иван Тургенев', slug: 'ivan-turgenev' }} linkClassName="hover:underline" />);
    const link = screen.getByRole('link', { name: 'Иван Тургенев' });
    expect(link).toHaveAttribute('href', '/authors/ivan-turgenev');
    expect(link).toHaveClass('hover:underline');
  });

  it('renders a non-published author (slug null) as plain text', () => {
    render(<AuthorNameLink author={{ name: 'Неизвестный автор', slug: null }} className="text-lg" linkClassName="hover:underline" />);
    expect(screen.queryByRole('link')).not.toBeInTheDocument();
    const text = screen.getByText('Неизвестный автор');
    expect(text).toHaveClass('text-lg');
    expect(text).not.toHaveClass('hover:underline');
  });
});
