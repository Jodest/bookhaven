import { api } from '@bookhaven/api-client';

export const adminScreenWorksApi = {
  list: api.admin.screenWorks.list,
  get: api.admin.screenWorks.get,
  update: api.admin.screenWorks.update,
  delete: api.admin.screenWorks.delete,
  uploadPoster: api.admin.screenWorks.uploadPoster,
  deletePoster: api.admin.screenWorks.deletePoster,
};
