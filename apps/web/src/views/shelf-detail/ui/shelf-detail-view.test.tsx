import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import type { ImgHTMLAttributes } from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { Shelf } from '@bookhaven/api-client';
import { ShelfDetailView } from './shelf-detail-view';

const getMock = vi.fn();
const updateMock = vi.fn();

vi.mock('@/entities/shelf', () => ({
  shelvesApi: {
    get: (...args: unknown[]) => getMock(...args),
    update: (...args: unknown[]) => updateMock(...args),
    remove: vi.fn(),
    removeBook: vi.fn(),
  },
}));

vi.mock('next/navigation', () => ({ useRouter: () => ({ push: vi.fn() }) }));

vi.mock('next/image', () => ({
  default: ({ fill: _fill, ...props }: ImgHTMLAttributes<HTMLImageElement> & { fill?: boolean }) => (
    // eslint-disable-next-line @next/next/no-img-element, jsx-a11y/alt-text
    <img {...props} />
  ),
}));

function shelf(overrides: Partial<Shelf>): Shelf {
  return {
    id: 's1',
    userId: 'u1',
    name: 'Отпуск',
    type: 'custom',
    description: 'Для пляжа',
    isPublic: true,
    booksCount: 0,
    createdAt: '2024-01-01T00:00:00Z',
    updatedAt: '2024-01-01T00:00:00Z',
    ...overrides,
  };
}

function respond(s: Shelf) {
  getMock.mockResolvedValue({ shelf: s, items: [], total: 0, page: 1, limit: 12 });
}

describe('ShelfDetailView', () => {
  beforeEach(() => {
    getMock.mockReset();
    updateMock.mockReset().mockResolvedValue(undefined);
  });

  it('shows the description and edits name and description together', async () => {
    respond(shelf({}));
    render(<ShelfDetailView id="s1" />);

    expect(await screen.findByText('Для пляжа')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Редактировать' }));

    const description = screen.getByLabelText('Описание');
    fireEvent.change(description, { target: { value: 'Новое описание' } });
    fireEvent.click(screen.getByRole('button', { name: 'Сохранить' }));

    await waitFor(() =>
      expect(updateMock).toHaveBeenCalledWith('s1', {
        name: 'Отпуск',
        description: 'Новое описание',
      })
    );
  });

  it('toggles privacy and shows the badge on a private shelf', async () => {
    respond(shelf({ isPublic: false }));
    render(<ShelfDetailView id="s1" />);

    expect(await screen.findByText('Приватная')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Сделать публичной' }));

    await waitFor(() => expect(updateMock).toHaveBeenCalledWith('s1', { is_public: true }));
  });

  it('shows no controls on the default «Прочитано» shelf', async () => {
    respond(shelf({ name: 'Прочитано', type: 'read', description: '' }));
    render(<ShelfDetailView id="s1" />);

    expect(await screen.findByText('Прочитано')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Редактировать' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /Сделать/ })).not.toBeInTheDocument();
    expect(screen.queryByText('Приватная')).not.toBeInTheDocument();
  });
});
