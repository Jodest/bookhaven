// bcrypt hashes at most 72 bytes of input. The limit is in UTF-8 bytes, not
// characters (a Cyrillic letter takes 2 bytes). HTML maxLength counts UTF-16
// code units, so it must not be used for this rule.
export const PASSWORD_MAX_BYTES = 72;

export const PASSWORD_MAX_MESSAGE = 'Пароль слишком длинный: до 72 латинских или 36 русских букв';

export function passwordByteLength(value: string): number {
  return new TextEncoder().encode(value).length;
}

export function isPasswordWithinByteLimit(value: string): boolean {
  return passwordByteLength(value) <= PASSWORD_MAX_BYTES;
}
