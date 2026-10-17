import { api, type AdminGenreAISuggestionListParams } from '@bookhaven/api-client';

export const adminGenreAiSuggestionsApi = {
  list: api.admin.genreAiSuggestions.list,
  status: api.admin.genreAiSuggestions.status,
  analyze: api.admin.genreAiSuggestions.analyze,
  apply: api.admin.genreAiSuggestions.apply,
  bulkApply: api.admin.genreAiSuggestions.bulkApply,
  dismiss: api.admin.genreAiSuggestions.dismiss,
  bulkDismiss: api.admin.genreAiSuggestions.bulkDismiss,
};

// Query keys of the AI genre suggestion endpoints. GENRE_AI_SUGGESTIONS_QUERY_KEY
// is the common prefix (invalidates the list and the status).
export const GENRE_AI_SUGGESTIONS_QUERY_KEY = ['admin', 'genre-ai-suggestions'] as const;
export const GENRE_AI_STATUS_QUERY_KEY = ['admin', 'genre-ai-suggestions', 'status'] as const;
export const genreAiSuggestionsListQueryKey = (params: AdminGenreAISuggestionListParams) =>
  ['admin', 'genre-ai-suggestions', 'list', params] as const;
