import { api } from '@bookhaven/api-client';

export const collectionsApi = {
  list: api.collections.list,
  get: api.collections.get,
  forBook: api.collections.forBook,
};
