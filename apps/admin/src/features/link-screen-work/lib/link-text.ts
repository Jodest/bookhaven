// Messages of the add film dialog (openspec screen-works/admin-editing).

/**
 * `qidMatched` is true when a new work was submitted but its QID matched a
 * stored work, so that one was linked instead.
 */
export function linkSuccessText(type: 'book' | 'author', title: string, qidMatched: boolean): string {
  const base = type === 'book' ? `Экранизация «${title}» добавлена.` : `Фильм «${title}» добавлен.`;
  return qidMatched ? `${base} Фильм с этим QID уже был в каталоге — привязан он.` : base;
}

export function linkFallbackText(type: 'book' | 'author'): string {
  return type === 'book' ? 'Не удалось добавить экранизацию.' : 'Не удалось добавить фильм.';
}
