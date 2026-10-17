import { describe, expect, it } from 'vitest';
import { languageNameRu } from './language-name';

describe('languageNameRu', () => {
  it('names known languages in Russian', () => {
    expect(languageNameRu('ru')).toBe('Русский');
    expect(languageNameRu('en')).toBe('Английский');
  });

  it('returns an unknown code as is', () => {
    expect(languageNameRu('!!')).toBe('!!');
  });

  it('returns null for an empty code', () => {
    expect(languageNameRu('')).toBeNull();
    expect(languageNameRu(undefined)).toBeNull();
  });
});
