import { api } from '@bookhaven/api-client';

export const adminAuthorsApi = {
  list: api.admin.authors.list,
  get: api.admin.authors.get,
  create: api.admin.authors.create,
  update: api.admin.authors.update,
  merge: api.admin.authors.merge,
  moderationQueue: api.admin.authors.moderationQueue,
  bulkStatus: api.admin.authors.bulkStatus,
  importWikidata: api.admin.authors.importWikidata,
  enrichWikidata: api.admin.authors.enrichWikidata,
};
