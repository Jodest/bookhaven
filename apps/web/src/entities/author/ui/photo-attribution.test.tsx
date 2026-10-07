import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { PhotoAttribution } from './photo-attribution';

const sourceUrl = 'https://commons.wikimedia.org/wiki/File:Montbolo_001.jpg';

describe('PhotoAttribution', () => {
  it('renders author, linked licence and the Commons link', () => {
    const { container } = render(
      <PhotoAttribution
        attribution={{
          author: 'Grondin',
          license: 'CC BY-SA 3.0',
          licenseUrl: 'https://creativecommons.org/licenses/by-sa/3.0',
          sourceUrl,
        }}
      />,
    );
    expect(container.textContent).toBe('Фото: Grondin, CC BY-SA 3.0, Wikimedia Commons');
    const license = screen.getByRole('link', { name: 'CC BY-SA 3.0' });
    expect(license).toHaveAttribute('href', 'https://creativecommons.org/licenses/by-sa/3.0');
    expect(license).toHaveAttribute('target', '_blank');
    expect(license).toHaveAttribute('rel', 'noopener noreferrer');
    const commons = screen.getByRole('link', { name: 'Wikimedia Commons' });
    expect(commons).toHaveAttribute('href', sourceUrl);
    expect(commons).toHaveAttribute('rel', 'noopener noreferrer');
  });

  it('renders the licence as plain text without a URL and omits a missing author', () => {
    const { container } = render(
      <PhotoAttribution attribution={{ author: null, license: 'Public domain', licenseUrl: null, sourceUrl }} />,
    );
    expect(container.textContent).toBe('Фото: Public domain, Wikimedia Commons');
    expect(screen.getAllByRole('link')).toHaveLength(1);
  });
});
