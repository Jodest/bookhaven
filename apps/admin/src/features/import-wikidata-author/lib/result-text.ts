import { ApiError } from '@bookhaven/api-client';
import { AUTHOR_STATUS_LABELS, authorModerationHref, importEnrichmentSuffix } from '@/entities/author';
import type { WikidataImportOutcome } from '../model/use-import-wikidata-author';

export interface ImportFeedbackLink {
  href: string;
  label: string;
}

export interface ImportFeedback {
  type: 'success' | 'error';
  message: string;
  links: ImportFeedbackLink[];
}

const HARVEST_LINK: ImportFeedbackLink = { href: '/import/jobs', label: 'К задачам харвеста' };

/** confirm() text before "Импортировать с книгами" / "Импортировать книги". */
export function harvestConfirmText(name: string): string {
  return `Запустить харвест книг автора «${name}»? Книги появятся скрытыми в очереди на проверку.`;
}

/** Banner for a successful import call (any outcome) plus the optional harvest step. */
export function importResultFeedback({ result, withBooks, harvest }: WikidataImportOutcome): ImportFeedback {
  const { author } = result;
  const open: ImportFeedbackLink = { href: authorModerationHref(author), label: 'Открыть' };

  if (result.outcome === 'rejected') {
    let message = `Автор «${author.name}» был отклонён ранее и повторно не создаётся.`;
    if (withBooks) message += ' Книги не импортированы.';
    return { type: 'error', message, links: [open] };
  }

  if (harvest.state === 'failed') {
    return {
      type: 'error',
      message: `Автор импортирован, но харвест не запущен: ${harvest.error}. Нажмите «Импортировать книги» ещё раз.`,
      links: [open],
    };
  }

  let message: string;
  const links: ImportFeedbackLink[] = [];
  if (result.outcome === 'created') {
    message = `Автор «${author.name}» импортирован и ждёт модерации.`;
    if (result.namesake_of) {
      message += ` В каталоге уже есть тёзка «${result.namesake_of.name}» с другим QID — создан отдельный автор.`;
    }
    message += importEnrichmentSuffix(result.enrichment ?? null);
  } else if (result.match === 'slug' && result.qid_backfilled) {
    message = `Найден автор «${author.name}» без QID — QID ${author.external_id ?? ''} привязан к нему.`;
    links.push(open);
  } else {
    const label = (AUTHOR_STATUS_LABELS[author.status] ?? author.status).toLowerCase();
    message = `Автор «${author.name}» уже есть в каталоге (${label}).`;
    links.push(open);
  }

  if (harvest.state === 'started') {
    message += ' Харвест книг запущен — книги появятся скрытыми в очереди на проверку.';
    links.push(HARVEST_LINK);
  }
  return { type: 'success', message, links };
}

/** Banner text for a failed import call. */
export function importErrorMessage(err: unknown): string {
  if (err instanceof ApiError) {
    if (err.code === 'no_ru_label') {
      return 'У элемента Wikidata нет русского названия — импорт невозможен. Создайте автора вручную.';
    }
    if (err.code === 'not_a_person') {
      return 'Элемент Wikidata не является человеком или псевдонимом — импорт отменён.';
    }
    if (err.status === 502) {
      return 'Wikidata недоступна, попробуйте позже.';
    }
    if (err.message) return err.message;
  }
  return 'Не удалось импортировать автора.';
}
