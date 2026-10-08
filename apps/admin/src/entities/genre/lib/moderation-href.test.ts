import { describe, expect, it } from 'vitest';
import { genreModerationHref } from './moderation-href';

describe('genreModerationHref', () => {
  it('links to the status tab with a name search', () => {
    const href = genreModerationHref({ name: 'научная фантастика', status: 'rejected' });
    const url = new URL(href, 'http://admin.local');
    expect(url.pathname).toBe('/moderation/genres');
    expect(url.searchParams.get('status')).toBe('rejected');
    expect(url.searchParams.get('search')).toBe('научная фантастика');
  });
});
