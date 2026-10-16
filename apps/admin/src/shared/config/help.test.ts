import fs from 'node:fs';
import path from 'node:path';
import { describe, expect, it } from 'vitest';
import { getHelpChapter, getHelpHref, getVisibleHelpChapters, helpChapters } from './help';

const GUIDE_DIR = path.join(process.cwd(), '..', '..', 'docs', 'admin-guide');

describe('help chapter registry', () => {
  it('lists the nine chapters in order with their titles', () => {
    expect(helpChapters.map((c) => [c.slug, c.title])).toEqual([
      ['getting-started', 'Первые шаги'],
      ['adding-content', 'Как добавить книгу, автора или жанр'],
      ['import-jobs', 'Задачи импорта'],
      ['collections', 'Подборки и ИИ-кураторы'],
      ['catalog-review', 'Проверка каталога'],
      ['moderation', 'Модерация отзывов'],
      ['duplicates', 'Слияние дублей'],
      ['screen-works', 'Экранизации'],
      ['users-roles', 'Пользователи и роли'],
    ]);
  });

  it('only moderation allows the moderator', () => {
    const forModerator = helpChapters.filter((c) => c.roles.includes('moderator')).map((c) => c.slug);
    expect(forModerator).toEqual(['moderation']);
    expect(helpChapters.every((c) => c.roles.includes('admin'))).toBe(true);
  });

  it('role filtering and helpers', () => {
    expect(getVisibleHelpChapters('admin')).toHaveLength(9);
    expect(getVisibleHelpChapters('moderator').map((c) => c.slug)).toEqual(['moderation']);
    expect(getVisibleHelpChapters('user')).toEqual([]);
    expect(getVisibleHelpChapters(undefined)).toEqual([]);
    expect(getHelpChapter('duplicates')?.title).toBe('Слияние дублей');
    expect(getHelpChapter('nope')).toBeNull();
    expect(getHelpHref('import-jobs')).toBe('/help/import-jobs');
  });
});

describe('docs/admin-guide files', () => {
  const files = fs
    .readdirSync(GUIDE_DIR)
    .filter((f) => f.endsWith('.md') && f !== 'README.md')
    .map((f) => f.replace(/\.md$/, ''));

  it('every registered chapter has a file and every file is registered', () => {
    expect(files.sort()).toEqual(helpChapters.map((c) => c.slug).sort());
  });

  it.each(helpChapters.map((c) => [c.slug, c.title]))('%s starts with its registry title', (slug, title) => {
    const text = fs.readFileSync(path.join(GUIDE_DIR, `${slug}.md`), 'utf8');
    expect(text.split(/\r?\n/)[0]).toBe(`# ${title}`);
  });

  it('the moderator chapter links to no other chapter', () => {
    const text = fs.readFileSync(path.join(GUIDE_DIR, 'moderation.md'), 'utf8');
    expect(text).not.toMatch(/\]\(\.\/[a-z-]+\.md/);
  });
});
