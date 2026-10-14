import { api } from '@bookhaven/api-client';

export const adminAiCuratorApi = {
  status: api.admin.aiCurator.status,
  create: api.admin.aiCurator.create,
  list: api.admin.aiCurator.list,
  get: api.admin.aiCurator.get,
};

// Query keys of the AI curator endpoints (GET /admin/ai-curator/*).
export const CURATION_STATUS_QUERY_KEY = ['admin', 'ai-curator', 'status'] as const;
export const CURATION_REQUESTS_QUERY_KEY = ['admin', 'ai-curator', 'requests'] as const;
export const curationRequestQueryKey = (id: string) => ['admin', 'ai-curator', 'requests', id] as const;
