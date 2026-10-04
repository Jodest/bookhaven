import { api } from '@bookhaven/api-client';

export const homeApi = {
  get: api.home.get,
  random: api.home.random,
  me: api.home.me,
};
