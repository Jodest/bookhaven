import type { AIDescriptionStatus } from '@bookhaven/api-client';

/** Status badge of an AI description (book edit panel, review rows). */
export const STATUS_LABELS: Record<AIDescriptionStatus, string> = {
  draft: 'Черновик',
  approved: 'Одобрено',
  discarded: 'Отклонено',
  unknown: 'ИИ не знает книгу',
  failed: 'Ошибка',
};

/** Options of the review tab's status filter, in display order. */
export const STATUS_FILTER_LABELS: Record<AIDescriptionStatus, string> = {
  draft: 'Черновики',
  approved: 'Одобренные',
  discarded: 'Отклонённые',
  unknown: 'ИИ не знает',
  failed: 'Ошибки',
};

export const AI_DESCRIPTION_STATUSES: AIDescriptionStatus[] = ['draft', 'approved', 'discarded', 'unknown', 'failed'];

/** Readable messages of a failed record's error_code. */
export const ERROR_LABELS: Record<string, string> = {
  model_refusal: 'Модель отказалась писать описание.',
  model_truncated: 'Ответ модели оборвался.',
  invalid_output: 'Модель вернула непригодный ответ (не тот формат, длина или язык).',
};

/** Badge text of a status; an unknown value is shown raw. */
export function aiDescriptionStatusLabel(status: string): string {
  return STATUS_LABELS[status as AIDescriptionStatus] ?? status;
}

/** Message of an error code; an unknown code is shown raw. */
export function aiDescriptionErrorLabel(code: string): string {
  return ERROR_LABELS[code] ?? code;
}

/** Type guard for a status filter value from the URL. */
export function isAIDescriptionStatus(value: string | null): value is AIDescriptionStatus {
  return value !== null && (AI_DESCRIPTION_STATUSES as string[]).includes(value);
}
