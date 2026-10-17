import { act, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { SearchBar } from './search-bar';

let searchParamsValue = new URLSearchParams('search=dune');

vi.mock('next/navigation', () => ({
  useSearchParams: () => searchParamsValue,
}));

describe('SearchBar', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    searchParamsValue = new URLSearchParams('search=dune');
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('does not call onSearch when only onSearch identity or searchParams change', () => {
    const first = vi.fn();
    const { rerender } = render(<SearchBar onSearch={first} />);

    searchParamsValue = new URLSearchParams('search=dune&page=2');
    const second = vi.fn();
    rerender(<SearchBar onSearch={second} />);

    act(() => {
      vi.advanceTimersByTime(1000);
    });

    expect(first).not.toHaveBeenCalled();
    expect(second).not.toHaveBeenCalled();
  });

  it('calls onSearch once after debounce when the user types a new value', () => {
    const onSearch = vi.fn();
    render(<SearchBar onSearch={onSearch} />);

    fireEvent.change(screen.getByRole('textbox'), { target: { value: 'dune messiah' } });
    act(() => {
      vi.advanceTimersByTime(299);
    });
    expect(onSearch).not.toHaveBeenCalled();

    act(() => {
      vi.advanceTimersByTime(1);
    });
    expect(onSearch).toHaveBeenCalledTimes(1);
    expect(onSearch).toHaveBeenCalledWith('dune messiah');
  });
});
