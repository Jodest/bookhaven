import type { AdminCoverCopy } from '@bookhaven/api-client';

const FAILED_REASONS: Record<string, string> = {
  host_not_allowed: 'сайт не входит в список разрешённых',
  blocked_address: 'сайт не входит в список разрешённых',
  too_large: 'файл больше 5 МБ',
  not_image: 'это не изображение JPEG, PNG или WebP',
};

const UNAVAILABLE_REASON = 'источник недоступен';

/**
 * Short status of the self-hosted cover copy shown under «URL обложки» on the
 * edit page (openspec catalog-admin/book-editing). Null when the book has no
 * cover link (no copy object), so no hint is shown.
 */
export function coverCopyHint(copy: AdminCoverCopy | null | undefined): string | null {
  if (!copy) return null;
  switch (copy.status) {
    case 'stored':
      return 'Копия обложки сохранена на нашем сервере.';
    case 'pending':
      return 'Обложка копируется на наш сервер. Пока показывается исходная ссылка.';
    case 'failed': {
      const reason = (copy.error && FAILED_REASONS[copy.error]) || UNAVAILABLE_REASON;
      return `Не удалось скопировать обложку: ${reason}. На сайте показывается исходная ссылка.`;
    }
    case 'absent':
      return 'По этой ссылке нет обложки. На сайте показывается «Нет обложки».';
    default:
      return null;
  }
}
