export const homeKeys = {
  all: ['home'] as const,
  me: (userId?: string) => [...homeKeys.all, 'me', userId] as const,
};
