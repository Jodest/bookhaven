/** Russian display name of a language code ("ru" -> "Русский"); null for an empty code. */
export function languageNameRu(code: string | null | undefined): string | null {
  const trimmed = code?.trim();
  if (!trimmed) return null;
  try {
    const name = new Intl.DisplayNames(['ru'], { type: 'language' }).of(trimmed);
    if (!name) return trimmed;
    return name.charAt(0).toUpperCase() + name.slice(1);
  } catch {
    return trimmed;
  }
}
