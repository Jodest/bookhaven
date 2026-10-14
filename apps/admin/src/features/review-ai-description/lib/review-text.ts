import { ApiError } from '@bookhaven/api-client';

/** Max length of an edited text (backend: 1–2000 after trim). */
export const MAX_EDITED_LENGTH = 2000;

export const APPROVED_TEXT = 'Описание ИИ одобрено и показывается на сайте с пометкой.';
export const DISCARDED_TEXT = 'Описание ИИ отклонено.';

/** Readable Russian message for a failed approve/discard. */
export function reviewErrorText(err: unknown): string {
  if (err instanceof ApiError) {
    switch (err.status) {
      case 404:
        return 'Описание ИИ не найдено.';
      case 409:
        return err.message === 'book already has a description'
          ? 'У книги уже есть собственное описание.'
          : 'Описание уже не черновик. Обновите страницу.';
      case 422:
        return 'Текст должен быть от 1 до 2000 символов и написан в основном кириллицей.';
    }
  }
  return 'Не удалось сохранить решение.';
}
