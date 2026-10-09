import { api } from '@bookhaven/api-client';

export const adminGenresApi = {
  list: api.admin.genres.list,
  get: api.admin.genres.get,
  create: api.admin.genres.create,
  update: api.admin.genres.update,
  moderationQueue: api.admin.genres.moderationQueue,
  bulkStatus: api.admin.genres.bulkStatus,
  merge: api.admin.genres.merge,
  searchWikidata: api.admin.genres.searchWikidata,
  importWikidata: api.admin.genres.importWikidata,
};
