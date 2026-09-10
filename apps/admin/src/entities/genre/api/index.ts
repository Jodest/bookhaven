import { api } from '@bookhaven/api-client';

export const adminGenresApi = {
  list: api.admin.genres.list,
  create: api.admin.genres.create,
  update: api.admin.genres.update,
};
