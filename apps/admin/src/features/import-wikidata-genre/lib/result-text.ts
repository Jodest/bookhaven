import { AdminGenreImportResult, ApiError } from '@bookhaven/api-client';
import { GENRE_STATUS_LABELS, genreModerationHref } from '@/entities/genre';

export interface GenreImportFeedbackLink {
  href: string;
  label: string;
}

export interface GenreImportFeedback {
  type: 'success' | 'error';
  message: string;
  links: GenreImportFeedbackLink[];
}

const MODERATION_LINK: GenreImportFeedbackLink = { href: '/moderation/genres', label: 'К модерации жанров' };

/**
 * Banner for a successful genre import call (any outcome), per
 * docs/features/genre-wikidata-import.md UX. result.genre is the canonical
 * genre (the alias target when the match went through a merged genre).
 */
export function genreImportResultFeedback(result: AdminGenreImportResult): GenreImportFeedback {
  const { genre } = result;
  const open: GenreImportFeedbackLink = { href: genreModerationHref(genre), label: 'Открыть' };
  const statusLabel = (GENRE_STATUS_LABELS[genre.status] ?? genre.status).toLowerCase();

  if (result.outcome === 'rejected') {
    let message = `Жанр «${genre.name}» был отклонён ранее и повторно не создаётся.`;
    if (result.qid_backfilled) message += ' QID привязан к нему.';
    return { type: 'error', message, links: [open] };
  }

  if (result.outcome === 'created') {
    return {
      type: 'success',
      message: `Жанр «${genre.name}» импортирован и ждёт модерации.`,
      links: [open, MODERATION_LINK],
    };
  }

  if (result.via_alias) {
    return {
      type: 'success',
      message: `Этот жанр был слит в «${genre.name}» (${statusLabel}) — новый жанр не создан.`,
      links: [open],
    };
  }

  if (result.match === 'slug' && result.qid_backfilled) {
    return {
      type: 'success',
      message: `Найден жанр «${genre.name}» без QID — QID ${genre.wikidata_qid ?? ''} привязан к нему.`,
      links: [open],
    };
  }

  if (result.match === 'slug' && result.qid_mismatch) {
    return {
      type: 'success',
      message:
        `Жанр «${genre.name}» уже есть в каталоге с другим QID (${result.qid_mismatch}) — новый жанр не создан. ` +
        'Если это дубликат или другое понятие, разберитесь в модерации.',
      links: [open, MODERATION_LINK],
    };
  }

  return {
    type: 'success',
    message: `Жанр «${genre.name}» уже есть в каталоге (${statusLabel}).`,
    links: [open],
  };
}

/** Banner text for a failed genre import call. */
export function genreImportErrorMessage(err: unknown): string {
  if (err instanceof ApiError) {
    if (err.code === 'no_ru_label') {
      return 'У элемента Wikidata нет русского названия — импорт невозможен. Создайте жанр вручную в каталоге.';
    }
    if (err.code === 'not_a_literary_genre') {
      return 'Элемент Wikidata не является литературным жанром — импорт отменён.';
    }
    if (err.status === 502) {
      return 'Wikidata недоступна, попробуйте позже.';
    }
    if (err.status === 503) {
      return 'Импорт жанров не настроен на сервере.';
    }
    if (err.message) return err.message;
  }
  return 'Не удалось импортировать жанр.';
}
