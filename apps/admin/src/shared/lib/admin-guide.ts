import fs from 'node:fs';
import path from 'node:path';

/**
 * Server-only: reads a chapter of the admin guide (docs/admin-guide/<slug>.md)
 * at build time. Assumes cwd = apps/admin (turbo, `pnpm --filter`, Docker).
 * Not exported from the shared/lib index — it pulls node:fs; import by path
 * from server routes only.
 */
export function getGuideDir(): string {
  return path.join(process.cwd(), '..', '..', 'docs', 'admin-guide');
}

export function readGuideChapter(slug: string): string {
  if (!/^[a-z0-9-]+$/.test(slug)) {
    throw new Error(`Invalid admin guide chapter slug: ${slug}`);
  }
  const file = path.join(getGuideDir(), `${slug}.md`);
  if (!fs.existsSync(file)) {
    throw new Error(`Admin guide chapter "${slug}" not found at ${file} (cwd=${process.cwd()}).`);
  }
  return fs.readFileSync(file, 'utf8');
}
