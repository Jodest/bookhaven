import { describe, expect, it } from 'vitest';
import { collectionPlaceHref } from './place-href';

describe('collectionPlaceHref', () => {
  it('links to the page that holds the place', () => {
    expect(collectionPlaceHref('s', 1)).toBe('/top/s');
    expect(collectionPlaceHref('s', 50)).toBe('/top/s');
    expect(collectionPlaceHref('s', 51)).toBe('/top/s?page=2');
    expect(collectionPlaceHref('s', 73)).toBe('/top/s?page=2');
  });
});
