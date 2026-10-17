import { ApiError, type AdminGenreMergeUndoCode } from '@bookhaven/api-client';

const CONFLICT_MESSAGES: Record<AdminGenreMergeUndoCode, string> = {
  already_undone: 'Это слияние уже отменено.',
  target_missing: 'Целевой жанр удалён — отменить слияние нельзя.',
  target_merged: 'Целевой жанр с тех пор слит с другим жанром — сначала отмените более позднее слияние.',
  source_missing: 'Один из исходных жанров удалён — отменить слияние нельзя.',
  source_not_alias: 'Один из исходных жанров больше не связан с целевым — отменить слияние нельзя.',
  qid_conflict: 'QID одного из исходных жанров теперь занят другим жанром — отменить слияние нельзя.',
};

// Texts shared by a failed undo preview and a failed undo.
export function undoErrorMessage(err: unknown): string {
  if (err instanceof ApiError) {
    if (err.status === 409 && err.code && err.code in CONFLICT_MESSAGES) {
      return CONFLICT_MESSAGES[err.code as AdminGenreMergeUndoCode];
    }
    if (err.status === 404) {
      return 'Слияние не найдено. Обновите список.';
    }
  }
  return 'Не удалось отменить слияние. Попробуйте позже.';
}
