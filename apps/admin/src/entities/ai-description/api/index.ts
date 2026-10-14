import { api, type AdminAIDescriptionListParams } from '@bookhaven/api-client';

export const adminAiDescriptionsApi = {
  status: api.admin.aiDescriptions.status,
  generate: api.admin.aiDescriptions.generate,
  list: api.admin.aiDescriptions.list,
  get: api.admin.aiDescriptions.get,
  approve: api.admin.aiDescriptions.approve,
  discard: api.admin.aiDescriptions.discard,
};

// Query keys of the AI descriptions endpoints (GET /admin/ai-descriptions/*).
// AI_DESCRIPTIONS_QUERY_KEY is the common prefix (invalidates everything).
export const AI_DESCRIPTIONS_QUERY_KEY = ['admin', 'ai-descriptions'] as const;
export const AI_DESCRIPTIONS_STATUS_QUERY_KEY = ['admin', 'ai-descriptions', 'status'] as const;
export const aiDescriptionsListQueryKey = (params: AdminAIDescriptionListParams) =>
  ['admin', 'ai-descriptions', 'list', params] as const;
export const aiDescriptionQueryKey = (bookId: string) => ['admin', 'ai-descriptions', bookId] as const;
