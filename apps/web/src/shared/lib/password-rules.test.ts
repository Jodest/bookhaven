import { describe, expect, it } from 'vitest';
import { isPasswordWithinByteLimit, passwordByteLength } from './password-rules';

describe('password-rules', () => {
  it('counts UTF-8 bytes', () => {
    expect(passwordByteLength('a'.repeat(5))).toBe(5);
    expect(passwordByteLength('я'.repeat(5))).toBe(10);
  });

  it('accepts 72 ASCII bytes and rejects 73', () => {
    expect(isPasswordWithinByteLimit('a'.repeat(72))).toBe(true);
    expect(isPasswordWithinByteLimit('a'.repeat(73))).toBe(false);
  });

  it('accepts 36 Cyrillic letters and rejects 37', () => {
    expect(isPasswordWithinByteLimit('я'.repeat(36))).toBe(true);
    expect(isPasswordWithinByteLimit('я'.repeat(37))).toBe(false);
  });
});
