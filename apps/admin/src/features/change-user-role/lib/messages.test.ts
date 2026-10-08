import { describe, expect, it } from 'vitest';
import { ApiError } from '@bookhaven/api-client';
import { changeRoleConfirmText, changeRoleErrorText, changeRoleSuccessText } from './messages';

describe('changeRoleConfirmText', () => {
  it('names the user and both roles', () => {
    expect(changeRoleConfirmText({ name: 'Анна', email: 'ann@example.com' }, 'user', 'moderator')).toBe(
      'Изменить роль пользователя «Анна»: Читатель → Модератор?'
    );
  });

  it('falls back to the email when the name is empty', () => {
    expect(changeRoleConfirmText({ name: null, email: 'bob@example.com' }, 'moderator', 'contributor')).toBe(
      'Изменить роль пользователя «bob@example.com»: Модератор → Автор контента?'
    );
    expect(changeRoleConfirmText({ name: '  ', email: 'bob@example.com' }, 'admin', 'user')).toContain(
      '«bob@example.com»'
    );
  });

  it('warns about full access when promoting to admin', () => {
    expect(changeRoleConfirmText({ name: 'Анна', email: 'a@x.ru' }, 'moderator', 'admin')).toBe(
      'Изменить роль пользователя «Анна»: Модератор → Администратор? Пользователь получит полный доступ к админ-панели.'
    );
  });
});

describe('changeRoleSuccessText', () => {
  it('describes the change', () => {
    expect(changeRoleSuccessText({ name: null, email: 'c@x.ru' }, 'user', 'admin')).toBe(
      'Роль пользователя «c@x.ru» изменена: Читатель → Администратор.'
    );
  });
});

describe('changeRoleErrorText', () => {
  const apiError = (status: number, code?: string) => new ApiError(status, 'x', 'x', '', code);

  it('maps known error codes', () => {
    expect(changeRoleErrorText(apiError(409, 'last_admin'))).toBe('Нельзя снять роль с последнего администратора.');
    expect(changeRoleErrorText(apiError(409, 'cannot_change_own_role'))).toBe('Нельзя изменить собственную роль.');
    expect(changeRoleErrorText(apiError(404, 'user_not_found'))).toBe('Пользователь не найден — обновите список.');
    expect(changeRoleErrorText(apiError(400, 'invalid_role'))).toBe('Недопустимая роль.');
  });

  it('falls back to a generic message for unknown codes and non-API errors', () => {
    const generic = 'Не удалось изменить роль. Попробуйте ещё раз.';
    expect(changeRoleErrorText(apiError(500))).toBe(generic);
    expect(changeRoleErrorText(apiError(409, 'something_else'))).toBe(generic);
    expect(changeRoleErrorText(new Error('network'))).toBe(generic);
  });
});
