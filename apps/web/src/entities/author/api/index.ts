import { api } from '@bookhaven/api-client';

export const authorsApi = {
  list: api.authors.list,
  get: api.authors.get,
};
