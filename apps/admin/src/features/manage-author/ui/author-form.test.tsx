import { render, screen } from '@testing-library/react';
import type { AdminAuthor, AdminAuthorInput } from '@bookhaven/api-client';
import { describe, expect, it, vi } from 'vitest';
import { AuthorForm } from './author-form';

const photo = 'https://upload.wikimedia.org/wikipedia/commons/thumb/a/ab/T.jpg/500px-T.jpg';

function editing(overrides: Partial<AdminAuthor> = {}): AdminAuthor {
  return {
    id: 'a1',
    name: 'Лев Толстой',
    slug: 'lev-tolstoy',
    biography: null,
    photo_url: photo,
    birth_date: null,
    death_date: null,
    status: 'published',
    external_source: 'wikidata',
    external_id: 'Q7243',
    photo_attribution: {
      author: 'Sergei Prokudin-Gorskii',
      license: 'Public domain',
      license_url: null,
      source_url: 'https://commons.wikimedia.org/wiki/File:T.jpg',
    },
    ...overrides,
  };
}

function renderForm(author: AdminAuthor | null, formData: AdminAuthorInput) {
  return render(
    <AuthorForm
      formData={formData}
      onFieldChange={vi.fn()}
      editing={author}
      onSubmit={vi.fn()}
      onCancel={vi.fn()}
      pending={false}
    />
  );
}

describe('AuthorForm photo attribution', () => {
  it('shows the read-only attribution with a link to the file page', () => {
    renderForm(editing(), { name: 'Лев Толстой', photo_url: photo });
    expect(screen.getByText(/Фото: Sergei Prokudin-Gorskii, Public domain, Wikimedia Commons/)).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Страница файла' })).toHaveAttribute(
      'href',
      'https://commons.wikimedia.org/wiki/File:T.jpg'
    );
    expect(screen.queryByText('При смене фото атрибуция будет удалена')).not.toBeInTheDocument();
  });

  it('warns that changing the photo drops the attribution', () => {
    renderForm(editing(), { name: 'Лев Толстой', photo_url: 'https://example.com/other.jpg' });
    expect(screen.getByText('При смене фото атрибуция будет удалена')).toBeInTheDocument();
  });

  it('shows nothing without an attribution', () => {
    renderForm(editing({ photo_attribution: null }), { name: 'Лев Толстой', photo_url: 'https://example.com/x.jpg' });
    expect(screen.queryByText(/Фото:/)).not.toBeInTheDocument();
    expect(screen.queryByText('При смене фото атрибуция будет удалена')).not.toBeInTheDocument();
  });
});

describe('AuthorForm heading', () => {
  it('renders no inner heading (the page or section provides it)', () => {
    renderForm(editing(), { name: 'Лев Толстой' });
    expect(screen.queryByRole('heading')).not.toBeInTheDocument();
    renderForm(null, { name: '' });
    expect(screen.queryByRole('heading')).not.toBeInTheDocument();
  });
});
