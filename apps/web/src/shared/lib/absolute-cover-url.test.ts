import { afterEach, describe, expect, it, vi } from 'vitest';
import { absoluteCoverUrl } from './absolute-cover-url';

describe('absoluteCoverUrl', () => {
  afterEach(() => {
    vi.unstubAllEnvs();
  });

  it('keeps an absolute Open Library cover unchanged', () => {
    const url = 'https://covers.openlibrary.org/b/id/1-L.jpg';
    expect(absoluteCoverUrl(url)).toBe(url);
  });

  it('prefixes a self-hosted cover with SITE_URL', () => {
    vi.stubEnv('SITE_URL', 'https://bookhaven.example');
    expect(absoluteCoverUrl('/covers/ab/abc.jpg')).toBe('https://bookhaven.example/covers/ab/abc.jpg');
  });

  it('drops a trailing slash of SITE_URL', () => {
    vi.stubEnv('SITE_URL', 'https://bookhaven.example/');
    expect(absoluteCoverUrl('/covers/ab/abc.jpg')).toBe('https://bookhaven.example/covers/ab/abc.jpg');
  });

  it('defaults to localhost when SITE_URL is unset', () => {
    vi.stubEnv('SITE_URL', '');
    expect(absoluteCoverUrl('/covers/ab/abc.jpg')).toBe('http://localhost:3000/covers/ab/abc.jpg');
  });

  it('gives null for no cover or an unknown shape', () => {
    expect(absoluteCoverUrl(null)).toBeNull();
    expect(absoluteCoverUrl('')).toBeNull();
    expect(absoluteCoverUrl('/other/x.jpg')).toBeNull();
    expect(absoluteCoverUrl('ftp://x/y.jpg')).toBeNull();
  });
});
