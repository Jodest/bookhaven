import { api } from '@bookhaven/api-client';

export const adminReviewsApi = {
  list: api.admin.reviews.list,
  moderate: api.admin.reviews.moderate,
};
