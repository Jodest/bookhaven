import { api } from '@bookhaven/api-client';

export const adminAiCollectionsApi = {
  status: api.admin.aiCollections.status,
  create: api.admin.aiCollections.create,
  list: api.admin.aiCollections.list,
  get: api.admin.aiCollections.get,
};

// Query keys of the AI collection draft endpoints (GET /admin/ai-collections/*).
export const AI_COLLECTIONS_STATUS_QUERY_KEY = ['admin', 'ai-collections', 'status'] as const;
export const aiDraftsQueryKey = (collectionId: string) =>
  ['admin', 'ai-collections', 'drafts', 'list', collectionId] as const;
export const aiDraftQueryKey = (id: string) => ['admin', 'ai-collections', 'drafts', 'detail', id] as const;
