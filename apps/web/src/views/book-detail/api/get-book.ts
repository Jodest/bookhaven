import { booksApi } from '@/entities/book';
import * as React from 'react';

// React `cache` dedups the request between generateMetadata and the view
// (the API fetcher uses `no-store`, so Next's fetch dedup does not apply).
// It is absent in non-RSC React builds (e.g. vitest): fall back to identity.
type Memo = <T extends (...args: never[]) => unknown>(fn: T) => T;
const memo: Memo =
  (React as unknown as { cache?: Memo }).cache ?? ((fn) => fn);

export const getBook = memo((slug: string) => booksApi.get(slug).catch(() => null));
