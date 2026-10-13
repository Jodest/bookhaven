import type { AdminBookAIVerdictValue } from '@bookhaven/api-client';

// AI pre-moderation verdict labels (openspec capability
// catalog-moderation/ai-verdicts).

export const AI_VERDICT_LABELS: Record<AdminBookAIVerdictValue, string> = {
  publish: 'Опубликовать',
  reject: 'Отклонить',
  needs_human: 'Нужен человек',
};

/** Russian labels of the model and system reason codes; unknown codes show raw. */
export const AI_REASON_LABELS: Record<string, string> = {
  duplicate: 'дубликат',
  not_russian: 'не на русском',
  not_a_book: 'не книга',
  collection: 'сборник',
  periodical: 'периодика',
  excerpt: 'отрывок',
  wrong_author: 'неверный автор',
  genre_mismatch: 'жанр не подходит',
  insufficient_data: 'мало данных',
  model_refusal: 'модель отказалась',
  model_truncated: 'ответ обрезан',
  invalid_output: 'некорректный ответ',
};

export function aiReasonLabel(code: string): string {
  return AI_REASON_LABELS[code] ?? code;
}
