import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { NavBadge } from './nav-badge';

describe('NavBadge', () => {
  it.each([0, undefined])('renders nothing for %s', (count) => {
    const { container } = render(<NavBadge count={count} srLabel="ожидают проверки" />);
    expect(container).toBeEmptyDOMElement();
  });

  it('renders the count visually (aria-hidden) plus sr-only text', () => {
    const { container } = render(
      <a href="/x">
        Модерация
        <NavBadge count={7} srLabel="ожидают проверки" />
      </a>
    );
    const visual = container.querySelector('[aria-hidden="true"]');
    expect(visual).toHaveTextContent('7');
    expect(screen.getByText(', ожидают проверки: 7')).toHaveClass('sr-only');
    expect(screen.getByRole('link')).toHaveAccessibleName('Модерация, ожидают проверки: 7');
  });

  it('caps the visual value at 99+ while sr text keeps the exact count', () => {
    const { container } = render(<NavBadge count={150} srLabel="задач в работе" />);
    expect(container.querySelector('[aria-hidden="true"]')).toHaveTextContent('99+');
    expect(screen.getByText(', задач в работе: 150')).toHaveClass('sr-only');
  });
});
