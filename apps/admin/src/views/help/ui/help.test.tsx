import { render, screen } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { HelpChapter } from './help-chapter';
import { HelpIndex } from './help-index';
import { HelpLayout } from './help-layout';

let role = 'admin';
let pathnameValue = '/help';

vi.mock('next/navigation', () => ({
  usePathname: () => pathnameValue,
}));

vi.mock('@/entities/session', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/entities/session')>()),
  useAuthStore: () => ({ user: { id: 'u1', email: 'a@example.com', role } }),
}));

beforeEach(() => {
  role = 'admin';
  pathnameValue = '/help';
});

describe('HelpIndex', () => {
  it('lists nine chapters for an admin', () => {
    render(<HelpIndex />);
    expect(screen.getByRole('heading', { name: 'Справка' })).toBeInTheDocument();
    expect(screen.getAllByRole('link')).toHaveLength(9);
    expect(screen.getByRole('link', { name: 'Задачи импорта' })).toHaveAttribute('href', '/help/import-jobs');
  });

  it('lists only the review moderation chapter for a moderator', () => {
    role = 'moderator';
    render(<HelpIndex />);
    expect(screen.getAllByRole('link').map((l) => l.textContent)).toEqual(['Модерация отзывов']);
  });
});

describe('HelpLayout', () => {
  it('highlights the current chapter', () => {
    pathnameValue = '/help/import-jobs';
    render(
      <HelpLayout>
        <div>content</div>
      </HelpLayout>
    );
    const current = screen.getAllByRole('link').filter((l) => l.getAttribute('aria-current') === 'page');
    expect(current.map((l) => l.textContent)).toEqual(['Задачи импорта']);
    expect(screen.getByText('content')).toBeInTheDocument();
  });
});

describe('HelpChapter', () => {
  const md = [
    '# Заголовок',
    '',
    '## Раздел',
    '',
    'Текст с **жирным** и [главой](./duplicates.md) и [якорем](./duplicates.md#merge) и [сайтом](https://example.com/x).',
    '',
    '- один',
    '- два',
    '',
    '| A | B |',
    '| --- | --- |',
    '| 1 | 2 |',
    '',
    '<script>window.__pwned = 1</script>',
  ].join('\n');

  it('renders headings, list and table', () => {
    const { container } = render(<HelpChapter markdown={md} />);
    expect(screen.getByRole('heading', { level: 1, name: 'Заголовок' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { level: 2, name: 'Раздел' })).toBeInTheDocument();
    expect(screen.getAllByRole('listitem')).toHaveLength(2);
    expect(screen.getByRole('table')).toBeInTheDocument();
    expect(container.querySelector('strong')?.textContent).toBe('жирным');
  });

  it('rewrites chapter links and keeps the fragment', () => {
    render(<HelpChapter markdown={md} />);
    expect(screen.getByRole('link', { name: 'главой' })).toHaveAttribute('href', '/help/duplicates');
    expect(screen.getByRole('link', { name: 'якорем' })).toHaveAttribute('href', '/help/duplicates#merge');
  });

  it('opens external links in a new tab', () => {
    render(<HelpChapter markdown={md} />);
    const link = screen.getByRole('link', { name: 'сайтом' });
    expect(link).toHaveAttribute('target', '_blank');
    expect(link).toHaveAttribute('rel', 'noopener noreferrer');
  });

  it('does not create a script element from raw HTML', () => {
    const { container } = render(<HelpChapter markdown={md} />);
    expect(container.querySelector('script')).toBeNull();
  });
});
