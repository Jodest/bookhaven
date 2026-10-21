import { api } from '@bookhaven/api-client';

export const screenWorksApi = {
  forBook: api.books.adaptations,
  forAuthor: api.authors.screenWorks,
  verdicts: api.books.adaptationVerdicts,
  vote: api.books.voteAdaptation,
  unvote: api.books.unvoteAdaptation,
};
