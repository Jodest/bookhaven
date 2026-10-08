import { fireEvent, render, screen } from '@testing-library/react';
import type { AdminGenre, AdminGenreInput } from '@bookhaven/api-client';
import { describe, expect, it, vi } from 'vitest';
import { GenreForm } from './genre-form';

const genre: AdminGenre = {
  id: 'g1',
  name: 'роман',
  slug: 'roman',
  status: 'pending',
  wikidata_qid: 'Q8261',
};

function renderForm(
  editing: AdminGenre | null,
  formData: AdminGenreInput,
  onFieldChange = vi.fn()
) {
  render(
    <GenreForm
      formData={formData}
      onFieldChange={onFieldChange}
      editing={editing}
      onSubmit={vi.fn()}
      onCancel={vi.fn()}
      pending={false}
    />
  );
  return onFieldChange;
}

describe('GenreForm', () => {
  it('shows the status select on edit only', () => {
    renderForm(null, { name: '' });
    expect(screen.queryByLabelText('Статус')).not.toBeInTheDocument();
    expect(screen.getByLabelText('Wikidata QID')).toBeInTheDocument();
  });

  it('prefills status and QID on edit', () => {
    renderForm(genre, { name: 'роман', status: 'pending', wikidata_qid: 'Q8261' });
    expect((screen.getByLabelText('Статус') as HTMLSelectElement).value).toBe('pending');
    expect((screen.getByLabelText('Wikidata QID') as HTMLInputElement).value).toBe('Q8261');
  });

  it('reports QID edits, including clearing it to ""', () => {
    const onFieldChange = renderForm(genre, { name: 'роман', wikidata_qid: 'Q8261' });
    fireEvent.change(screen.getByLabelText('Wikidata QID'), { target: { value: '' } });
    expect(onFieldChange).toHaveBeenCalledWith({ wikidata_qid: '' });
  });

  it('hints at a malformed QID without blocking submit', () => {
    renderForm(null, { name: 'роман', wikidata_qid: 'abc' });
    expect(screen.getByText('QID должен иметь вид Q123')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Создать' })).not.toBeDisabled();
  });

  it('accepts a lowercase QID as valid (the server uppercases it)', () => {
    renderForm(null, { name: 'роман', wikidata_qid: ' q8261 ' });
    expect(screen.queryByText('QID должен иметь вид Q123')).not.toBeInTheDocument();
  });
});
