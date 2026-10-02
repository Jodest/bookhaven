import { api } from '@bookhaven/api-client';

export const adminAuthorsApi = {
  list: api.admin.authors.list,
  create: api.admin.authors.create,
  update: api.admin.authors.update,
  merge: api.admin.authors.merge,
};
