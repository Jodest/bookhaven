import { create } from 'zustand';

// The signed-in reader's rating per book, shared between the star widget and
// the reviews section of the book page. Not persisted. Values belong to
// ownerId: switching the signed-in user drops them, and useMyRating hides
// values that belong to someone else until the switch has happened.
interface MyRatingState {
  ownerId: string | null;
  byBook: Record<string, number>;
  setOwner: (ownerId: string | null) => void;
  setMyRating: (bookId: string, value: number) => void;
}

export const useMyRatingStore = create<MyRatingState>((set) => ({
  ownerId: null,
  byBook: {},
  setOwner: (ownerId) =>
    set((state) => (state.ownerId === ownerId ? state : { ownerId, byBook: {} })),
  setMyRating: (bookId, value) =>
    set((state) =>
      state.byBook[bookId] === value
        ? state
        : { byBook: { ...state.byBook, [bookId]: value } }
    ),
}));

export function useMyRating(ownerId: string | null, bookId: string) {
  return useMyRatingStore((s) => (s.ownerId === ownerId ? s.byBook[bookId] : undefined));
}
