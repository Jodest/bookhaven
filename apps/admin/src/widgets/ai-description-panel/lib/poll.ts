/** Poll interval after queueing a generation. */
export const PANEL_POLL_MS = 4000;
/** Polling stops quietly after this long without a new result. */
export const PANEL_POLL_MAX_MS = 3 * 60 * 1000;

/** Polling state: the generated_at seen when queueing, and when. */
export interface PanelPoll {
  previousGeneratedAt: string | null;
  startedAt: number;
}

/**
 * Whether the panel should still poll: a poll is active, the record's
 * generated_at has not changed yet and the 3 minute budget is not spent.
 */
export function shouldPoll(poll: PanelPoll | null, currentGeneratedAt: string | null, now: number): boolean {
  if (!poll) return false;
  if (currentGeneratedAt !== poll.previousGeneratedAt) return false;
  return now - poll.startedAt < PANEL_POLL_MAX_MS;
}
