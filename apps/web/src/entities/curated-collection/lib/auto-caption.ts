import type { AutoMetric, AutoRule } from '@bookhaven/api-client';
import { formatTimestampLongDate } from '@/shared/lib/format-date-ru';

const BASIS: Record<AutoMetric, string> = {
  weighted_rating: 'оценкам читателей BookHaven',
  most_read: 'отметкам «Прочитано»',
  most_wanted: 'отметкам «Хочу прочитать»',
  trending: 'активности читателей',
};

/** «Составлена автоматически по … за последние N дн. Обновлено ….» */
export function autoCaption(rule: AutoRule, computedAt: string | null): string {
  // The period ends with «дн.», which also closes the sentence.
  const period = rule.period_days ? ` за последние ${rule.period_days} дн.` : '';
  const head = `Составлена автоматически по ${BASIS[rule.metric]}${period || '.'}`;
  return computedAt ? `${head} Обновлено ${formatTimestampLongDate(computedAt)}.` : head;
}
