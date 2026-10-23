import { renderHook } from '@testing-library/react';
import { beforeEach, describe, expect, it } from 'vitest';
import { useMyRating, useMyRatingStore } from './my-rating-store';

describe('useMyRatingStore', () => {
  beforeEach(() => useMyRatingStore.setState({ ownerId: null, byBook: {} }));

  it('starts empty and stores a rating per book', () => {
    expect(useMyRatingStore.getState().byBook).toEqual({});
    useMyRatingStore.getState().setMyRating('b1', 4);
    useMyRatingStore.getState().setMyRating('b2', 2);
    expect(useMyRatingStore.getState().byBook).toEqual({ b1: 4, b2: 2 });
  });

  it('overwrites a value and keeps state identity when unchanged', () => {
    useMyRatingStore.getState().setMyRating('b1', 4);
    const before = useMyRatingStore.getState();
    useMyRatingStore.getState().setMyRating('b1', 4);
    expect(useMyRatingStore.getState()).toBe(before);
    useMyRatingStore.getState().setMyRating('b1', 1);
    expect(useMyRatingStore.getState().byBook.b1).toBe(1);
  });

  it('drops the values when the signed-in user changes', () => {
    useMyRatingStore.getState().setOwner('u1');
    useMyRatingStore.getState().setMyRating('b1', 4);
    useMyRatingStore.getState().setOwner('u1');
    expect(useMyRatingStore.getState().byBook).toEqual({ b1: 4 });
    useMyRatingStore.getState().setOwner('u2');
    expect(useMyRatingStore.getState()).toMatchObject({ ownerId: 'u2', byBook: {} });
  });

  it('useMyRating hides values that belong to another user', () => {
    useMyRatingStore.getState().setOwner('u1');
    useMyRatingStore.getState().setMyRating('b1', 4);
    expect(renderHook(() => useMyRating('u1', 'b1')).result.current).toBe(4);
    expect(renderHook(() => useMyRating('u2', 'b1')).result.current).toBeUndefined();
  });
});
