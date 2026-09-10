import { api } from '@bookhaven/api-client';

export const reviewsApi = {
  list: api.reviews.list,
  create: api.reviews.create,
  update: api.reviews.update,
  remove: api.reviews.remove,
  toggleLike: api.reviews.toggleLike,
};
