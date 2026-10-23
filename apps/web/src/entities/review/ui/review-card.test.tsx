import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { ReviewCard } from './review-card';

const base = {
  id: 'r1',
  book_id: 'b1',
  user_id: 'u1',
  user: { id: 'u1', name: 'Аня', avatar_url: null },
  rating: 4,
  content: 'Текст рецензии',
  likes_count: 2,
  liked_by_me: false,
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
};

function setup(review: typeof base & { is_hidden?: boolean }, isOwner = true) {
  const onDelete = vi.fn();
  render(
    <ReviewCard
      review={review}
      isOwner={isOwner}
      isAuthenticated
      onToggleLike={vi.fn()}
      onEdit={vi.fn()}
      onDelete={onDelete}
    />
  );
  return { onDelete };
}

describe('ReviewCard', () => {
  it('hidden review shows the badge and note, only delete for the owner', () => {
    const { onDelete } = setup({ ...base, is_hidden: true });

    expect(screen.getByText('Скрыт модератором')).toBeInTheDocument();
    expect(
      screen.getByText(
        'Модератор скрыл эту рецензию — другие читатели её не видят. Вы можете удалить её и написать новую.'
      )
    ).toBeInTheDocument();
    expect(screen.queryByText('Редактировать')).toBeNull();
    expect(screen.queryByText('2')).toBeNull();

    fireEvent.click(screen.getByText('Удалить'));
    expect(onDelete).toHaveBeenCalled();
  });

  it('visible review keeps like, edit and delete', () => {
    setup({ ...base, is_hidden: false });

    expect(screen.queryByText('Скрыт модератором')).toBeNull();
    expect(screen.getByText('2')).toBeInTheDocument();
    expect(screen.getByText('Редактировать')).toBeInTheDocument();
    expect(screen.getByText('Удалить')).toBeInTheDocument();
  });
});
