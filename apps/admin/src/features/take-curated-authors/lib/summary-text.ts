import type { TakeAuthorResult, TakeHarvestStep, TakeSummary } from '../model/use-take-curated-authors';

/** confirm() text before a bulk take. */
export function takeConfirmText(count: number, withBooks: boolean, autoHarvest: boolean): string {
  let text = `Импортировать выбранных авторов (${count})?`;
  if (withBooks) text += ' Затем будет запущен один харвест их книг.';
  if (autoHarvest) text += ' Для них будет включено автопополнение книг.';
  return `${text} Авторы и книги появятся скрытыми в очереди на проверку.`;
}

/** One line of the per-author report. */
export function takeResultLine(r: TakeAuthorResult): string {
  const name = r.author?.name ?? r.label;
  if (r.error) return `«${r.label}»: ошибка импорта — ${r.error}`;
  if (r.outcome === 'rejected') return `«${name}»: отклонён ранее, пропущен`;
  let line = r.outcome === 'created' ? `«${name}»: импортирован, ждёт модерации` : `«${name}»: уже в каталоге`;
  switch (r.autoHarvest.state) {
    case 'set':
      line += ', автопополнение включено';
      break;
    case 'already':
      line += ', автопополнение уже включено';
      break;
    case 'failed':
      line += `, автопополнение не включено: ${r.autoHarvest.error}`;
      break;
  }
  return line;
}

/** The harvest step of a bulk take, or null when it was not requested. */
export function takeHarvestLine(h: TakeHarvestStep): string | null {
  switch (h.state) {
    case 'started':
      return `Харвест книг запущен для авторов: ${h.count}. Книги появятся скрытыми в очереди на проверку.`;
    case 'skipped':
      return 'Харвест не запущен: нет импортированных авторов.';
    case 'failed':
      return `Харвест не запущен: ${h.error}`;
    default:
      return null;
  }
}

/** True when any step of the take failed or was rejected. */
export function takeHasProblems(s: TakeSummary): boolean {
  return (
    s.harvest.state === 'failed' ||
    s.results.some((r) => r.error || r.outcome === 'rejected' || r.autoHarvest.state === 'failed')
  );
}
