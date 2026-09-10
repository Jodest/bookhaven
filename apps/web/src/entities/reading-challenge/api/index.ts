import { api } from '@bookhaven/api-client';

export const readingChallengesApi = {
  list: api.readingChallenges.list,
  get: api.readingChallenges.get,
  upsert: api.readingChallenges.upsert,
};
