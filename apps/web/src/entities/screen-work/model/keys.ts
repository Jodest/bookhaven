export const screenWorkKeys = {
  all: ['screen-works'] as const,
  verdicts: (userId: string | undefined, bookId: string) =>
    [...screenWorkKeys.all, 'verdicts', userId ?? 'guest', bookId] as const,
};
