export const readingChallengeKeys = {
  all: ['reading-challenges'] as const,
  year: (year: number) => [...readingChallengeKeys.all, year] as const,
  detail: (year: number, userId?: string) => [...readingChallengeKeys.year(year), userId] as const,
};
