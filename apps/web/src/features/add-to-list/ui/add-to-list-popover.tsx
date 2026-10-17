'use client';

import { useAuthStore } from '@/entities/session';
import { loginHref } from '@/shared/lib/safe-redirect';
import { Button, Popover, PopoverContent, PopoverTrigger } from '@bookhaven/ui';
import { ChevronDown } from 'lucide-react';
import Link from 'next/link';
import { AddToListButton } from './add-to-list-button';

interface AddToListPopoverProps {
  bookId: string;
  slug: string;
}

/** Compact «В список» trigger that opens the unchanged AddToListButton in a popover. */
export function AddToListPopover({ bookId, slug }: AddToListPopoverProps) {
  const { isAuthenticated } = useAuthStore();

  if (!isAuthenticated) {
    return (
      <Button variant="outline" asChild>
        <Link href={loginHref(`/books/${slug}`)}>Добавить в список</Link>
      </Button>
    );
  }

  return (
    <Popover>
      <PopoverTrigger asChild>
        <Button variant="outline">
          В список
          <ChevronDown className="ml-2 h-4 w-4" />
        </Button>
      </PopoverTrigger>
      <PopoverContent
        className="max-h-[60vh] w-72 overflow-y-auto"
        onEscapeKeyDown={(e) => {
          // Escape inside the new-list field only cancels the field (handled by the
          // input itself); a second Escape closes the popover.
          if (e.target instanceof HTMLInputElement) e.preventDefault();
        }}
      >
        <AddToListButton bookId={bookId} slug={slug} />
      </PopoverContent>
    </Popover>
  );
}
