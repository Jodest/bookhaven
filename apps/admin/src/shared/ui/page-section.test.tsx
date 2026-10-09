import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { PageSection } from './page-section';

describe('PageSection', () => {
  it('renders a region labelled by its h2 with description and children', () => {
    render(
      <PageSection title="Вручную" description="Описание секции">
        <p>Содержимое</p>
      </PageSection>
    );

    const region = screen.getByRole('region', { name: 'Вручную' });
    expect(screen.getByRole('heading', { level: 2, name: 'Вручную' })).toBeInTheDocument();
    expect(region).toHaveTextContent('Описание секции');
    expect(region).toHaveTextContent('Содержимое');
  });
});
