import { describe, expect, it } from 'vitest';
import { isHotlinkCoverUrl } from './hotlink-cover';

describe('isHotlinkCoverUrl', () => {
  it('accepts https FantLab links and subdomains', () => {
    expect(isHotlinkCoverUrl('https://fantlab.ru/images/editions/big/16469')).toBe(true);
    expect(isHotlinkCoverUrl('https://data.fantlab.ru/images/x.jpg')).toBe(true);
  });

  it('rejects other hosts, look-alikes, http and invalid values', () => {
    expect(isHotlinkCoverUrl('https://covers.openlibrary.org/b/id/1-L.jpg')).toBe(false);
    expect(isHotlinkCoverUrl('https://notfantlab.ru/x.jpg')).toBe(false);
    expect(isHotlinkCoverUrl('https://fantlab.ru.evil.example/x.jpg')).toBe(false);
    expect(isHotlinkCoverUrl('http://fantlab.ru/x.jpg')).toBe(false);
    expect(isHotlinkCoverUrl('/covers/ab/x.jpg')).toBe(false);
    expect(isHotlinkCoverUrl(null)).toBe(false);
  });
});
