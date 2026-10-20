import { useMutation, useQueryClient } from '@tanstack/react-query';
import { adminHarvestApi, HARVEST_JOBS_QUERY_KEY } from '../api';

/**
 * Queues the catalog-wide adaptation sync job
 * (POST /admin/books/sync-adaptations/catalog). On success the jobs list is
 * invalidated so the new job shows up. A 409 reaches the caller as ApiError.
 */
export function useTriggerCatalogAdaptationSync() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => adminHarvestApi.triggerCatalogAdaptationSync(),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: HARVEST_JOBS_QUERY_KEY });
    },
  });
}
