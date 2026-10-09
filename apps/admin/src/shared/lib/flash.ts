// One-shot flash messages across a client-side navigation
// (docs/features/catalog-unified-lists.md, Business Rule 7). Stored in
// sessionStorage; every storage failure (disabled storage, quota, SSR) is a
// silent no-op.

const PREFIX = 'bookhaven.flash.';

export function setFlash(key: string, message: string): void {
  try {
    window.sessionStorage.setItem(PREFIX + key, message);
  } catch {
    // Storage unavailable: the flash is simply lost.
  }
}

/** Reads and removes the message; null when there is none. */
export function consumeFlash(key: string): string | null {
  try {
    const storage = window.sessionStorage;
    const message = storage.getItem(PREFIX + key);
    if (message !== null) {
      storage.removeItem(PREFIX + key);
    }
    return message;
  } catch {
    return null;
  }
}
