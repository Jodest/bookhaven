import { screenWorkKeys } from '@/entities/screen-work';
import { shelfKeys } from '@/entities/shelf';
import { QueryClient } from '@tanstack/react-query';
import { describe, expect, it } from 'vitest';
import { invalidateShelfContents } from './invalidate-shelf-caches';

describe('invalidateShelfContents', () => {
  it('marks shelves and the adaptation verdicts (can_vote) as stale', async () => {
    const queryClient = new QueryClient();
    const verdictsKey = screenWorkKeys.verdicts('u1', 'b1');
    const shelvesKey = shelfKeys.forBook('u1', 'b1');
    queryClient.setQueryData(verdictsKey, { can_vote: false });
    queryClient.setQueryData(shelvesKey, []);

    await invalidateShelfContents(queryClient);

    expect(queryClient.getQueryState(verdictsKey)?.isInvalidated).toBe(true);
    expect(queryClient.getQueryState(shelvesKey)?.isInvalidated).toBe(true);
  });
});
