import { api } from '@bookhaven/api-client';

export const adminGenresApi = {
  list: api.admin.genres.list,
  create: api.admin.genres.create,
  update: api.admin.genres.update,
  moderationQueue: api.admin.genres.moderationQueue,
  bulkStatus: api.admin.genres.bulkStatus,
  merge: api.admin.genres.merge,
};
