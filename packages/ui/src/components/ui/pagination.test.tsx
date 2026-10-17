import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { Pagination } from './pagination';

const href = (p: number) => `/books?page=${p}`;

describe('Pagination', () => {
  it('renders nothing when totalPages <= 1', () => {
    const { container } = render(<Pagination page={1} totalPages={1} onPageChange={vi.fn()} />);
    expect(container).toBeEmptyDOMElement();
  });

  it('link mode renders anchors and a non-link current page', () => {
    render(<Pagination page={5} totalPages={42} getHref={href} />);
    expect(screen.getByRole('navigation', { name: 'Пагинация' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Страница 4' })).toHaveAttribute('href', '/books?page=4');
    expect(screen.getByRole('link', { name: 'Страница 42' })).toHaveAttribute('href', '/books?page=42');
    const current = screen.getByLabelText('Страница 5', { selector: '[aria-current="page"]' });
    expect(current.tagName).not.toBe('A');
    expect(screen.getByRole('link', { name: 'Предыдущая страница' })).toHaveAttribute('href', '/books?page=4');
    expect(screen.getByRole('link', { name: 'Следующая страница' })).toHaveAttribute('href', '/books?page=6');
  });

  it('button mode calls onPageChange', () => {
    const onPageChange = vi.fn();
    render(<Pagination page={3} totalPages={5} onPageChange={onPageChange} />);
    fireEvent.click(screen.getByRole('button', { name: 'Страница 5' }));
    expect(onPageChange).toHaveBeenLastCalledWith(5);
    fireEvent.click(screen.getByRole('button', { name: 'Предыдущая страница' }));
    expect(onPageChange).toHaveBeenLastCalledWith(2);
    fireEvent.click(screen.getByRole('button', { name: 'Следующая страница' }));
    expect(onPageChange).toHaveBeenLastCalledWith(4);
  });

  it('disables prev/next at the edges in button mode', () => {
    const { rerender } = render(<Pagination page={1} totalPages={5} onPageChange={vi.fn()} />);
    expect(screen.getByRole('button', { name: 'Предыдущая страница' })).toBeDisabled();
    expect(screen.getByRole('button', { name: 'Следующая страница' })).not.toBeDisabled();
    rerender(<Pagination page={5} totalPages={5} onPageChange={vi.fn()} />);
    expect(screen.getByRole('button', { name: 'Следующая страница' })).toBeDisabled();
  });

  it('disables prev/next at the edges in link mode', () => {
    render(<Pagination page={1} totalPages={5} getHref={href} />);
    const prev = screen.getByLabelText('Предыдущая страница');
    expect(prev).toHaveAttribute('aria-disabled', 'true');
    expect(prev).not.toHaveAttribute('href');
    expect(screen.getByLabelText('Следующая страница')).not.toHaveAttribute('aria-disabled');
  });

  it('honours prevPage / nextPage overrides', () => {
    render(<Pagination page={5} prevPage={2} nextPage={7} totalPages={9} getHref={href} />);
    expect(screen.getByRole('link', { name: 'Предыдущая страница' })).toHaveAttribute('href', '/books?page=2');
    expect(screen.getByRole('link', { name: 'Следующая страница' })).toHaveAttribute('href', '/books?page=7');
  });

  it('disables a control whose target is out of range', () => {
    render(<Pagination page={5} nextPage={6} prevPage={0} totalPages={5} getHref={href} />);
    expect(screen.getByLabelText('Предыдущая страница')).toHaveAttribute('aria-disabled', 'true');
    expect(screen.getByLabelText('Следующая страница')).toHaveAttribute('aria-disabled', 'true');
  });

  it('renders compact text and hides ellipses from assistive tech', () => {
    render(<Pagination page={5} totalPages={42} getHref={href} />);
    expect(screen.getByText('5 / 42')).toBeInTheDocument();
    expect(screen.getByLabelText('Страница 5 из 42')).toBeInTheDocument();
    document.querySelectorAll('li[aria-hidden="true"]').forEach((el) => expect(el).toHaveTextContent('…'));
    expect(document.querySelectorAll('li[aria-hidden="true"]')).toHaveLength(2);
  });

  it('uses renderLink when provided and calls onPageChange from the link click', () => {
    const onPageChange = vi.fn();
    render(
      <Pagination
        page={2}
        totalPages={5}
        getHref={href}
        onPageChange={onPageChange}
        renderLink={({ href: h, ...rest }) => <a data-custom="1" href={h} {...rest} />}
      />
    );
    const link = screen.getByRole('link', { name: 'Страница 3' });
    expect(link).toHaveAttribute('data-custom', '1');
    fireEvent.click(link);
    expect(onPageChange).toHaveBeenCalledWith(3);
  });
});
